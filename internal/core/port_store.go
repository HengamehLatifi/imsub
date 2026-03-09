package core

import (
	"context"
	"time"
)

// LifecycleStore covers process-level health and schema operations.
type LifecycleStore interface {
	Ping(ctx context.Context) error
	Close() error
	EnsureSchema(ctx context.Context) error
}

// IdentityStore covers Telegram/Twitch identity and per-user cleanup.
type IdentityStore interface {
	UserIdentity(ctx context.Context, telegramUserID int64) (UserIdentity, bool, error)
	SaveUserIdentityOnly(ctx context.Context, telegramUserID int64, twitchUserID, twitchLogin, language string) (displacedUserID int64, err error)
	RemoveUserCreatorByTwitch(ctx context.Context, twitchUserID, creatorID string) (telegramUserID int64, found bool, err error)
	DeleteAllUserData(ctx context.Context, telegramUserID int64) error
}

// GroupStore covers managed-group configuration and membership observation/cache state.
type GroupStore interface {
	ManagedGroupByChatID(ctx context.Context, chatID int64) (ManagedGroup, bool, error)
	ListManagedGroups(ctx context.Context) ([]ManagedGroup, error)
	ListManagedGroupsByCreator(ctx context.Context, creatorID string) ([]ManagedGroup, error)
	ListTrackedGroupIDsForUser(ctx context.Context, telegramUserID int64) ([]int64, error)
	UpsertManagedGroup(ctx context.Context, group ManagedGroup) error
	DeleteManagedGroup(ctx context.Context, chatID int64) error
	AddTrackedGroupMember(ctx context.Context, chatID, telegramUserID int64, source string, at time.Time) error
	RemoveTrackedGroupMember(ctx context.Context, chatID, telegramUserID int64) error
	IsTrackedGroupMember(ctx context.Context, chatID, telegramUserID int64) (bool, error)
	UpsertUntrackedGroupMember(ctx context.Context, chatID, telegramUserID int64, source, status string, at time.Time) error
	RemoveUntrackedGroupMember(ctx context.Context, chatID, telegramUserID int64) error
	CountUntrackedGroupMembers(ctx context.Context, chatID int64) (int, error)
}

// CreatorStore covers creator records, auth state, and owner lookup.
type CreatorStore interface {
	Creator(ctx context.Context, creatorID string) (Creator, bool, error)
	ListCreators(ctx context.Context) ([]Creator, error)
	ListActiveCreators(ctx context.Context) ([]Creator, error)
	ListActiveCreatorGroups(ctx context.Context) ([]ActiveCreatorGroups, error)
	OwnedCreatorForUser(ctx context.Context, ownerTelegramID int64) (Creator, bool, error)
	LoadCreatorsByIDs(ctx context.Context, ids []string, filter func(Creator) bool) ([]Creator, error)
	UpsertCreator(ctx context.Context, c Creator) error
	DeleteCreatorData(ctx context.Context, ownerTelegramID int64) (deletedCount int, deletedNames []string, err error)
	UpdateCreatorTokens(ctx context.Context, creatorID, accessToken, refreshToken string) error
	MarkCreatorAuthReconnectRequired(ctx context.Context, creatorID, errorCode string, at time.Time) (transitioned bool, err error)
	MarkCreatorAuthHealthy(ctx context.Context, creatorID string, at time.Time) error
	UpdateCreatorLastSync(ctx context.Context, creatorID string, at time.Time) error
	UpdateCreatorLastReconnectNotice(ctx context.Context, creatorID string, at time.Time) error
	CreatorAuthReconnectRequiredCount(ctx context.Context) (int, error)
}

// OAuthStateStore covers persisted OAuth callback state.
type OAuthStateStore interface {
	SaveOAuthState(ctx context.Context, state string, payload OAuthStatePayload, ttl time.Duration) error
	OAuthState(ctx context.Context, state string) (OAuthStatePayload, error)
	DeleteOAuthState(ctx context.Context, state string) (OAuthStatePayload, error)
}

// SubscriberStore covers subscriber cache reads/writes and dump lifecycle.
type SubscriberStore interface {
	IsCreatorSubscriber(ctx context.Context, creatorID, twitchUserID string) (bool, error)
	AddCreatorSubscriber(ctx context.Context, creatorID, twitchUserID string) error
	RemoveCreatorSubscriber(ctx context.Context, creatorID, twitchUserID string) error
	CreatorSubscriberCount(ctx context.Context, creatorID string) (int64, error)
	NewSubscriberDumpKey(creatorID string) string
	AddToSubscriberDump(ctx context.Context, tmpKey string, userIDs []string) error
	FinalizeSubscriberDump(ctx context.Context, creatorID, tmpKey string, hasData bool) error
	CleanupSubscriberDump(ctx context.Context, tmpKey string)
}

// EventDeduperStore covers webhook/event deduplication.
type EventDeduperStore interface {
	MarkEventProcessed(ctx context.Context, messageID string, ttl time.Duration) (alreadyProcessed bool, err error)
}

// IntegrityStore covers audit and repair operations over derived/indexed state.
type IntegrityStore interface {
	RepairTrackedGroupReverseIndex(ctx context.Context) (indexUsers, repairedUsers, missingLinks, staleLinks int, err error)
	ActiveCreatorIDsWithoutGroup(ctx context.Context, creators []Creator) (int, error)
}

// Store defines the full data access contract for the application.
type Store interface {
	LifecycleStore
	IdentityStore
	GroupStore
	CreatorStore
	OAuthStateStore
	SubscriberStore
	EventDeduperStore
	IntegrityStore
}
