package core

import (
	"context"
	"log/slog"

	"imsub/internal/events"
)

type viewerInviteOps interface {
	CreateInviteLink(ctx context.Context, groupChatID int64, telegramUserID int64, name string) (string, error)
}

type viewerInviteBuilder struct {
	group viewerInviteOps
	log   *slog.Logger
	obs   events.EventSink
}

func newViewerInviteBuilder(group viewerInviteOps, logger *slog.Logger, obs events.EventSink) *viewerInviteBuilder {
	return &viewerInviteBuilder{
		group: group,
		log:   logger,
		obs:   obs,
	}
}

func (b *viewerInviteBuilder) build(ctx context.Context, telegramUserID int64, groups []resolvedJoinGroup) []JoinLink {
	links := make([]JoinLink, 0, len(groups))
	for _, joinGroup := range groups {
		inviteLink, err := b.group.CreateInviteLink(ctx, joinGroup.group.ChatID, telegramUserID, joinGroup.creatorName)
		if err != nil {
			b.log.Warn("create invite link failed", "creator_name", joinGroup.creatorName, "chat_id", joinGroup.group.ChatID, "error", err)
			b.recordInviteLink(ctx, "failed")
			continue
		}
		links = append(links, JoinLink{
			CreatorName: joinGroup.creatorName,
			GroupName:   joinGroup.group.GroupName,
			InviteLink:  inviteLink,
		})
		b.recordInviteLink(ctx, "ok")
	}
	b.recordJoinTargets(ctx, "join_links", len(links))

	return links
}

func (b *viewerInviteBuilder) recordJoinTargets(ctx context.Context, kind string, count int) {
	if b == nil || b.obs == nil || count <= 0 {
		return
	}
	b.obs.Emit(ctx, events.Event{
		Name:   events.NameViewerJoinTarget,
		Fields: map[string]string{"kind": kind},
		Count:  count,
	})
}

func (b *viewerInviteBuilder) recordInviteLink(ctx context.Context, result string) {
	if b == nil || b.obs == nil {
		return
	}
	b.obs.Emit(ctx, events.Event{
		Name:    events.NameViewerInviteLink,
		Outcome: result,
	})
}
