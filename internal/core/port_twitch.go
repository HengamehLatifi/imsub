package core

import (
	"context"
)

// TwitchAPI abstracts Twitch HTTP API calls, enabling mock-based testing
// without real network requests.
type TwitchAPI interface {
	ExchangeCode(ctx context.Context, code string) (TokenResponse, error)
	RefreshToken(ctx context.Context, refreshToken string) (TokenResponse, error)
	FetchUser(ctx context.Context, userToken string) (id, login, displayName string, err error)
	CreateEventSub(ctx context.Context, broadcasterID, eventType, version string) error
	EnabledEventSubTypes(ctx context.Context, creatorID string) (map[string]bool, error)
	ListSubscriberPage(ctx context.Context, accessToken, broadcasterID, cursor string) (userIDs []string, nextCursor string, err error)
	ListBannedUserPage(ctx context.Context, accessToken, broadcasterID, cursor string) (userIDs []string, nextCursor string, err error)
	ListEventSubs(ctx context.Context, opts ListEventSubsOpts) ([]EventSubSubscription, error)
	DeleteEventSub(ctx context.Context, subscriptionID string) error
}
