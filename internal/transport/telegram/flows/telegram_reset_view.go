package flows

import (
	"context"
	"fmt"
	"html"

	"imsub/internal/core"
	"imsub/internal/platform/i18n"
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
	groupCount, err := c.resetSvc.CountViewerGroups(ctx, telegramUserID)
	if err != nil {
		c.log().Warn("count viewer groups failed", "telegram_user_id", telegramUserID, "error", err)
		return 0
	}
	return groupCount
}
