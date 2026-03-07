package flows

import (
	"context"
	"fmt"
	"html"

	"imsub/internal/platform/i18n"
	"imsub/internal/transport/telegram/client"
	"imsub/internal/transport/telegram/ui"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// handleResetPrompt is the entry point for /reset and reset callbacks.
//
// It loads the user's viewer identity and creator ownership, then either:
//   - renders "nothing to reset" if both are absent,
//   - shows a scope picker (viewer / creator / both) if both exist,
//   - or delegates to the single-scope confirmation prompt.
func (c *Controller) handleResetPrompt(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	scopes, err := c.resetSvc.LoadScopes(ctx, telegramUserID)
	if err != nil {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgErrReset), &client.MessageOptions{Markup: ui.MainMenuMarkup(lang)})
		return i18n.Translate(lang, msgErrReset)
	}

	// Nothing to reset: return a clean informational message and stop the flow.
	if !scopes.HasIdentity && !scopes.HasCreator {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgResetNothingHTML), &client.MessageOptions{ParseMode: telego.ModeHTML})
		return ""
	}

	// Both scopes exist: ask user explicitly which scope(s) they want to remove.
	if scopes.HasIdentity && scopes.HasCreator {
		viewerName := html.EscapeString(scopes.Identity.TwitchLogin)
		creatorList := html.EscapeString(scopes.Creator.Name)
		text := fmt.Sprintf(i18n.Translate(lang, msgResetChooseScopeHTML), viewerName, creatorList)
		// When entered from /reset (editMsgID==0), "Back" exits to the cancelled screen.
		// When entered from a menu callback, "Back" returns to the originating menu.
		backAction := ui.ActionResetPickerBack
		if editMsgID == 0 {
			backAction = ui.ActionResetPickerCancel
		}
		markup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(ui.DeleteButton(i18n.Translate(lang, btnResetViewerData), ui.ActionResetPickViewer)),
			tu.InlineKeyboardRow(ui.DeleteButton(i18n.Translate(lang, btnResetCreatorData), ui.ActionResetPickCreator)),
			tu.InlineKeyboardRow(ui.DeleteButton(i18n.Translate(lang, btnResetAllData), ui.ActionResetPickBoth)),
			tu.InlineKeyboardRow(ui.BackButton(i18n.Translate(lang, btnBack), backAction)),
		)
		c.reply(ctx, telegramUserID, editMsgID, text, &client.MessageOptions{ParseMode: telego.ModeHTML, Markup: markup})
		return ""
	}

	// Single-scope case: jump straight to the matching confirmation screen.
	if scopes.HasIdentity {
		return c.handleResetViewerConfirmPrompt(ctx, telegramUserID, editMsgID, lang)
	}
	return c.handleResetCreatorConfirmPrompt(ctx, telegramUserID, editMsgID, lang)
}

// handleResetBack handles the back button from reset confirmation screens.
// If the user has both scopes, it returns to the scope picker.
// If they have only one scope, it returns to the originating menu.
func (c *Controller) handleResetBack(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	scopes, err := c.resetSvc.LoadScopes(ctx, telegramUserID)
	if err != nil {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgErrReset), &client.MessageOptions{Markup: ui.MainMenuMarkup(lang)})
		return i18n.Translate(lang, msgErrReset)
	}

	// If the user has both scopes, "Back" means go back to the picker.
	if scopes.HasIdentity && scopes.HasCreator {
		return c.handleResetPrompt(ctx, telegramUserID, editMsgID, lang)
	}

	// If the user is only a Creator, return to the Creator menu.
	if !scopes.HasIdentity && scopes.HasCreator {
		return c.handleCreatorRegistrationStart(ctx, telegramUserID, editMsgID, lang)
	}

	// Otherwise, return to the Viewer menu (the default /start behavior).
	return c.handleViewerStart(ctx, telegramUserID, editMsgID, lang)
}

// handleResetBackToMenu returns from the reset scope picker to the originating menu.
// If the user has creator data, it returns to the creator status screen;
// otherwise it returns to the viewer/start screen.
func (c *Controller) handleResetBackToMenu(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	scopes, err := c.resetSvc.LoadScopes(ctx, telegramUserID)
	if err != nil {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgErrReset), &client.MessageOptions{Markup: ui.MainMenuMarkup(lang)})
		return i18n.Translate(lang, msgErrReset)
	}
	if scopes.HasCreator {
		return c.handleCreatorRegistrationStart(ctx, telegramUserID, editMsgID, lang)
	}
	return c.handleViewerStart(ctx, telegramUserID, editMsgID, lang)
}

// handleResetCancel cleanly aborts the reset flow, removing buttons and showing a safe message.
func (c *Controller) handleResetCancel(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgResetExitHTML), &client.MessageOptions{ParseMode: telego.ModeHTML})
	return ""
}

// handleResetViewerConfirmPrompt renders a confirmation screen for viewer data
// deletion, showing the linked Twitch account and estimated group removal count.
//
// Group counting scans all active creators — O(N) Redis membership checks.
func (c *Controller) handleResetViewerConfirmPrompt(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	scopes, err := c.resetSvc.LoadScopes(ctx, telegramUserID)
	if err != nil {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgErrReset), &client.MessageOptions{Markup: ui.MainMenuMarkup(lang)})
		return i18n.Translate(lang, msgErrReset)
	}
	// If viewer data disappeared meanwhile, degrade to "nothing to reset".
	if !scopes.HasIdentity {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgResetNothingHTML), &client.MessageOptions{ParseMode: telego.ModeHTML})
		return ""
	}
	// Estimate downstream impact (how many groups might revoke access).
	groupCount, err := c.resetSvc.CountViewerGroups(ctx, telegramUserID)
	if err != nil {
		c.log().Warn("countSubLinkedGroupsForUser failed", "telegram_user_id", telegramUserID, "error", err)
		groupCount = 0
	}
	// Render a destructive-action confirmation with explicit cancel path.
	text := fmt.Sprintf(
		i18n.Translate(lang, msgResetConfirmViewerHTML),
		html.EscapeString(scopes.Identity.TwitchLogin),
		groupCount,
	)
	markup := tu.InlineKeyboard(
		tu.InlineKeyboardRow(ui.DeleteButton(i18n.Translate(lang, btnResetConfirm), ui.ActionResetDoViewer)),
		tu.InlineKeyboardRow(ui.BackButton(i18n.Translate(lang, btnBack), ui.ActionResetConfirmBack)),
	)
	c.reply(ctx, telegramUserID, editMsgID, text, &client.MessageOptions{ParseMode: telego.ModeHTML, Markup: markup})
	return ""
}

// handleResetCreatorConfirmPrompt renders a confirmation screen for creator
// data deletion. O(1) with the current single-owner model.
func (c *Controller) handleResetCreatorConfirmPrompt(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	scopes, err := c.resetSvc.LoadScopes(ctx, telegramUserID)
	if err != nil {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgErrReset), &client.MessageOptions{Markup: ui.MainMenuMarkup(lang)})
		return i18n.Translate(lang, msgErrReset)
	}
	// If no creator record exists, exit with informational message.
	if !scopes.HasCreator {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgResetNothingHTML), &client.MessageOptions{ParseMode: telego.ModeHTML})
		return ""
	}
	// Render creator-only destructive confirmation.
	creatorList := html.EscapeString(scopes.Creator.Name)
	text := fmt.Sprintf(i18n.Translate(lang, msgResetConfirmCreatorHTML), creatorList, 1)
	markup := tu.InlineKeyboard(
		tu.InlineKeyboardRow(ui.DeleteButton(i18n.Translate(lang, btnResetConfirm), ui.ActionResetDoCreator)),
		tu.InlineKeyboardRow(ui.BackButton(i18n.Translate(lang, btnBack), ui.ActionResetConfirmBack)),
	)
	c.reply(ctx, telegramUserID, editMsgID, text, &client.MessageOptions{ParseMode: telego.ModeHTML, Markup: markup})
	return ""
}

// handleResetBothConfirmPrompt renders a confirmation screen for deleting both
// viewer and creator data. Linked-group counting dominates the cost.
func (c *Controller) handleResetBothConfirmPrompt(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	scopes, err := c.resetSvc.LoadScopes(ctx, telegramUserID)
	if err != nil {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgErrReset), &client.MessageOptions{Markup: ui.MainMenuMarkup(lang)})
		return i18n.Translate(lang, msgErrReset)
	}
	// If both scopes are already absent, there is nothing actionable.
	if !scopes.HasIdentity && !scopes.HasCreator {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgResetNothingHTML), &client.MessageOptions{ParseMode: telego.ModeHTML})
		return ""
	}
	// Count possible group-side effects; degrade gracefully if counting fails.
	groupCount, err := c.resetSvc.CountViewerGroups(ctx, telegramUserID)
	if err != nil {
		c.log().Warn("countSubLinkedGroupsForUser failed", "telegram_user_id", telegramUserID, "error", err)
		groupCount = 0
	}
	// Build display placeholders for potentially missing scopes.
	viewerName := "-"
	if scopes.HasIdentity {
		viewerName = html.EscapeString(scopes.Identity.TwitchLogin)
	}
	creatorList := "-"
	creatorCount := 0
	if scopes.HasCreator {
		creatorList = html.EscapeString(scopes.Creator.Name)
		creatorCount = 1
	}
	// Render the final full-reset confirmation.
	text := fmt.Sprintf(
		i18n.Translate(lang, msgResetConfirmBothHTML),
		viewerName,
		creatorList,
		creatorCount,
		groupCount,
	)
	markup := tu.InlineKeyboard(
		tu.InlineKeyboardRow(ui.DeleteButton(i18n.Translate(lang, btnResetConfirm), ui.ActionResetDoBoth)),
		tu.InlineKeyboardRow(ui.BackButton(i18n.Translate(lang, btnBack), ui.ActionResetConfirmBack)),
	)
	c.reply(ctx, telegramUserID, editMsgID, text, &client.MessageOptions{ParseMode: telego.ModeHTML, Markup: markup})
	return ""
}
