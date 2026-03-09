package usecase

import (
	"context"
	"errors"
	"fmt"

	"imsub/internal/core"
	"imsub/internal/events"
)

const (
	viewerOAuthResultSuccess             = "success"
	viewerOAuthResultSaveFailed          = "save_failed"
	viewerOAuthResultTokenExchangeFailed = "token_exchange_failed"
	viewerOAuthResultUserInfoFailed      = "userinfo_failed"
)

type viewerOAuthService interface {
	LinkViewer(ctx context.Context, code string, payload core.OAuthStatePayload, lang string) (core.ViewerResult, error)
}

// ViewerOAuthResult is the application-layer result for viewer OAuth completion.
type ViewerOAuthResult struct {
	ResultLabel       string
	TwitchUserID      string
	TwitchLogin       string
	TwitchDisplayName string
	DisplacedUserID   int64
}

// ViewerOAuthUseCase coordinates viewer OAuth completion and outcome events.
type ViewerOAuthUseCase struct {
	svc    viewerOAuthService
	events events.EventSink
}

// NewViewerOAuthUseCase builds a viewer OAuth use case.
func NewViewerOAuthUseCase(svc viewerOAuthService, sink events.EventSink) *ViewerOAuthUseCase {
	return &ViewerOAuthUseCase{svc: svc, events: events.EnsureSink(sink)}
}

// Complete finishes viewer OAuth linking and emits a stable outcome event.
func (u *ViewerOAuthUseCase) Complete(ctx context.Context, code string, payload core.OAuthStatePayload, lang string) (ViewerOAuthResult, error) {
	res, err := u.svc.LinkViewer(ctx, code, payload, lang)
	if err != nil {
		label := viewerOAuthResultSaveFailed
		var fe *core.FlowError
		if errors.As(err, &fe) {
			switch fe.Kind {
			case core.KindTokenExchange:
				label = viewerOAuthResultTokenExchangeFailed
			case core.KindUserInfo:
				label = viewerOAuthResultUserInfoFailed
			case core.KindSave, core.KindScopeMissing, core.KindStore, core.KindCreatorMismatch:
				label = viewerOAuthResultSaveFailed
			}
		}
		u.recordResult(ctx, label)
		return ViewerOAuthResult{ResultLabel: label}, fmt.Errorf("complete viewer oauth: %w", err)
	}

	u.recordResult(ctx, viewerOAuthResultSuccess)
	return ViewerOAuthResult{
		ResultLabel:       viewerOAuthResultSuccess,
		TwitchUserID:      res.TwitchUserID,
		TwitchLogin:       res.TwitchLogin,
		TwitchDisplayName: res.TwitchDisplayName,
		DisplacedUserID:   res.DisplacedUserID,
	}, nil
}

func (u *ViewerOAuthUseCase) recordResult(ctx context.Context, result string) {
	u.events.Emit(ctx, events.Event{
		Name:    events.NameViewerOAuth,
		Outcome: result,
	})
}
