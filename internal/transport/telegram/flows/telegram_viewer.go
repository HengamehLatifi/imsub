package flows

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"imsub/internal/core"
	"imsub/internal/platform/i18n"
	"imsub/internal/transport/telegram/client"
	"imsub/internal/transport/telegram/ui"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// --- Viewer flow ---

func (c *Controller) handleViewerStart(ctx context.Context, telegramUserID int64, editMsgID int, lang string) string {
	return c.handleViewerStartForUser(ctx, telegramUserID, editMsgID, lang, "")
}

func (c *Controller) handleViewerStartForUser(ctx context.Context, telegramUserID int64, editMsgID int, lang, userName string) string {
	identity, hasIdentity, err := c.viewerSvc.LoadIdentity(ctx, telegramUserID)
	if err != nil {
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgErrLoadStatus), &client.MessageOptions{Markup: ui.MainMenuMarkup(lang)})
		return i18n.Translate(lang, msgErrLoadStatus)
	}

	if !hasIdentity {
		payload := core.OAuthStatePayload{
			Mode:            core.OAuthModeViewer,
			TelegramUserID:  telegramUserID,
			Language:        lang,
			PromptMessageID: editMsgID,
		}
		state, err := c.createOAuthState(ctx, payload, 10*time.Minute)
		if err != nil {
			c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgErrLoadStatus), &client.MessageOptions{Markup: ui.MainMenuMarkup(lang)})
			return i18n.Translate(lang, msgErrLoadStatus)
		}
		authURL := c.oauthStartURL(state)
		markup := tu.InlineKeyboard(
			tu.InlineKeyboardRow(ui.LinkButton(i18n.Translate(lang, btnLinkTwitch), authURL)),
			tu.InlineKeyboardRow(ui.CopyLinkButton(i18n.Translate(lang, btnCopyLink), authURL)),
		)
		displayName := strings.TrimSpace(userName)
		if displayName == "" {
			displayName = i18n.Translate(lang, msgUserGenericName)
		}
		promptText := fmt.Sprintf(i18n.Translate(lang, msgLinkPromptHTML), html.EscapeString(displayName))
		if editMsgID != 0 {
			c.reply(ctx, telegramUserID, editMsgID, promptText, &client.MessageOptions{ParseMode: telego.ModeHTML, Markup: markup})
			return ""
		}
		messageID := c.sendMsg(ctx, telegramUserID, promptText, &client.MessageOptions{ParseMode: telego.ModeHTML, Markup: markup})
		if messageID != 0 {
			payload.PromptMessageID = messageID
			if err := c.store.SaveOAuthState(ctx, state, payload, 10*time.Minute); err != nil {
				c.log().Warn("saveOAuthState prompt message update failed", "error", err)
			}
		}
		return ""
	}

	joinRows, activeNames, err := c.buildJoinButtons(ctx, telegramUserID, identity.TwitchUserID, lang)
	if err != nil {
		c.log().Warn("buildJoinButtons failed", "telegram_user_id", telegramUserID, "error", err)
		c.reply(ctx, telegramUserID, editMsgID, i18n.Translate(lang, msgErrLoadStatus), &client.MessageOptions{Markup: ui.MainMenuMarkup(lang)})
		return i18n.Translate(lang, msgErrLoadStatus)
	}
	c.replyLinkedStatus(ctx, telegramUserID, editMsgID, lang, identity.TwitchLogin, joinRows, activeNames)
	return ""
}

// buildJoinButtons returns join buttons and active subscription names for the
// given viewer. The underlying viewer service computes active subscriptions and
// required join links for creators with registered groups (GroupChatID != 0):
//
//   - Non-subscriber links are removed from the persisted viewer linkage state.
//   - Subscribers already in the group are skipped.
//   - Subscribers not yet in the group get a fresh invite link button.
//
// O(N) external calls where N is the number of active creators, plus
// O(S log S) for sorting subscription names (S ≤ N).
func (c *Controller) buildJoinButtons(ctx context.Context, telegramUserID int64, twitchUserID, lang string) ([][]telego.InlineKeyboardButton, []string, error) {
	targets, err := c.viewerSvc.BuildJoinTargets(ctx, telegramUserID, twitchUserID)
	if err != nil {
		return nil, nil, fmt.Errorf("build join targets: %w", err)
	}

	rows := make([][]telego.InlineKeyboardButton, 0, len(targets.JoinLinks))
	for _, link := range targets.JoinLinks {
		btnText := link.CreatorName + " - " + link.GroupName
		rows = append(rows, tu.InlineKeyboardRow(ui.LinkButton(fmt.Sprintf(i18n.Translate(lang, btnJoin), btnText), link.InviteLink)))
	}
	return rows, targets.ActiveCreatorNames, nil
}
