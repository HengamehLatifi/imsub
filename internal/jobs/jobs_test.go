package jobs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"imsub/internal/core"
	"imsub/internal/events"
)

type fakeStore struct {
	listCreatorsFn     func(ctx context.Context) ([]core.Creator, error)
	activeWithoutGroup func(ctx context.Context, creators []core.Creator) (int, error)
	repairReverseIndex func(ctx context.Context) (int, int, int, int, error)
	listManagedGroups  func(ctx context.Context) ([]core.ManagedGroup, error)
	listUntracked      func(ctx context.Context, chatID int64) ([]core.UntrackedGroupMember, error)
	removeUntracked    func(ctx context.Context, chatID, telegramUserID int64) error
}

func (f *fakeStore) ListCreators(ctx context.Context) ([]core.Creator, error) {
	if f.listCreatorsFn != nil {
		return f.listCreatorsFn(ctx)
	}
	return nil, nil
}

func (f *fakeStore) ActiveCreatorIDsWithoutGroup(ctx context.Context, creators []core.Creator) (int, error) {
	if f.activeWithoutGroup != nil {
		return f.activeWithoutGroup(ctx, creators)
	}
	return 0, nil
}

func (f *fakeStore) RepairTrackedGroupReverseIndex(ctx context.Context) (indexUsers, repairedUsers, missingLinks, staleLinks int, err error) {
	if f.repairReverseIndex != nil {
		return f.repairReverseIndex(ctx)
	}
	return 0, 0, 0, 0, nil
}

func (f *fakeStore) ListManagedGroups(ctx context.Context) ([]core.ManagedGroup, error) {
	if f.listManagedGroups != nil {
		return f.listManagedGroups(ctx)
	}
	return nil, nil
}

func (f *fakeStore) ListUntrackedGroupMembers(ctx context.Context, chatID int64) ([]core.UntrackedGroupMember, error) {
	if f.listUntracked != nil {
		return f.listUntracked(ctx, chatID)
	}
	return nil, nil
}

func (f *fakeStore) RemoveUntrackedGroupMember(ctx context.Context, chatID, telegramUserID int64) error {
	if f.removeUntracked != nil {
		return f.removeUntracked(ctx, chatID, telegramUserID)
	}
	return nil
}

type fakeReconciler struct {
	mu     sync.Mutex
	result string
	calls  int
}

func (f *fakeReconciler) ReconcileSubscribersOnce(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.result != "ok" && f.result != "" {
		if f.result == "partial_failure" {
			return core.ErrPartialReconcile
		}
		return errors.New(f.result)
	}
	return nil
}

func (f *fakeReconciler) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

type fakeObserver struct {
	mu        sync.Mutex
	lastEvent events.Event
	calls     int
}

type fakeGroupKicker struct {
	mu    sync.Mutex
	kicks [][2]int64
}

func (f *fakeGroupKicker) KickFromGroup(_ context.Context, groupChatID, telegramUserID int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.kicks = append(f.kicks, [2]int64{groupChatID, telegramUserID})
	return nil
}

func (f *fakeGroupKicker) snapshot() [][2]int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([][2]int64(nil), f.kicks...)
}

func (f *fakeObserver) Emit(_ context.Context, evt events.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	f.lastEvent = evt
}

func (f *fakeObserver) snapshot() (calls int, evt events.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls, f.lastEvent
}

func TestRunScheduledRecordsObserverResult(t *testing.T) {
	t.Parallel()

	obs := &fakeObserver{}
	reconcile := &fakeReconciler{result: "partial_failure"}
	runner := NewRunner(nil, obs)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = runner.RunScheduled(ctx, Schedule{
			Task:     NewSubscriberTask(reconcile),
			Interval: 5 * time.Millisecond,
		})
	}()

	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		calls, _ := obs.snapshot()
		if calls > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	<-done

	calls, evt := obs.snapshot()
	if calls == 0 {
		t.Fatal("expected at least one observer call")
	}
	if evt.Name != events.NameBackgroundJob {
		t.Errorf("Emit() name = %q, want %q", evt.Name, events.NameBackgroundJob)
	}
	if evt.Fields["job"] != "reconcile_subscribers" {
		t.Errorf("Emit() job = %q, want \"reconcile_subscribers\"", evt.Fields["job"])
	}
	if evt.Outcome != "partial_failure" {
		t.Errorf("Emit() outcome = %q, want \"partial_failure\"", evt.Outcome)
	}
	if evt.Duration <= 0 {
		t.Errorf("Emit() duration = %v, want > 0", evt.Duration)
	}
}

func TestRunScheduledStopsOnCancel(t *testing.T) {
	t.Parallel()

	reconcile := &fakeReconciler{result: "ok"}
	runner := NewRunner(nil, nil)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = runner.RunScheduled(ctx, Schedule{
			Task:     NewSubscriberTask(reconcile),
			Interval: 5 * time.Millisecond,
		})
	}()

	deadline := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(deadline) {
		if reconcile.callCount() > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if reconcile.callCount() == 0 {
		t.Fatal("expected at least one reconcile call")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(300 * time.Millisecond):
		t.Fatal("RunScheduled did not stop after cancel")
	}
}

func TestIntegrityAuditTaskClassifiesFailureResult(t *testing.T) {
	t.Parallel()

	store := &fakeStore{
		listCreatorsFn: func(_ context.Context) ([]core.Creator, error) {
			return nil, errors.New("boom")
		},
	}
	task := NewIntegrityAuditTask(store, nil, nil)

	err := task.Run(t.Context())
	if err == nil {
		t.Fatal("Run() error = nil, want non-nil")
	}
	if got := task.Classify(err); got != "list_creators_failed" {
		t.Fatalf("Classify() = %q, want %q", got, "list_creators_failed")
	}
}

func TestGracePolicyTaskKicksExpiredUnverifiedMembers(t *testing.T) {
	t.Parallel()

	kicker := &fakeGroupKicker{}
	var removed [][2]int64
	store := &fakeStore{
		listManagedGroups: func(context.Context) ([]core.ManagedGroup, error) {
			return []core.ManagedGroup{
				{ChatID: 100, Policy: core.GroupPolicyGraceWeek},
				{ChatID: 101, Policy: core.GroupPolicyObserve},
			}, nil
		},
		listUntracked: func(_ context.Context, chatID int64) ([]core.UntrackedGroupMember, error) {
			if chatID != 100 {
				return nil, nil
			}
			return []core.UntrackedGroupMember{
				{ChatID: 100, TelegramUserID: 10, FirstSeenAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
				{ChatID: 100, TelegramUserID: 11, FirstSeenAt: time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)},
			}, nil
		},
		removeUntracked: func(_ context.Context, chatID, telegramUserID int64) error {
			removed = append(removed, [2]int64{chatID, telegramUserID})
			return nil
		},
	}

	taskIface := NewGracePolicyTask(store, kicker, nil)
	task, ok := taskIface.(gracePolicyTask)
	if !ok {
		t.Fatalf("NewGracePolicyTask() type = %T, want gracePolicyTask", taskIface)
	}
	task.now = func() time.Time { return time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC) }

	if err := task.Run(t.Context()); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if got := kicker.snapshot(); len(got) != 1 || got[0] != [2]int64{100, 10} {
		t.Fatalf("kicks = %#v, want only expired member", got)
	}
	if len(removed) != 1 || removed[0] != [2]int64{100, 10} {
		t.Fatalf("removed = %#v, want only expired member", removed)
	}
}

func TestGracePolicyTaskContinuesAfterMemberError(t *testing.T) {
	t.Parallel()

	kicker := &fakeGroupKicker{}
	store := &fakeStore{
		listManagedGroups: func(context.Context) ([]core.ManagedGroup, error) {
			return []core.ManagedGroup{{ChatID: 100, Policy: core.GroupPolicyGraceWeek}}, nil
		},
		listUntracked: func(_ context.Context, _ int64) ([]core.UntrackedGroupMember, error) {
			return []core.UntrackedGroupMember{
				{ChatID: 100, TelegramUserID: 10, FirstSeenAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
				{ChatID: 100, TelegramUserID: 11, FirstSeenAt: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)},
			}, nil
		},
		removeUntracked: func(_ context.Context, _ int64, telegramUserID int64) error {
			if telegramUserID == 10 {
				return errors.New("cleanup boom")
			}
			return nil
		},
	}

	taskIface := NewGracePolicyTask(store, kicker, nil)
	task, ok := taskIface.(gracePolicyTask)
	if !ok {
		t.Fatalf("NewGracePolicyTask() type = %T, want gracePolicyTask", taskIface)
	}
	task.now = func() time.Time { return time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC) }

	err := task.Run(t.Context())
	if err == nil {
		t.Fatal("Run() error = nil, want partial failure")
	}
	if got := task.Classify(err); got != "partial_failure" {
		t.Fatalf("Classify() = %q, want %q", got, "partial_failure")
	}
	if got := kicker.snapshot(); len(got) != 2 {
		t.Fatalf("kicks = %#v, want both expired members attempted", got)
	}
}
