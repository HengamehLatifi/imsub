package core

import (
	"context"
	"fmt"

	"imsub/internal/platform/i18n"
)

type subscriptionStore interface {
	RemoveCreatorSubscriber(ctx context.Context, creatorID, twitchUserID string) error
	Creator(ctx context.Context, creatorID string) (Creator, bool, error)
	ListManagedGroupsByCreator(ctx context.Context, creatorID string) ([]ManagedGroup, error)
	RemoveUserCreatorByTwitch(ctx context.Context, twitchUserID, creatorID string) (telegramUserID int64, found bool, err error)
	UserIdentity(ctx context.Context, telegramUserID int64) (UserIdentity, bool, error)
}

// SubscriptionService handles subscriber-end processing and derived notifications.
type SubscriptionService struct {
	store subscriptionStore
}

// NewSubscriptionService creates a subscription service.
func NewSubscriptionService(store subscriptionStore) *SubscriptionService {
	return &SubscriptionService{store: store}
}

// EndResult captures the direct result of processing a sub-end event.
type EndResult struct {
	TelegramUserID   int64
	Found            bool
	GroupChatIDs     []int64
	BroadcasterLogin string
	IdentityLanguage string
	HasIdentityLang  bool
}

// PreparedEnd is transport-ready data for subscription-end side effects.
type PreparedEnd struct {
	Found            bool
	TelegramUserID   int64
	GroupChatIDs     []int64
	Language         string
	BroadcasterLogin string
	ViewerLogin      string
}

// ProcessEnd applies subscriber-end effects and returns raw domain outcomes.
func (s *SubscriptionService) ProcessEnd(ctx context.Context, broadcasterID, broadcasterLogin, twitchUserID string) (EndResult, error) {
	if err := s.store.RemoveCreatorSubscriber(ctx, broadcasterID, twitchUserID); err != nil {
		return EndResult{}, fmt.Errorf("remove creator subscriber: %w", err)
	}

	creator, creatorFound, err := s.store.Creator(ctx, broadcasterID)
	if err != nil {
		return EndResult{}, fmt.Errorf("load creator: %w", err)
	}
	if broadcasterLogin == "" && creatorFound {
		broadcasterLogin = creator.TwitchLogin
	}
	groups, err := s.store.ListManagedGroupsByCreator(ctx, broadcasterID)
	if err != nil {
		return EndResult{}, fmt.Errorf("list managed groups by creator: %w", err)
	}

	telegramUserID, found, err := s.store.RemoveUserCreatorByTwitch(ctx, twitchUserID, broadcasterID)
	if err != nil {
		return EndResult{}, fmt.Errorf("remove user creator by twitch: %w", err)
	}
	if !found {
		return EndResult{Found: false}, nil
	}

	identity, hasIdentity, err := s.store.UserIdentity(ctx, telegramUserID)
	if err != nil {
		return EndResult{}, fmt.Errorf("load user identity: %w", err)
	}
	out := EndResult{
		TelegramUserID:   telegramUserID,
		Found:            true,
		GroupChatIDs:     make([]int64, 0, len(groups)),
		BroadcasterLogin: broadcasterLogin,
	}
	for _, group := range groups {
		out.GroupChatIDs = append(out.GroupChatIDs, group.ChatID)
	}
	if hasIdentity {
		out.IdentityLanguage = identity.Language
		out.HasIdentityLang = identity.Language != ""
	}
	return out, nil
}

// PrepareEnd converts subscriber-end outcomes into transport-ready data.
func (s *SubscriptionService) PrepareEnd(ctx context.Context, broadcasterID, broadcasterLogin, twitchUserID, twitchLogin string) (PreparedEnd, error) {
	res, err := s.ProcessEnd(ctx, broadcasterID, broadcasterLogin, twitchUserID)
	if err != nil {
		return PreparedEnd{}, fmt.Errorf("process end: %w", err)
	}
	if !res.Found {
		return PreparedEnd{Found: false}, nil
	}

	lang := "en"
	if res.HasIdentityLang {
		lang = i18n.NormalizeLanguage(res.IdentityLanguage)
	}

	return PreparedEnd{
		Found:            true,
		TelegramUserID:   res.TelegramUserID,
		GroupChatIDs:     res.GroupChatIDs,
		Language:         lang,
		BroadcasterLogin: res.BroadcasterLogin,
		ViewerLogin:      twitchLogin,
	}, nil
}
