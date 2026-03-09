package usecase

import (
	"context"
	"errors"
	"slices"
	"testing"

	"imsub/internal/core"
	"imsub/internal/events"
)

type resetServiceStub struct {
	loadScopesFn  func(context.Context, int64) (core.ScopeState, error)
	countViewerFn func(context.Context, int64) (int, error)
	viewerFn      func(context.Context, int64) (core.ViewerResetResult, error)
	creatorFn     func(context.Context, int64) (core.CreatorResetResult, error)
	bothFn        func(context.Context, int64) (core.BothResetResult, error)
}

func (s resetServiceStub) LoadScopes(ctx context.Context, telegramUserID int64) (core.ScopeState, error) {
	return s.loadScopesFn(ctx, telegramUserID)
}
func (s resetServiceStub) CountViewerGroups(ctx context.Context, telegramUserID int64) (int, error) {
	return s.countViewerFn(ctx, telegramUserID)
}
func (s resetServiceStub) ExecuteViewerReset(ctx context.Context, telegramUserID int64) (core.ViewerResetResult, error) {
	return s.viewerFn(ctx, telegramUserID)
}
func (s resetServiceStub) ExecuteCreatorReset(ctx context.Context, telegramUserID int64) (core.CreatorResetResult, error) {
	return s.creatorFn(ctx, telegramUserID)
}
func (s resetServiceStub) ExecuteBothReset(ctx context.Context, telegramUserID int64) (core.BothResetResult, error) {
	return s.bothFn(ctx, telegramUserID)
}

type resetObserverStub struct {
	events []events.Event
}

func (o *resetObserverStub) Emit(_ context.Context, evt events.Event) {
	o.events = append(o.events, evt)
}

func TestExecuteViewerRecordsMetrics(t *testing.T) {
	t.Parallel()

	obs := &resetObserverStub{}
	uc := NewResetUseCase(resetServiceStub{
		viewerFn: func(context.Context, int64) (core.ViewerResetResult, error) {
			return core.ViewerResetResult{
				HasIdentity: true,
				Identity:    core.UserIdentity{TwitchLogin: "viewer1"},
				GroupCount:  3,
				GroupResolution: core.GroupResolutionStats{
					TrackedCount:        2,
					CanonicalCount:      2,
					CanonicalAddedCount: 1,
					TotalCount:          3,
				},
			}, nil
		},
	}, obs)

	got, err := uc.Execute(t.Context(), 7, ResetScopeViewer)
	if err != nil {
		t.Fatalf("Execute(viewer) error = %v", err)
	}
	if got.GroupCount != 3 || got.ViewerLogin != "viewer1" {
		t.Fatalf("Execute(viewer) = %+v, want group_count=3 viewer=viewer1", got)
	}
	gotExec := []events.Event{
		{Name: events.NameResetGroupTarget, Fields: map[string]string{"source": "tracked"}, Count: 2},
		{Name: events.NameResetGroupTarget, Fields: map[string]string{"source": "canonical"}, Count: 2},
		{Name: events.NameResetGroupTarget, Fields: map[string]string{"source": "canonical_added"}, Count: 1},
		{Name: events.NameResetGroupTarget, Fields: map[string]string{"source": "final"}, Count: 3},
		{Name: events.NameResetExecuted, Outcome: "ok", Fields: map[string]string{"scope": "viewer"}},
	}
	if !slices.EqualFunc(obs.events, gotExec, func(a, b events.Event) bool {
		return a.Name == b.Name && a.Outcome == b.Outcome && a.Count == b.Count && mapsEqual(a.Fields, b.Fields)
	}) {
		t.Fatalf("events = %+v, want %+v", obs.events, gotExec)
	}
}

func TestExecuteCreatorEmpty(t *testing.T) {
	t.Parallel()

	obs := &resetObserverStub{}
	uc := NewResetUseCase(resetServiceStub{
		creatorFn: func(context.Context, int64) (core.CreatorResetResult, error) {
			return core.CreatorResetResult{}, nil
		},
	}, obs)

	got, err := uc.Execute(t.Context(), 7, ResetScopeCreator)
	if err != nil {
		t.Fatalf("Execute(creator) error = %v", err)
	}
	if !got.Empty {
		t.Fatalf("Execute(creator).Empty = false, want true")
	}
	want := []events.Event{{Name: events.NameResetExecuted, Outcome: "empty", Fields: map[string]string{"scope": "creator"}}}
	if !slices.EqualFunc(obs.events, want, func(a, b events.Event) bool {
		return a.Name == b.Name && a.Outcome == b.Outcome && a.Count == b.Count && mapsEqual(a.Fields, b.Fields)
	}) {
		t.Fatalf("events = %+v, want %+v", obs.events, want)
	}
}

func TestExecuteBothFailure(t *testing.T) {
	t.Parallel()

	obs := &resetObserverStub{}
	uc := NewResetUseCase(resetServiceStub{
		bothFn: func(context.Context, int64) (core.BothResetResult, error) {
			return core.BothResetResult{}, errors.New("boom")
		},
	}, obs)

	_, err := uc.Execute(t.Context(), 7, ResetScopeBoth)
	if err == nil {
		t.Fatal("Execute(both) error = nil, want non-nil")
	}
	want := []events.Event{{Name: events.NameResetExecuted, Outcome: "failed", Fields: map[string]string{"scope": "both"}}}
	if !slices.EqualFunc(obs.events, want, func(a, b events.Event) bool {
		return a.Name == b.Name && a.Outcome == b.Outcome && a.Count == b.Count && mapsEqual(a.Fields, b.Fields)
	}) {
		t.Fatalf("events = %+v, want %+v", obs.events, want)
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func equalEvents(a, b events.Event) bool {
	return a.Name == b.Name && a.Outcome == b.Outcome && a.Count == b.Count && mapsEqual(a.Fields, b.Fields)
}
