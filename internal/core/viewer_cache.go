package core

import (
	"context"
	"log/slog"
	"time"

	"imsub/internal/events"
)

type viewerTrackedMembershipStore interface {
	AddTrackedGroupMember(ctx context.Context, chatID, telegramUserID int64, source string, at time.Time) error
	RemoveTrackedGroupMember(ctx context.Context, chatID, telegramUserID int64) error
}

type viewerMembershipCache struct {
	store viewerTrackedMembershipStore
	log   *slog.Logger
	obs   events.EventSink
}

func newViewerMembershipCache(store viewerTrackedMembershipStore, logger *slog.Logger, obs events.EventSink) *viewerMembershipCache {
	return &viewerMembershipCache{
		store: store,
		log:   logger,
		obs:   obs,
	}
}

func (c *viewerMembershipCache) sync(ctx context.Context, telegramUserID int64, plan resolvedJoinPlan) {
	now := time.Now().UTC()

	for _, groupChatID := range plan.untrackedGroups {
		if err := c.store.RemoveTrackedGroupMember(ctx, groupChatID, telegramUserID); err != nil {
			c.log.Warn("remove tracked group member failed", "telegram_user_id", telegramUserID, "chat_id", groupChatID, "error", err)
		}
	}
	for _, joinGroup := range plan.inviteGroups {
		if err := c.store.AddTrackedGroupMember(ctx, joinGroup.group.ChatID, telegramUserID, "viewer_join_target", now); err != nil {
			c.log.Warn("add tracked group member failed", "telegram_user_id", telegramUserID, "chat_id", joinGroup.group.ChatID, "error", err)
		}
	}

	c.recordJoinTargets(ctx, "cache_removes", len(plan.untrackedGroups))
	c.recordJoinTargets(ctx, "cache_adds", len(plan.inviteGroups))
}

func (c *viewerMembershipCache) recordJoinTargets(ctx context.Context, kind string, count int) {
	if c == nil || c.obs == nil || count <= 0 {
		return
	}
	c.obs.Emit(ctx, events.Event{
		Name:   events.NameViewerJoinTarget,
		Fields: map[string]string{"kind": kind},
		Count:  count,
	})
}
