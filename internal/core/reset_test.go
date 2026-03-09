package core

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
)

type resetFakeStore struct {
	trackedGroupIDs      map[int64][]int64
	activeCreators       []Creator
	creatorGroups        map[string][]ManagedGroup
	subscriberByCreator  map[string]map[string]bool
	deleteAllUserDataErr error
	getIdentityFn        func(ctx context.Context, telegramUserID int64) (UserIdentity, bool, error)
	getCreatorFn         func(ctx context.Context, ownerTelegramID int64) (Creator, bool, error)

	deleteCreatorCount int
	deleteCreatorNames []string
	deleteCreatorErr   error

	deleteAllCalledWith int64
}

func (f *resetFakeStore) ListTrackedGroupIDsForUser(_ context.Context, telegramUserID int64) ([]int64, error) {
	return append([]int64(nil), f.trackedGroupIDs[telegramUserID]...), nil
}

func (f *resetFakeStore) ListActiveCreators(context.Context) ([]Creator, error) {
	return append([]Creator(nil), f.activeCreators...), nil
}

func (f *resetFakeStore) ListManagedGroupsByCreator(_ context.Context, creatorID string) ([]ManagedGroup, error) {
	return append([]ManagedGroup(nil), f.creatorGroups[creatorID]...), nil
}

func (f *resetFakeStore) IsCreatorSubscriber(_ context.Context, creatorID, twitchUserID string) (bool, error) {
	return f.subscriberByCreator[creatorID][twitchUserID], nil
}

func (f *resetFakeStore) UserIdentity(ctx context.Context, telegramUserID int64) (UserIdentity, bool, error) {
	if f.getIdentityFn != nil {
		return f.getIdentityFn(ctx, telegramUserID)
	}
	return UserIdentity{}, false, nil
}

func (f *resetFakeStore) OwnedCreatorForUser(ctx context.Context, ownerTelegramID int64) (Creator, bool, error) {
	if f.getCreatorFn != nil {
		return f.getCreatorFn(ctx, ownerTelegramID)
	}
	return Creator{}, false, nil
}

func (f *resetFakeStore) DeleteAllUserData(_ context.Context, telegramUserID int64) error {
	f.deleteAllCalledWith = telegramUserID
	return f.deleteAllUserDataErr
}

func (f *resetFakeStore) DeleteCreatorData(_ context.Context, _ int64) (int, []string, error) {
	return f.deleteCreatorCount, append([]string(nil), f.deleteCreatorNames...), f.deleteCreatorErr
}

func TestSubLinkedGroupIDsForUser(t *testing.T) {
	t.Parallel()

	st := &resetFakeStore{
		trackedGroupIDs: map[int64][]int64{
			7: {222, 111, 222},
		},
	}
	svc := NewResetter(st, func(context.Context, int64, int64) error { return nil }, nil)

	got, err := svc.SubLinkedGroupIDsForUser(t.Context(), 7)
	if err != nil {
		t.Fatalf("SubLinkedGroupIDsForUser(%d) returned error %v, want nil", 7, err)
	}
	want := []int64{111, 222}
	if !slices.Equal(got, want) {
		t.Errorf("SubLinkedGroupIDsForUser(%d) = %v, want %v", 7, got, want)
	}
}

func TestSubLinkedGroupIDsForUserIncludesCanonicalFallback(t *testing.T) {
	t.Parallel()

	st := &resetFakeStore{
		getIdentityFn: func(context.Context, int64) (UserIdentity, bool, error) {
			return UserIdentity{TwitchUserID: "tw-7"}, true, nil
		},
		activeCreators: []Creator{{ID: "c1"}, {ID: "c2"}},
		creatorGroups: map[string][]ManagedGroup{
			"c1": {{ChatID: 111}, {ChatID: 222}},
			"c2": {{ChatID: 333}},
		},
		subscriberByCreator: map[string]map[string]bool{
			"c1": {"tw-7": true},
			"c2": {"tw-7": false},
		},
	}
	svc := NewResetter(st, func(context.Context, int64, int64) error { return nil }, nil)

	got, err := svc.SubLinkedGroupIDsForUser(t.Context(), 7)
	if err != nil {
		t.Fatalf("SubLinkedGroupIDsForUser(%d) returned error %v, want nil", 7, err)
	}
	want := []int64{111, 222}
	if !slices.Equal(got, want) {
		t.Errorf("SubLinkedGroupIDsForUser(%d) = %v, want %v", 7, got, want)
	}
}

func TestSubLinkedGroupIDsForUserUnionsTrackedAndCanonicalGroups(t *testing.T) {
	t.Parallel()

	st := &resetFakeStore{
		trackedGroupIDs: map[int64][]int64{
			7: {222, 111},
		},
		getIdentityFn: func(context.Context, int64) (UserIdentity, bool, error) {
			return UserIdentity{TwitchUserID: "tw-7"}, true, nil
		},
		activeCreators: []Creator{{ID: "c1"}},
		creatorGroups: map[string][]ManagedGroup{
			"c1": {{ChatID: 222}, {ChatID: 333}},
		},
		subscriberByCreator: map[string]map[string]bool{
			"c1": {"tw-7": true},
		},
	}
	svc := NewResetter(st, func(context.Context, int64, int64) error { return nil }, nil)

	got, err := svc.SubLinkedGroupIDsForUser(t.Context(), 7)
	if err != nil {
		t.Fatalf("SubLinkedGroupIDsForUser(%d) returned error %v, want nil", 7, err)
	}
	want := []int64{111, 222, 333}
	if !slices.Equal(got, want) {
		t.Errorf("SubLinkedGroupIDsForUser(%d) = %v, want %v", 7, got, want)
	}
}

func TestResetViewerDataAndRevokeGroupAccess(t *testing.T) {
	t.Parallel()

	st := &resetFakeStore{
		trackedGroupIDs: map[int64][]int64{
			9: {300, 100},
		},
	}
	var kicked []int64
	svc := NewResetter(
		st,
		func(_ context.Context, groupChatID int64, _ int64) error {
			kicked = append(kicked, groupChatID)
			if groupChatID == 300 {
				return errors.New("telegram failure")
			}
			return nil
		},
		slog.New(slog.DiscardHandler),
	)

	var resolution GroupResolutionStats
	count, resolution, err := svc.ResetViewerDataAndRevokeGroupAccess(t.Context(), 9)
	if err != nil {
		t.Fatalf("ResetViewerDataAndRevokeGroupAccess(%d) returned error %v, want nil", 9, err)
	}
	if count != 2 {
		t.Errorf("ResetViewerDataAndRevokeGroupAccess(%d) count = %d, want %d", 9, count, 2)
	}
	if resolution.TrackedCount != 2 || resolution.TotalCount != 2 {
		t.Errorf("ResetViewerDataAndRevokeGroupAccess(%d) resolution = %+v, want tracked=2 total=2", 9, resolution)
	}
	if !slices.Equal(kicked, []int64{100, 300}) {
		t.Errorf("ResetViewerDataAndRevokeGroupAccess(%d) kicked groups = %v, want %v", 9, kicked, []int64{100, 300})
	}
	if st.deleteAllCalledWith != 9 {
		t.Errorf("ResetViewerDataAndRevokeGroupAccess(%d) DeleteAllUserData arg = %d, want %d", 9, st.deleteAllCalledWith, 9)
	}
}

func TestDeleteCreatorDataPassthrough(t *testing.T) {
	t.Parallel()

	st := &resetFakeStore{
		deleteCreatorCount: 1,
		deleteCreatorNames: []string{"creator-a"},
	}
	svc := NewResetter(st, func(context.Context, int64, int64) error { return nil }, nil)

	count, names, err := svc.DeleteCreatorData(t.Context(), 42)
	if err != nil {
		t.Fatalf("DeleteCreatorData(%d) returned error %v, want nil", 42, err)
	}
	if count != 1 || !slices.Equal(names, []string{"creator-a"}) {
		t.Errorf("DeleteCreatorData(%d) = (count=%d, names=%v), want (count=%d, names=%v)", 42, count, names, 1, []string{"creator-a"})
	}
}

func TestExecuteBothReset(t *testing.T) {
	t.Parallel()

	svc := NewResetter(
		&resetFakeStore{
			getIdentityFn: func(context.Context, int64) (UserIdentity, bool, error) {
				return UserIdentity{TwitchLogin: "viewer1"}, true, nil
			},
			deleteCreatorCount: 2,
			deleteCreatorNames: []string{"c1", "c2"},
		},
		func(context.Context, int64, int64) error { return nil },
		nil,
	)

	// Override linked groups for deterministic count in this unit test.
	svc.store = &resetFakeStore{
		getIdentityFn: func(context.Context, int64) (UserIdentity, bool, error) {
			return UserIdentity{TwitchLogin: "viewer1"}, true, nil
		},
		trackedGroupIDs:    map[int64][]int64{7: {1, 2, 3}},
		deleteCreatorCount: 2,
		deleteCreatorNames: []string{"c1", "c2"},
	}

	got, err := svc.ExecuteBothReset(t.Context(), 7)
	if err != nil {
		t.Fatalf("ExecuteBothReset(%d) returned error %v, want nil", 7, err)
	}
	type comparableResult struct {
		HasIdentity  bool
		Identity     UserIdentity
		GroupCount   int
		DeletedCount int
	}
	gotCore := comparableResult{
		HasIdentity:  got.HasIdentity,
		Identity:     got.Identity,
		GroupCount:   got.GroupCount,
		DeletedCount: got.DeletedCount,
	}
	wantCore := comparableResult{
		HasIdentity:  true,
		Identity:     UserIdentity{TwitchLogin: "viewer1"},
		GroupCount:   3,
		DeletedCount: 2,
	}
	if gotCore != wantCore {
		t.Errorf("ExecuteBothReset(%d) core = %+v, want %+v", 7, gotCore, wantCore)
	}
	if !slices.Equal(got.DeletedNames, []string{"c1", "c2"}) {
		t.Errorf("ExecuteBothReset(%d).DeletedNames = %v, want %v", 7, got.DeletedNames, []string{"c1", "c2"})
	}
	if got.GroupResolution.TotalCount != 3 || got.GroupResolution.TrackedCount != 3 {
		t.Errorf("ExecuteBothReset(%d).GroupResolution = %+v, want tracked=3 total=3", 7, got.GroupResolution)
	}
}

func TestExecuteViewerResetNoIdentity(t *testing.T) {
	t.Parallel()

	svc := NewResetter(&resetFakeStore{}, func(context.Context, int64, int64) error { return nil }, nil)
	got, err := svc.ExecuteViewerReset(t.Context(), 1)
	if err != nil {
		t.Fatalf("ExecuteViewerReset(%d) returned error %v, want nil", 1, err)
	}
	if got.HasIdentity {
		t.Errorf("ExecuteViewerReset(%d).HasIdentity = %t, want %t", 1, got.HasIdentity, false)
	}
}

type resetFakeEventSubCleaner struct {
	deletedCreatorIDs []string
	err               error
}

func (f *resetFakeEventSubCleaner) DeleteEventSubsForCreator(_ context.Context, creatorID string) error {
	f.deletedCreatorIDs = append(f.deletedCreatorIDs, creatorID)
	return f.err
}

func TestDeleteCreatorDataCallsEventSubCleaner(t *testing.T) {
	t.Parallel()

	cleaner := &resetFakeEventSubCleaner{}
	st := &resetFakeStore{
		getCreatorFn: func(_ context.Context, _ int64) (Creator, bool, error) {
			return Creator{ID: "c1"}, true, nil
		},
		deleteCreatorCount: 1,
		deleteCreatorNames: []string{"creator-a"},
	}
	svc := NewResetter(st, func(context.Context, int64, int64) error { return nil }, nil)
	svc.SetEventSubCleaner(cleaner)

	count, names, err := svc.DeleteCreatorData(t.Context(), 42)
	if err != nil {
		t.Fatalf("DeleteCreatorData(%d) returned error %v, want nil", 42, err)
	}
	if count != 1 || !slices.Equal(names, []string{"creator-a"}) {
		t.Errorf("DeleteCreatorData(%d) = (count=%d, names=%v), want (count=%d, names=%v)", 42, count, names, 1, []string{"creator-a"})
	}
	if !slices.Equal(cleaner.deletedCreatorIDs, []string{"c1"}) {
		t.Errorf("EventSub cleaner called with %v, want %v", cleaner.deletedCreatorIDs, []string{"c1"})
	}
}

func TestDeleteCreatorDataContinuesOnCleanerError(t *testing.T) {
	t.Parallel()

	cleaner := &resetFakeEventSubCleaner{err: errors.New("twitch down")}
	st := &resetFakeStore{
		getCreatorFn: func(_ context.Context, _ int64) (Creator, bool, error) {
			return Creator{ID: "c1"}, true, nil
		},
		deleteCreatorCount: 1,
		deleteCreatorNames: []string{"creator-a"},
	}
	svc := NewResetter(st, func(context.Context, int64, int64) error { return nil }, slog.New(slog.DiscardHandler))
	svc.SetEventSubCleaner(cleaner)

	count, _, err := svc.DeleteCreatorData(t.Context(), 42)
	if err != nil {
		t.Fatalf("DeleteCreatorData(%d) returned error %v, want nil (cleaner failure is non-fatal)", 42, err)
	}
	if count != 1 {
		t.Errorf("DeleteCreatorData(%d) count = %d, want %d", 42, count, 1)
	}
}

func TestLoadScopesPropagatesError(t *testing.T) {
	t.Parallel()

	svc := NewResetter(
		&resetFakeStore{
			getIdentityFn: func(context.Context, int64) (UserIdentity, bool, error) {
				return UserIdentity{}, false, errors.New("boom")
			},
		},
		func(context.Context, int64, int64) error { return nil },
		nil,
	)
	_, err := svc.LoadScopes(t.Context(), 1)
	if err == nil {
		t.Fatal("LoadScopes(1) error = nil, want non-nil")
	}
}
