package usecase

import (
	"context"
	"errors"
	"slices"
	"testing"

	"imsub/internal/core"
	"imsub/internal/events"
)

type viewerAccessServiceStub struct {
	loadIdentityFn     func(context.Context, int64) (core.UserIdentity, bool, error)
	buildJoinTargetsFn func(context.Context, int64, string) (core.JoinTargets, error)
	buildForCreatorFn  func(context.Context, string, int64, string) (core.JoinTargets, error)
}

func (s viewerAccessServiceStub) LoadIdentity(ctx context.Context, telegramUserID int64) (core.UserIdentity, bool, error) {
	return s.loadIdentityFn(ctx, telegramUserID)
}

func (s viewerAccessServiceStub) BuildJoinTargets(ctx context.Context, telegramUserID int64, twitchUserID string) (core.JoinTargets, error) {
	return s.buildJoinTargetsFn(ctx, telegramUserID, twitchUserID)
}

func (s viewerAccessServiceStub) BuildJoinTargetsForCreator(ctx context.Context, creatorID string, telegramUserID int64, twitchUserID string) (core.JoinTargets, error) {
	return s.buildForCreatorFn(ctx, creatorID, telegramUserID, twitchUserID)
}

type viewerAccessObserverStub struct {
	events []events.Event
}

func TestViewerAccessLoadAccessForCreator(t *testing.T) {
	t.Parallel()

	obs := &viewerAccessObserverStub{}
	uc := NewViewerAccessUseCase(viewerAccessServiceStub{
		loadIdentityFn: func(context.Context, int64) (core.UserIdentity, bool, error) {
			return core.UserIdentity{TelegramUserID: 7, TwitchUserID: "tw-1"}, true, nil
		},
		buildJoinTargetsFn: func(context.Context, int64, string) (core.JoinTargets, error) {
			t.Fatal("BuildJoinTargets should not be used in creator-scoped load")
			return core.JoinTargets{}, nil
		},
		buildForCreatorFn: func(_ context.Context, creatorID string, telegramUserID int64, twitchUserID string) (core.JoinTargets, error) {
			if creatorID != "c1" || telegramUserID != 7 || twitchUserID != "tw-1" {
				t.Fatalf("BuildJoinTargetsForCreator(%q, %d, %q) got unexpected args", creatorID, telegramUserID, twitchUserID)
			}
			return core.JoinTargets{JoinLinks: []core.JoinLink{{CreatorName: "alpha"}}}, nil
		},
	}, obs)

	got, err := uc.LoadAccessForCreator(t.Context(), "c1", 7)
	if err != nil {
		t.Fatalf("LoadAccessForCreator() error = %v", err)
	}
	if !got.HasIdentity || len(got.Targets.JoinLinks) != 1 {
		t.Fatalf("LoadAccessForCreator() = %+v, want linked identity with one join link", got)
	}
	if len(obs.events) != 0 {
		t.Fatalf("LoadAccessForCreator() emitted events = %+v, want none", obs.events)
	}
}

func (o *viewerAccessObserverStub) Emit(_ context.Context, evt events.Event) {
	o.events = append(o.events, evt)
}

func TestViewerAccessLoadAccessUnlinked(t *testing.T) {
	t.Parallel()

	obs := &viewerAccessObserverStub{}
	uc := NewViewerAccessUseCase(viewerAccessServiceStub{
		loadIdentityFn: func(context.Context, int64) (core.UserIdentity, bool, error) {
			return core.UserIdentity{}, false, nil
		},
		buildJoinTargetsFn: func(context.Context, int64, string) (core.JoinTargets, error) {
			return core.JoinTargets{}, nil
		},
	}, obs)

	got, err := uc.LoadAccess(t.Context(), 7)
	if err != nil {
		t.Fatalf("LoadAccess() error = %v", err)
	}
	if got.HasIdentity {
		t.Fatalf("HasIdentity = true, want false")
	}
	want := []events.Event{{Name: events.NameViewerAccess, Outcome: "unlinked"}}
	if !slices.EqualFunc(obs.events, want, equalEvents) {
		t.Fatalf("events = %+v, want %+v", obs.events, want)
	}
}

func TestViewerAccessLoadAccessLinked(t *testing.T) {
	t.Parallel()

	obs := &viewerAccessObserverStub{}
	uc := NewViewerAccessUseCase(viewerAccessServiceStub{
		loadIdentityFn: func(context.Context, int64) (core.UserIdentity, bool, error) {
			return core.UserIdentity{TelegramUserID: 7, TwitchUserID: "tw-1", TwitchLogin: "viewer"}, true, nil
		},
		buildJoinTargetsFn: func(context.Context, int64, string) (core.JoinTargets, error) {
			return core.JoinTargets{
				ActiveCreatorNames: []string{"alpha"},
				JoinLinks:          []core.JoinLink{{CreatorName: "alpha", GroupName: "VIP", InviteLink: "https://invite"}},
			}, nil
		},
	}, obs)

	got, err := uc.LoadAccess(t.Context(), 7)
	if err != nil {
		t.Fatalf("LoadAccess() error = %v", err)
	}
	if !got.HasIdentity || got.Identity.TwitchLogin != "viewer" || len(got.Targets.JoinLinks) != 1 {
		t.Fatalf("got = %+v", got)
	}
	want := []events.Event{{Name: events.NameViewerAccess, Outcome: "linked"}}
	if !slices.EqualFunc(obs.events, want, equalEvents) {
		t.Fatalf("events = %+v, want %+v", obs.events, want)
	}
}

func TestViewerAccessLoadAccessFailure(t *testing.T) {
	t.Parallel()

	obs := &viewerAccessObserverStub{}
	uc := NewViewerAccessUseCase(viewerAccessServiceStub{
		loadIdentityFn: func(context.Context, int64) (core.UserIdentity, bool, error) {
			return core.UserIdentity{}, false, errors.New("boom")
		},
		buildJoinTargetsFn: func(context.Context, int64, string) (core.JoinTargets, error) {
			return core.JoinTargets{}, nil
		},
	}, obs)

	_, err := uc.LoadAccess(t.Context(), 7)
	if err == nil {
		t.Fatal("LoadAccess() error = nil, want non-nil")
	}
	want := []events.Event{{Name: events.NameViewerAccess, Outcome: "failed"}}
	if !slices.EqualFunc(obs.events, want, equalEvents) {
		t.Fatalf("events = %+v, want %+v", obs.events, want)
	}
}
