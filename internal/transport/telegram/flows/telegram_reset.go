package flows

import (
	"context"

	"imsub/internal/platform/i18n"
	"imsub/internal/transport/telegram/client"
	"imsub/internal/transport/telegram/ui"

	"github.com/mymmrac/telego"
)

func (c *Controller) handleResetAction(ctx context.Context, telegramUserID int64, editMsgID int, lang string, action callbackAction) string {
	switch action.verb {
	case callbackVerbOpen:
		return c.renderResetPrompt(ctx, telegramUserID, editMsgID, lang, action.origin)
	case callbackVerbPick:
		return c.renderResetConfirm(ctx, telegramUserID, editMsgID, lang, action.origin, action.scope)
	case callbackVerbBack:
		return c.handleResetBack(ctx, telegramUserID, editMsgID, lang, action.origin)
	case callbackVerbMenu:
		return c.handleResetBackToMenu(ctx, telegramUserID, editMsgID, lang, action.origin)
	case callbackVerbCancel:
		return c.handleResetCancel(ctx, telegramUserID, editMsgID, lang)
	case callbackVerbExecute:
		return c.executeReset(ctx, telegramUserID, editMsgID, lang, action.scope)
	case callbackVerbRefresh, callbackVerbRegister, callbackVerbReconnect:
		// parseCallbackAction rejects these verbs for reset callbacks.
		c.log().Warn("unsupported reset callback verb", "telegram_user_id", telegramUserID, "verb", action.verb)
		return ""
	default:
		c.log().Warn("unsupported reset callback verb", "telegram_user_id", telegramUserID, "verb", action.verb)
		return ""
	}
}

// renderResetPrompt is the entry point for /reset and reset callbacks.
func (c *Controller) renderResetPrompt(ctx context.Context, telegramUserID int64, editMsgID int, lang string, origin resetOrigin) string {
	scopes, err := c.resetSvc.LoadScopes(ctx, telegramUserID)
	if err != nil {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgErrReset), &client.MessageOptions{Markup: viewerMainMenuMarkup(lang)})
		return i18n.Translate(lang, msgErrReset)
	}

	if !scopes.HasIdentity && !scopes.HasCreator {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgResetNothingHTML), &client.MessageOptions{ParseMode: telego.ModeHTML})
		return ""
	}

	if scopes.HasIdentity && scopes.HasCreator {
		c.reply(ctx, telegramUserID, editMsgID, resetChooseScopeText(lang, scopes), &client.MessageOptions{
			ParseMode: telego.ModeHTML,
			Markup: ui.ResetScopePickerMarkup(
				lang,
				resetPickCallback(origin, resetScopeViewer),
				resetPickCallback(origin, resetScopeCreator),
				resetPickCallback(origin, resetScopeBoth),
				c.resetPromptBackCallback(origin),
			),
		})
		return ""
	}

	if scopes.HasIdentity {
		return c.renderResetConfirm(ctx, telegramUserID, editMsgID, lang, origin, resetScopeViewer)
	}
	return c.renderResetConfirm(ctx, telegramUserID, editMsgID, lang, origin, resetScopeCreator)
}

func (c *Controller) renderResetConfirm(ctx context.Context, telegramUserID int64, editMsgID int, lang string, origin resetOrigin, scope resetScope) string {
	scopes, err := c.resetSvc.LoadScopes(ctx, telegramUserID)
	if err != nil {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgErrReset), &client.MessageOptions{Markup: viewerMainMenuMarkup(lang)})
		return i18n.Translate(lang, msgErrReset)
	}

	view := c.buildResetConfirmView(ctx, telegramUserID, lang, scopes, scope)
	if view.text == "" {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgResetNothingHTML), &client.MessageOptions{ParseMode: telego.ModeHTML})
		return ""
	}
	c.reply(ctx, telegramUserID, editMsgID, view.text, &client.MessageOptions{
		ParseMode: telego.ModeHTML,
		Markup:    ui.ResetConfirmMarkup(lang, resetExecuteCallback(origin, scope), resetBackCallback(origin)),
	})
	return ""
}

func (c *Controller) handleResetBack(ctx context.Context, telegramUserID int64, editMsgID int, lang string, origin resetOrigin) string {
	scopes, err := c.resetSvc.LoadScopes(ctx, telegramUserID)
	if err != nil {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgErrReset), &client.MessageOptions{Markup: viewerMainMenuMarkup(lang)})
		return i18n.Translate(lang, msgErrReset)
	}

	if scopes.HasIdentity && scopes.HasCreator {
		return c.renderResetPrompt(ctx, telegramUserID, editMsgID, lang, origin)
	}

	switch origin {
	case resetOriginViewer:
		return c.handleViewerStart(ctx, telegramUserID, editMsgID, lang)
	case resetOriginCreator:
		return c.handleCreatorRegistrationStart(ctx, telegramUserID, editMsgID, lang)
	case resetOriginCommand:
		return c.handleResetCancel(ctx, telegramUserID, editMsgID, lang)
	}
	return ""
}

func (c *Controller) handleResetBackToMenu(ctx context.Context, telegramUserID int64, editMsgID int, lang string, origin resetOrigin) string {
	switch origin {
	case resetOriginViewer:
		return c.handleViewerStart(ctx, telegramUserID, editMsgID, lang)
	case resetOriginCreator:
		return c.handleCreatorRegistrationStart(ctx, telegramUserID, editMsgID, lang)
	case resetOriginCommand:
		return c.handleResetCancel(ctx, telegramUserID, editMsgID, lang)
	}
	return ""
}

func (c *Controller) resetPromptBackCallback(origin resetOrigin) string {
	switch origin {
	case resetOriginViewer, resetOriginCreator:
		return resetMenuCallback(origin)
	case resetOriginCommand:
		return resetCancelCallback(origin)
	}
	return ""
}

// handleResetCancel cleanly aborts the reset flow, removing buttons and showing a safe message.
func (c *Controller) handleResetCancel(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgResetExitHTML), &client.MessageOptions{ParseMode: telego.ModeHTML})
	return ""
}

func (c *Controller) executeReset(ctx context.Context, telegramUserID int64, editMsgID int, lang string, scope resetScope) string {
	switch scope {
	case resetScopeViewer:
		return c.handleResetViewerCommand(ctx, telegramUserID, editMsgID, lang)
	case resetScopeCreator:
		return c.handleResetCreatorCommand(ctx, telegramUserID, editMsgID, lang)
	case resetScopeBoth:
		return c.handleResetBothCommand(ctx, telegramUserID, editMsgID, lang)
	default:
		c.log().Warn("unsupported reset execute scope", "telegram_user_id", telegramUserID, "scope", scope)
		return ""
	}
}
