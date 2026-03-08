package core

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"time"
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

type viewerStore interface {
	UserIdentity(ctx context.Context, telegramUserID int64) (UserIdentity, bool, error)
	ListActiveCreators(ctx context.Context) ([]Creator, error)
	ListManagedGroupsByCreator(ctx context.Context, creatorID string) ([]ManagedGroup, error)
	IsCreatorSubscriber(ctx context.Context, creatorID, twitchUserID string) (bool, error)
	AddTrackedGroupMember(ctx context.Context, chatID, telegramUserID int64, source string, at time.Time) error
	RemoveTrackedGroupMember(ctx context.Context, chatID, telegramUserID int64) error
}

// Viewer owns viewer subscription-to-group eligibility logic.
type Viewer struct {
	store viewerStore
	group GroupOps
	log   *slog.Logger
}

// NewViewer creates a Viewer service with optional logger fallback.
func NewViewer(store viewerStore, group GroupOps, logger *slog.Logger) *Viewer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Viewer{
		store: store,
		group: group,
		log:   logger,
	}
}

// LoadIdentity returns viewer identity for telegramUserID, if linked.
func (v *Viewer) LoadIdentity(ctx context.Context, telegramUserID int64) (UserIdentity, bool, error) {
	identity, found, err := v.store.UserIdentity(ctx, telegramUserID)
	if err != nil {
		return UserIdentity{}, false, fmt.Errorf("load user identity: %w", err)
	}
	return identity, found, nil
}

// BuildJoinTargets resolves active subscriptions and invite links for a viewer.
func (v *Viewer) BuildJoinTargets(ctx context.Context, telegramUserID int64, twitchUserID string) (JoinTargets, error) {
	creators, err := v.store.ListActiveCreators(ctx)
	if err != nil {
		v.log.Warn("build join targets list active creators failed", "error", err)
		return JoinTargets{}, fmt.Errorf("list active creators: %w", err)
	}

	out := JoinTargets{
		ActiveCreatorNames: make([]string, 0, len(creators)),
		JoinLinks:          make([]JoinLink, 0, len(creators)),
	}
	for _, creator := range creators {
		groups, err := v.store.ListManagedGroupsByCreator(ctx, creator.ID)
		if err != nil {
			v.log.Warn("build join targets list managed groups failed", "creator_id", creator.ID, "error", err)
			continue
		}
		isSubscriber, err := v.store.IsCreatorSubscriber(ctx, creator.ID, twitchUserID)
		if err != nil {
			v.log.Warn("build join targets is creator subscriber failed", "creator_id", creator.ID, "error", err)
			continue
		}
		if !isSubscriber {
			for _, group := range groups {
				if err := v.store.RemoveTrackedGroupMember(ctx, group.ChatID, telegramUserID); err != nil {
					v.log.Warn("remove tracked group member failed", "telegram_user_id", telegramUserID, "chat_id", group.ChatID, "error", err)
				}
			}
			continue
		}

		out.ActiveCreatorNames = append(out.ActiveCreatorNames, creator.Name)
		for _, group := range groups {
			if v.group.IsGroupMember(ctx, group.ChatID, telegramUserID) {
				continue
			}
			if err := v.store.AddTrackedGroupMember(ctx, group.ChatID, telegramUserID, "viewer_join_target", time.Now().UTC()); err != nil {
				v.log.Warn("add tracked group member failed", "telegram_user_id", telegramUserID, "chat_id", group.ChatID, "error", err)
			}
			inviteLink, err := v.group.CreateInviteLink(ctx, group.ChatID, telegramUserID, creator.Name)
			if err != nil {
				v.log.Warn("create invite link failed", "creator_id", creator.ID, "chat_id", group.ChatID, "error", err)
				continue
			}
			out.JoinLinks = append(out.JoinLinks, JoinLink{
				CreatorName: creator.Name,
				GroupName:   group.GroupName,
				InviteLink:  inviteLink,
			})
		}
	}

	slices.Sort(out.ActiveCreatorNames)
	return out, nil
}
