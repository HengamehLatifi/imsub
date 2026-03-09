package usecase

import (
	"context"
	"fmt"

	"imsub/internal/core"
	"imsub/internal/events"
)

type viewerAccessService interface {
	LoadIdentity(ctx context.Context, telegramUserID int64) (core.UserIdentity, bool, error)
	BuildJoinTargets(ctx context.Context, telegramUserID int64, twitchUserID string) (core.JoinTargets, error)
}

// ViewerAccessResult is the application-layer result for the linked viewer flow.
type ViewerAccessResult struct {
	HasIdentity bool
	Identity    core.UserIdentity
	Targets     core.JoinTargets
}

// ViewerAccessUseCase coordinates linked-viewer access loading.
type ViewerAccessUseCase struct {
	svc    viewerAccessService
	events events.EventSink
}

// NewViewerAccessUseCase builds a viewer access use case.
func NewViewerAccessUseCase(svc viewerAccessService, sink events.EventSink) *ViewerAccessUseCase {
	return &ViewerAccessUseCase{svc: svc, events: events.EnsureSink(sink)}
}

// LoadIdentity resolves viewer identity without loading join targets.
func (u *ViewerAccessUseCase) LoadIdentity(ctx context.Context, telegramUserID int64) (core.UserIdentity, bool, error) {
	identity, found, err := u.svc.LoadIdentity(ctx, telegramUserID)
	if err != nil {
		return core.UserIdentity{}, false, fmt.Errorf("load viewer identity: %w", err)
	}
	return identity, found, nil
}

// LoadAccess resolves linked viewer identity and join targets.
func (u *ViewerAccessUseCase) LoadAccess(ctx context.Context, telegramUserID int64) (ViewerAccessResult, error) {
	identity, found, err := u.svc.LoadIdentity(ctx, telegramUserID)
	if err != nil {
		u.recordResult(ctx, "failed")
		return ViewerAccessResult{}, fmt.Errorf("load viewer identity: %w", err)
	}
	if !found {
		u.recordResult(ctx, "unlinked")
		return ViewerAccessResult{}, nil
	}

	targets, err := u.svc.BuildJoinTargets(ctx, telegramUserID, identity.TwitchUserID)
	if err != nil {
		u.recordResult(ctx, "failed")
		return ViewerAccessResult{}, fmt.Errorf("build join targets: %w", err)
	}
	u.recordResult(ctx, "linked")
	return ViewerAccessResult{
		HasIdentity: true,
		Identity:    identity,
		Targets:     targets,
	}, nil
}

func (u *ViewerAccessUseCase) recordResult(ctx context.Context, result string) {
	u.events.Emit(ctx, events.Event{
		Name:    events.NameViewerAccess,
		Outcome: result,
	})
}
