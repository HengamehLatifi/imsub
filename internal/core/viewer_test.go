package core

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

type viewerFakeStore struct {
	Store
	getIdentityFn        func(ctx context.Context, telegramUserID int64) (UserIdentity, bool, error)
	listActiveCreatorsFn func(ctx context.Context) ([]Creator, error)
	listGroupsFn         func(ctx context.Context, creatorID string) ([]ManagedGroup, error)
	isSubscriberFn       func(ctx context.Context, creatorID, twitchUserID string) (bool, error)
	removeMembershipFn   func(ctx context.Context, chatID, telegramUserID int64) error
	addMembershipFn      func(ctx context.Context, chatID, telegramUserID int64) error
}

func (f *viewerFakeStore) UserIdentity(ctx context.Context, telegramUserID int64) (UserIdentity, bool, error) {
	if f.getIdentityFn != nil {
		return f.getIdentityFn(ctx, telegramUserID)
	}
	return UserIdentity{}, false, nil
}

func (f *viewerFakeStore) ListActiveCreators(ctx context.Context) ([]Creator, error) {
	if f.listActiveCreatorsFn != nil {
		return f.listActiveCreatorsFn(ctx)
	}
	return nil, nil
}

func (f *viewerFakeStore) IsCreatorSubscriber(ctx context.Context, creatorID, twitchUserID string) (bool, error) {
	if f.isSubscriberFn != nil {
		return f.isSubscriberFn(ctx, creatorID, twitchUserID)
	}
	return false, nil
}

func (f *viewerFakeStore) ListManagedGroupsByCreator(ctx context.Context, creatorID string) ([]ManagedGroup, error) {
	if f.listGroupsFn != nil {
		return f.listGroupsFn(ctx, creatorID)
	}
	return nil, nil
}

func (f *viewerFakeStore) RemoveTrackedGroupMember(ctx context.Context, chatID, telegramUserID int64) error {
	if f.removeMembershipFn != nil {
		return f.removeMembershipFn(ctx, chatID, telegramUserID)
	}
	return nil
}

func (f *viewerFakeStore) AddTrackedGroupMember(ctx context.Context, chatID, telegramUserID int64, _ string, _ time.Time) error {
	if f.addMembershipFn != nil {
		return f.addMembershipFn(ctx, chatID, telegramUserID)
	}
	return nil
}

type fakeGroupOps struct {
	isMemberFn     func(ctx context.Context, groupChatID, telegramUserID int64) bool
	createInviteFn func(ctx context.Context, groupChatID int64, telegramUserID int64, name string) (string, error)
}

func (f *fakeGroupOps) IsGroupMember(ctx context.Context, groupChatID, telegramUserID int64) bool {
	if f.isMemberFn != nil {
		return f.isMemberFn(ctx, groupChatID, telegramUserID)
	}
	return false
}

func (f *fakeGroupOps) CreateInviteLink(ctx context.Context, groupChatID int64, telegramUserID int64, name string) (string, error) {
	if f.createInviteFn != nil {
		return f.createInviteFn(ctx, groupChatID, telegramUserID, name)
	}
	return "", nil
}

func TestBuildJoinTargets(t *testing.T) {
	t.Parallel()

	added := make([]int64, 0)
	removed := make([]int64, 0)
	svc := NewViewer(
		&viewerFakeStore{
			listActiveCreatorsFn: func(_ context.Context) ([]Creator, error) {
				return []Creator{
					{ID: "c1", Name: "zeta"},
					{ID: "c2", Name: "alpha"},
					{ID: "c3", Name: "beta"},
				}, nil
			},
			listGroupsFn: func(_ context.Context, creatorID string) ([]ManagedGroup, error) {
				switch creatorID {
				case "c1":
					return []ManagedGroup{{ChatID: 101, CreatorID: "c1", GroupName: "Group Z"}}, nil
				case "c2":
					return []ManagedGroup{{ChatID: 102, CreatorID: "c2", GroupName: "Group A"}}, nil
				case "c3":
					return []ManagedGroup{{ChatID: 103, CreatorID: "c3", GroupName: "Group B"}}, nil
				}
				return nil, nil
			},
			isSubscriberFn: func(_ context.Context, creatorID, _ string) (bool, error) {
				switch creatorID {
				case "c1", "c2":
					return true, nil
				case "c3":
					return false, nil
				}
				return false, nil
			},
			addMembershipFn: func(_ context.Context, chatID, _ int64) error {
				added = append(added, chatID)
				return nil
			},
			removeMembershipFn: func(_ context.Context, chatID, _ int64) error {
				removed = append(removed, chatID)
				return nil
			},
		},
		&fakeGroupOps{
			isMemberFn: func(_ context.Context, groupChatID, _ int64) bool {
				return groupChatID == 102 // already in alpha group
			},
			createInviteFn: func(_ context.Context, _ int64, _ int64, name string) (string, error) {
				return "https://invite/" + name, nil
			},
		},
		nil,
	)

	got, err := svc.BuildJoinTargets(t.Context(), 7, "tw-1")
	if err != nil {
		t.Fatalf("BuildJoinTargets(%d, %q) returned error %v, want nil", 7, "tw-1", err)
	}

	if !slices.Equal(got.ActiveCreatorNames, []string{"alpha", "zeta"}) {
		t.Errorf("BuildJoinTargets(7, tw-1) active names mismatch: got=%v want=%v", got.ActiveCreatorNames, []string{"alpha", "zeta"})
	}
	if len(got.JoinLinks) != 1 || got.JoinLinks[0].CreatorName != "zeta" {
		t.Errorf("BuildJoinTargets() JoinLinks = %+v, want 1 link with CreatorName=\"zeta\"", got.JoinLinks)
	}
	if !slices.Equal(added, []int64{101}) {
		t.Errorf("BuildJoinTargets(7, tw-1) added memberships mismatch: got=%v want=%v", added, []int64{101})
	}
	if !slices.Equal(removed, []int64{103}) {
		t.Errorf("BuildJoinTargets(7, tw-1) removed memberships mismatch: got=%v want=%v", removed, []int64{103})
	}
}

func TestBuildJoinTargetsListError(t *testing.T) {
	t.Parallel()

	svc := NewViewer(
		&viewerFakeStore{
			listActiveCreatorsFn: func(_ context.Context) ([]Creator, error) {
				return nil, errors.New("boom")
			},
		},
		&fakeGroupOps{},
		nil,
	)

	got, err := svc.BuildJoinTargets(t.Context(), 7, "tw-1")
	if err == nil {
		t.Fatalf("BuildJoinTargets(%d, %q) returned error nil, want non-nil error", 7, "tw-1")
	}
	if len(got.ActiveCreatorNames) != 0 || len(got.JoinLinks) != 0 {
		t.Fatalf("BuildJoinTargets(%d, %q) = %+v, want empty targets", 7, "tw-1", got)
	}
}
