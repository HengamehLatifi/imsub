package core

import (
	"context"
	"fmt"
	"time"
)

type creatorTokenStore interface {
	UpdateCreatorTokens(ctx context.Context, creatorID, accessToken, refreshToken string, grantedScopes []string) error
	MarkCreatorAuthHealthy(ctx context.Context, creatorID string, at time.Time) error
}

func refreshCreatorAccessToken(ctx context.Context, creator Creator, twitch TwitchAPI, store creatorTokenStore, emit func(string)) (Creator, error) {
	tok, err := twitch.RefreshToken(ctx, creator.RefreshToken)
	if err != nil {
		if emit != nil {
			emit("failed")
		}
		return creator, fmt.Errorf("refresh token call: %w", err)
	}
	if emit != nil {
		emit("ok")
	}
	if err := store.UpdateCreatorTokens(ctx, creator.ID, tok.AccessToken, tok.RefreshToken, tok.Scope); err != nil {
		return creator, fmt.Errorf("update creator tokens in store: %w", err)
	}
	now := time.Now().UTC()
	if creator.AuthStatus == CreatorAuthReconnectRequired {
		if err := store.MarkCreatorAuthHealthy(ctx, creator.ID, now); err != nil {
			return creator, fmt.Errorf("mark creator auth healthy: %w", err)
		}
	}
	creator.AccessToken = tok.AccessToken
	if tok.RefreshToken != "" {
		creator.RefreshToken = tok.RefreshToken
	}
	if len(tok.Scope) > 0 {
		creator.GrantedScopes = append([]string(nil), tok.Scope...)
	}
	creator.AuthStatus = CreatorAuthHealthy
	creator.AuthErrorCode = ""
	creator.AuthStatusAt = now
	return creator, nil
}
