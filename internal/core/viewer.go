package core

import (
	"context"
	"fmt"
	"log/slog"

	"imsub/internal/events"
)

// GroupOps abstracts Telegram group membership and invite operations.
type GroupOps interface {
	IsGroupMember(ctx context.Context, groupChatID, telegramUserID int64) bool
	CreateInviteLink(ctx context.Context, groupChatID int64, telegramUserID int64, name string) (string, error)
}

// JoinLink is a transport-agnostic join action for one creator group.
type JoinLink struct {
	CreatorName string
	GroupName   string
	InviteLink  string
}

// JoinTargets contains the viewer's active creators and join links.
type JoinTargets struct {
	ActiveCreatorNames []string
	JoinLinks          []JoinLink
}

type viewerIdentityStore interface {
	UserIdentity(ctx context.Context, telegramUserID int64) (UserIdentity, bool, error)
}

type viewerStore interface {
	viewerIdentityStore
	viewerResolverStore
	viewerTrackedMembershipStore
}

// Viewer orchestrates viewer subscription-to-group eligibility, cache sync,
// and invite creation through focused internal components.
type Viewer struct {
	identity viewerIdentityStore
	resolver *viewerEligibilityResolver
	cache    *viewerMembershipCache
	invites  *viewerInviteBuilder
}

// NewViewer creates a Viewer service with optional logger fallback.
func NewViewer(store viewerStore, group GroupOps, logger *slog.Logger, obs events.EventSink) *Viewer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Viewer{
		identity: store,
		resolver: newViewerEligibilityResolver(store, group, logger, obs),
		cache:    newViewerMembershipCache(store, logger, obs),
		invites:  newViewerInviteBuilder(group, logger, obs),
	}
}

// LoadIdentity returns viewer identity for telegramUserID, if linked.
func (v *Viewer) LoadIdentity(ctx context.Context, telegramUserID int64) (UserIdentity, bool, error) {
	identity, found, err := v.identity.UserIdentity(ctx, telegramUserID)
	if err != nil {
		return UserIdentity{}, false, fmt.Errorf("load user identity: %w", err)
	}
	return identity, found, nil
}

// BuildJoinTargets resolves active subscriptions and invite links for a viewer.
func (v *Viewer) BuildJoinTargets(ctx context.Context, telegramUserID int64, twitchUserID string) (JoinTargets, error) {
	plan, err := v.resolver.resolve(ctx, telegramUserID, twitchUserID)
	if err != nil {
		return JoinTargets{}, err
	}

	v.cache.sync(ctx, telegramUserID, plan)
	joinLinks := v.invites.build(ctx, telegramUserID, plan.inviteGroups)

	return JoinTargets{
		ActiveCreatorNames: plan.activeCreatorNames,
		JoinLinks:          joinLinks,
	}, nil
}

// resolveJoinPlan exposes the resolver seam for focused package tests.
func (v *Viewer) resolveJoinPlan(ctx context.Context, telegramUserID int64, twitchUserID string) (resolvedJoinPlan, error) {
	return v.resolver.resolve(ctx, telegramUserID, twitchUserID)
}
