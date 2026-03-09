// Package application defines the composition-facing application runtime,
// organized around command and query boundaries.
package application

import (
	"context"

	"imsub/internal/core"
	"imsub/internal/operator"
	"imsub/internal/usecase"
)

// Dependencies configure the base application runtime.
type Dependencies struct {
	CreatorStatus       *usecase.CreatorStatusUseCase
	ViewerOAuth         *usecase.ViewerOAuthUseCase
	CreatorOAuth        *usecase.CreatorOAuthUseCase
	GroupRegistration   *usecase.GroupRegistrationUseCase
	GroupUnregistration *usecase.GroupUnregistrationUseCase
	CreatorActivation   *usecase.CreatorActivationUseCase
	SubscriptionEnd     *usecase.SubscriptionEndUseCase
	NewViewerAccess     func(groupOps core.GroupOps) *usecase.ViewerAccessUseCase
	NewReset            func(kick func(ctx context.Context, groupChatID, telegramUserID int64) error) *usecase.ResetUseCase
	OperatorReadModel   *operator.ReadModel
}

// Runtime is the base application boundary shared across transports.
type Runtime struct {
	CreatorStatus       *usecase.CreatorStatusUseCase
	ViewerOAuth         *usecase.ViewerOAuthUseCase
	CreatorOAuth        *usecase.CreatorOAuthUseCase
	GroupRegistration   *usecase.GroupRegistrationUseCase
	GroupUnregistration *usecase.GroupUnregistrationUseCase
	CreatorActivation   *usecase.CreatorActivationUseCase
	SubscriptionEnd     *usecase.SubscriptionEndUseCase
	Operator            *operator.ReadModel

	newViewerAccess func(groupOps core.GroupOps) *usecase.ViewerAccessUseCase
	newReset        func(kick func(ctx context.Context, groupChatID, telegramUserID int64) error) *usecase.ResetUseCase
}

// TelegramRuntime is the application boundary bound to Telegram-specific ports.
type TelegramRuntime struct {
	CreatorStatus       *usecase.CreatorStatusUseCase
	ViewerOAuth         *usecase.ViewerOAuthUseCase
	CreatorOAuth        *usecase.CreatorOAuthUseCase
	ViewerAccess        *usecase.ViewerAccessUseCase
	GroupRegistration   *usecase.GroupRegistrationUseCase
	GroupUnregistration *usecase.GroupUnregistrationUseCase
	CreatorActivation   *usecase.CreatorActivationUseCase
	SubscriptionEnd     *usecase.SubscriptionEndUseCase
	Reset               *usecase.ResetUseCase
	Operator            *operator.ReadModel
}

// NewRuntime builds the base application runtime.
func NewRuntime(deps Dependencies) *Runtime {
	return &Runtime{
		CreatorStatus:       deps.CreatorStatus,
		ViewerOAuth:         deps.ViewerOAuth,
		CreatorOAuth:        deps.CreatorOAuth,
		GroupRegistration:   deps.GroupRegistration,
		GroupUnregistration: deps.GroupUnregistration,
		CreatorActivation:   deps.CreatorActivation,
		SubscriptionEnd:     deps.SubscriptionEnd,
		Operator:            deps.OperatorReadModel,
		newViewerAccess:     deps.NewViewerAccess,
		newReset:            deps.NewReset,
	}
}

// BindTelegram binds transport-specific ports needed for Telegram viewer/reset flows.
func (r *Runtime) BindTelegram(groupOps core.GroupOps, kick func(ctx context.Context, groupChatID, telegramUserID int64) error) *TelegramRuntime {
	if r == nil {
		return nil
	}
	var viewerAccess *usecase.ViewerAccessUseCase
	if r.newViewerAccess != nil {
		viewerAccess = r.newViewerAccess(groupOps)
	}
	var reset *usecase.ResetUseCase
	if r.newReset != nil {
		reset = r.newReset(kick)
	}
	return &TelegramRuntime{
		CreatorStatus:       r.CreatorStatus,
		ViewerOAuth:         r.ViewerOAuth,
		CreatorOAuth:        r.CreatorOAuth,
		ViewerAccess:        viewerAccess,
		GroupRegistration:   r.GroupRegistration,
		GroupUnregistration: r.GroupUnregistration,
		CreatorActivation:   r.CreatorActivation,
		SubscriptionEnd:     r.SubscriptionEnd,
		Reset:               reset,
		Operator:            r.Operator,
	}
}
