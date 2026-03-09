package flows

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

func resetChooseScopeText(lang string, scopes core.ScopeState) string {
	return fmt.Sprintf(
		i18n.Translate(lang, msgResetChooseScopeHTML),
		html.EscapeString(scopes.Identity.TwitchLogin),
		html.EscapeString(scopes.Creator.Name),
	)
}

func (c *Controller) buildResetConfirmView(
	ctx context.Context,
	telegramUserID int64,
	lang string,
	scopes core.ScopeState,
	scope resetScope,
) resetConfirmView {
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
		return resetConfirmView{
			text: fmt.Sprintf(i18n.Translate(lang, msgResetConfirmCreatorHTML), html.EscapeString(scopes.Creator.Name), 1),
		}
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
	groupCount, err := c.app.Reset.CountViewerGroups(ctx, telegramUserID)
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
		return fmt.Sprintf(
			i18n.Translate(lang, msgResetDoneCreatorHTML),
			html.EscapeString(strings.Join(res.DeletedNames, ", ")),
			res.DeletedCount,
		)
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
