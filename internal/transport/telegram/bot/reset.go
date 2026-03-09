package bot

import (
	"context"
	"fmt"
	"html"
	"strings"

	"imsub/internal/core"
	"imsub/internal/platform/i18n"
	"imsub/internal/transport/telegram/client"
	"imsub/internal/transport/telegram/ui"
	"imsub/internal/usecase"

	"github.com/mymmrac/telego"
	tghandler "github.com/mymmrac/telego/telegohandler"
)

const (
	msgErrReset                = "err_reset"
	msgResetNothingHTML        = "reset_nothing_html"
	msgResetDoneViewerHTML     = "reset_done_viewer_html"
	msgResetDoneCreatorHTML    = "reset_done_creator_html"
	msgResetDoneBothHTML       = "reset_done_both_html"
	msgResetChooseScopeHTML    = "reset_choose_scope_html"
	msgResetConfirmViewerHTML  = "reset_confirm_viewer_html"
	msgResetConfirmCreatorHTML = "reset_confirm_creator_html"
	msgResetConfirmBothHTML    = "reset_confirm_both_html"
	msgResetExitHTML           = "reset_exit_html"
)

type resetConfirmView struct{ text string }

// onResetCommand handles /reset by showing the reset confirmation prompt.
func (c *Controller) onResetCommand(ctx *tghandler.Context, message telego.Message) error {
	lang := i18n.NormalizeLanguage(message.From.LanguageCode)
	c.renderResetPrompt(ctx, message.From.ID, 0, lang, resetOriginCommand)
	return nil
}

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
		c.log().Warn("unsupported reset callback verb", "telegram_user_id", telegramUserID, "verb", action.verb)
		return ""
	default:
		c.log().Warn("unsupported reset callback verb", "telegram_user_id", telegramUserID, "verb", action.verb)
		return ""
	}
}

func (c *Controller) renderResetPrompt(ctx context.Context, telegramUserID int64, editMsgID int, lang string, origin resetOrigin) string {
	scopes, err := c.reset.LoadScopes(ctx, telegramUserID)
	if err != nil {
		view := buildResetErrorView(lang)
		c.reply(ctx, telegramUserID, editMsgID, view.text, &view.opts)
		return view.text
	}

	if view, ok := buildResetPromptView(lang, scopes, origin); ok {
		c.reply(ctx, telegramUserID, editMsgID, view.text, &view.opts)
		return ""
	}

	if scopes.HasIdentity {
		return c.renderResetConfirm(ctx, telegramUserID, editMsgID, lang, origin, resetScopeViewer)
	}
	return c.renderResetConfirm(ctx, telegramUserID, editMsgID, lang, origin, resetScopeCreator)
}

func (c *Controller) renderResetConfirm(ctx context.Context, telegramUserID int64, editMsgID int, lang string, origin resetOrigin, scope resetScope) string {
	scopes, err := c.reset.LoadScopes(ctx, telegramUserID)
	if err != nil {
		view := buildResetErrorView(lang)
		c.reply(ctx, telegramUserID, editMsgID, view.text, &view.opts)
		return view.text
	}

	view := c.buildResetConfirmView(ctx, telegramUserID, lang, scopes, scope)
	if view.text == "" {
		emptyView := buildResetEmptyView(lang)
		c.reply(ctx, telegramUserID, editMsgID, emptyView.text, &emptyView.opts)
		return ""
	}
	replyView := buildResetConfirmReply(lang, view, origin, scope)
	c.reply(ctx, telegramUserID, editMsgID, replyView.text, &replyView.opts)
	return ""
}

func (c *Controller) handleResetBack(ctx context.Context, telegramUserID int64, editMsgID int, lang string, origin resetOrigin) string {
	scopes, err := c.reset.LoadScopes(ctx, telegramUserID)
	if err != nil {
		view := buildMainMenuTextView(lang, msgErrReset)
		c.reply(ctx, telegramUserID, editMsgID, view.text, &view.opts)
		return view.text
	}

	if scopes.HasIdentity && scopes.HasCreator {
		return c.renderResetPrompt(ctx, telegramUserID, editMsgID, lang, origin)
	}

	switch origin {
	case resetOriginViewer:
		return c.handleViewerStart(ctx, telegramUserID, editMsgID, lang)
	case resetOriginCreator:
		return c.handleCreatorStart(ctx, telegramUserID, editMsgID, lang)
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
		return c.handleCreatorStart(ctx, telegramUserID, editMsgID, lang)
	case resetOriginCommand:
		return c.handleResetCancel(ctx, telegramUserID, editMsgID, lang)
	}
	return ""
}

func (c *Controller) handleResetCancel(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	view := buildHTMLTextView(lang, msgResetExitHTML)
	c.reply(ctx, telegramUserID, editMsgID, view.text, &view.opts)
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

func (c *Controller) handleResetViewerCommand(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	return c.executeResetScope(ctx, telegramUserID, editMsgID, lang, usecase.ResetScopeViewer)
}

func (c *Controller) handleResetCreatorCommand(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	return c.executeResetScope(ctx, telegramUserID, editMsgID, lang, usecase.ResetScopeCreator)
}

func (c *Controller) handleResetBothCommand(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	return c.executeResetScope(ctx, telegramUserID, editMsgID, lang, usecase.ResetScopeBoth)
}

func (c *Controller) executeResetScope(ctx context.Context, telegramUserID int64, editMsgID int, lang string, scope usecase.ResetScope) string {
	res, err := c.reset.Execute(ctx, telegramUserID, scope)
	if err != nil {
		view := buildResetErrorView(lang)
		c.reply(ctx, telegramUserID, editMsgID, view.text, &view.opts)
		return view.text
	}
	if res.Empty {
		view := buildResetEmptyView(lang)
		c.reply(ctx, telegramUserID, editMsgID, view.text, &view.opts)
		return ""
	}
	view := buildResetExecutionView(lang, res)
	c.reply(ctx, telegramUserID, editMsgID, view.text, &view.opts)
	return ""
}

func resetChooseScopeText(lang string, scopes core.ScopeState) string {
	return fmt.Sprintf(
		i18n.Translate(lang, msgResetChooseScopeHTML),
		html.EscapeString(scopes.Identity.TwitchLogin),
		html.EscapeString(scopes.Creator.Name),
	)
}

func (c *Controller) buildResetConfirmView(ctx context.Context, telegramUserID int64, lang string, scopes core.ScopeState, scope resetScope) resetConfirmView {
	switch scope {
	case resetScopeViewer:
		if !scopes.HasIdentity {
			return resetConfirmView{}
		}
		return resetConfirmView{
			text: fmt.Sprintf(
				i18n.Translate(lang, msgResetConfirmViewerHTML),
				html.EscapeString(scopes.Identity.TwitchLogin),
				c.resetViewerGroupCount(ctx, telegramUserID),
			),
		}
	case resetScopeCreator:
		if !scopes.HasCreator {
			return resetConfirmView{}
		}
		return resetConfirmView{text: fmt.Sprintf(i18n.Translate(lang, msgResetConfirmCreatorHTML), html.EscapeString(scopes.Creator.Name), 1)}
	case resetScopeBoth:
		if !scopes.HasIdentity && !scopes.HasCreator {
			return resetConfirmView{}
		}
		viewerName := "-"
		if scopes.HasIdentity {
			viewerName = html.EscapeString(scopes.Identity.TwitchLogin)
		}
		creatorName := "-"
		creatorCount := 0
		if scopes.HasCreator {
			creatorName = html.EscapeString(scopes.Creator.Name)
			creatorCount = 1
		}
		return resetConfirmView{
			text: fmt.Sprintf(
				i18n.Translate(lang, msgResetConfirmBothHTML),
				viewerName,
				creatorName,
				creatorCount,
				c.resetViewerGroupCount(ctx, telegramUserID),
			),
		}
	default:
		c.log().Warn("unsupported reset scope", "telegram_user_id", telegramUserID, "scope", scope)
		return resetConfirmView{}
	}
}

func (c *Controller) resetViewerGroupCount(ctx context.Context, telegramUserID int64) int {
	groupCount, err := c.reset.CountViewerGroups(ctx, telegramUserID)
	if err != nil {
		c.log().Warn("count viewer groups failed", "telegram_user_id", telegramUserID, "error", err)
		return 0
	}
	return groupCount
}

func buildResetPromptView(lang string, scopes core.ScopeState, origin resetOrigin) (sharedView, bool) {
	if !scopes.HasIdentity && !scopes.HasCreator {
		return buildResetEmptyView(lang), true
	}
	if !scopes.HasIdentity || !scopes.HasCreator {
		return sharedView{}, false
	}
	return sharedView{
		text: resetChooseScopeText(lang, scopes),
		opts: client.MessageOptions{
			ParseMode: telego.ModeHTML,
			Markup: ui.ResetScopePickerMarkup(
				lang,
				resetPickCallback(origin, resetScopeViewer),
				resetPickCallback(origin, resetScopeCreator),
				resetPickCallback(origin, resetScopeBoth),
				resetPromptBackCallback(origin),
			),
		},
	}, true
}

func buildResetConfirmReply(lang string, view resetConfirmView, origin resetOrigin, scope resetScope) sharedView {
	return sharedView{
		text: view.text,
		opts: client.MessageOptions{
			ParseMode: telego.ModeHTML,
			Markup:    ui.ResetConfirmMarkup(lang, resetExecuteCallback(origin, scope), resetBackCallback(origin)),
		},
	}
}

func buildResetErrorView(lang string) sharedView {
	return sharedView{
		text: i18n.Translate(lang, msgErrReset),
		opts: client.MessageOptions{Markup: viewerMainMenuMarkup(lang)},
	}
}

func buildResetEmptyView(lang string) sharedView {
	return sharedView{
		text: i18n.Translate(lang, msgResetNothingHTML),
		opts: client.MessageOptions{ParseMode: telego.ModeHTML},
	}
}

func buildResetExecutionView(lang string, res usecase.ResetResult) sharedView {
	return sharedView{
		text: renderResetExecutionResult(lang, res),
		opts: client.MessageOptions{ParseMode: telego.ModeHTML},
	}
}

func resetPromptBackCallback(origin resetOrigin) string {
	switch origin {
	case resetOriginViewer, resetOriginCreator:
		return resetMenuCallback(origin)
	case resetOriginCommand:
		return resetCancelCallback(origin)
	}
	return ""
}

func renderResetExecutionResult(lang string, res usecase.ResetResult) string {
	switch res.Scope {
	case usecase.ResetScopeViewer:
		return fmt.Sprintf(i18n.Translate(lang, msgResetDoneViewerHTML), html.EscapeString(res.ViewerLogin), res.GroupCount)
	case usecase.ResetScopeCreator:
		return fmt.Sprintf(i18n.Translate(lang, msgResetDoneCreatorHTML), html.EscapeString(strings.Join(res.DeletedNames, ", ")), res.DeletedCount)
	case usecase.ResetScopeBoth:
		viewerName := "-"
		if res.ViewerLogin != "" {
			viewerName = html.EscapeString(res.ViewerLogin)
		}
		return fmt.Sprintf(
			i18n.Translate(lang, msgResetDoneBothHTML),
			viewerName,
			res.GroupCount,
			html.EscapeString(strings.Join(res.DeletedNames, ", ")),
			res.DeletedCount,
		)
	default:
		return i18n.Translate(lang, msgErrReset)
	}
}
