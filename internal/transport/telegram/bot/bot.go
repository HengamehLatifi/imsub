package bot

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"imsub/internal/core"
	"imsub/internal/platform/i18n"
	"imsub/internal/transport/telegram/client"
	telegramui "imsub/internal/transport/telegram/ui"

	"github.com/mymmrac/telego"
	tghandler "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"
)

const msgCbRefreshed = "cb_refreshed"

func (c *Bot) oauthStartURL(state string) string {
	return c.cfg.PublicBaseURL + "/auth/start/" + url.PathEscape(state)
}

// sendMsg sends a Telegram message and returns its message ID, or 0 on failure.
func (c *Bot) sendMsg(ctx context.Context, chatID int64, text string, opts *client.MessageOptions) int {
	if c == nil || c.telegramClient == nil {
		return 0
	}
	return c.telegramClient.Send(ctx, chatID, text, opts)
}

func (c *Bot) reply(ctx context.Context, chatID int64, messageID int, text string, opts *client.MessageOptions) {
	if c == nil || c.telegramClient == nil {
		return
	}
	c.telegramClient.Reply(ctx, chatID, messageID, text, opts)
}

func (c *Bot) sendDraft(ctx context.Context, chatID int64, draftID int, text string, opts *client.MessageOptions) {
	if c == nil || c.telegramClient == nil {
		return
	}
	c.telegramClient.SendDraft(ctx, chatID, draftID, text, opts)
}

func (c *Bot) deleteMessage(ctx context.Context, chatID int64, messageID int) {
	if c == nil || c.telegramClient == nil {
		return
	}
	c.telegramClient.Delete(ctx, chatID, messageID)
}

func (c *Bot) createInviteLink(ctx context.Context, groupChatID int64, telegramUserID int64, name string) (string, error) {
	if c == nil || c.telegramGroups == nil {
		return "", errTelegramBotNotConfigured
	}
	link, err := c.telegramGroups.CreateInviteLink(ctx, groupChatID, telegramUserID, name)
	if err != nil {
		return "", fmt.Errorf("create invite link from group ops: %w", err)
	}
	return link, nil
}

func (c *Bot) kickDisplacedUser(ctx context.Context, telegramUserID int64) {
	if c == nil || c.telegramGroups == nil {
		return
	}
	c.telegramGroups.KickDisplacedUser(ctx, telegramUserID)
}

func (c *Bot) isGroupMember(ctx context.Context, groupChatID, telegramUserID int64) bool {
	if c == nil || c.telegramGroups == nil {
		return false
	}
	return c.telegramGroups.IsGroupMember(ctx, groupChatID, telegramUserID)
}

// KickFromGroup removes a Telegram user from a managed group.
func (c *Bot) KickFromGroup(ctx context.Context, groupChatID int64, telegramUserID int64) error {
	if c == nil || c.telegramGroups == nil {
		return nil
	}
	if err := c.telegramGroups.KickFromGroup(ctx, groupChatID, telegramUserID); err != nil {
		return fmt.Errorf("kick from group via group ops: %w", err)
	}
	return nil
}

func renderJoinButtons(targets core.JoinTargets, lang string) [][]telego.InlineKeyboardButton {
	rows := make([][]telego.InlineKeyboardButton, 0, len(targets.JoinLinks))
	for _, link := range targets.JoinLinks {
		btnText := link.CreatorName + " - " + link.GroupName
		rows = append(rows, tu.InlineKeyboardRow(telegramui.LinkButton(fmt.Sprintf(i18n.Translate(lang, btnJoin), btnText), link.InviteLink)))
	}
	return rows
}

func (c *Bot) answerCallback(ctx context.Context, callbackID, text string) {
	c.answerCallbackOpts(ctx, callbackID, text, false)
}

func (c *Bot) answerCallbackAlert(ctx context.Context, callbackID, text string) {
	c.answerCallbackOpts(ctx, callbackID, text, true)
}

func (c *Bot) answerCallbackOpts(ctx context.Context, callbackID, text string, showAlert bool) {
	if c == nil || c.telegramClient == nil {
		return
	}
	c.telegramClient.AnswerCallback(ctx, callbackID, text, showAlert)
}

func viewerMainMenuCallbacks() telegramui.MainMenuCallbacks {
	return telegramui.MainMenuCallbacks{
		Refresh: viewerRefreshCallback(),
		Reset:   resetOpenCallback(resetOriginViewer),
	}
}

func viewerMainMenuMarkup(lang string) *telego.InlineKeyboardMarkup {
	return telegramui.MainMenuMarkup(lang, viewerMainMenuCallbacks())
}

func creatorStatusMenuCallbacks(hasManageGroups, isActive bool) telegramui.CreatorMenuCallbacks {
	callbacks := telegramui.CreatorMenuCallbacks{
		Refresh: creatorRefreshCallback(),
		Reset:   resetOpenCallback(resetOriginCreator),
	}
	if hasManageGroups {
		callbacks.ManageGroups = creatorManageGroupsCallback()
	}
	if isActive {
		callbacks.Blocklist = creatorBlocklistToggleCallback()
	}
	return callbacks
}

func creatorMainMenuCallbacks() telegramui.CreatorMenuCallbacks {
	return telegramui.CreatorMenuCallbacks{
		Refresh: creatorRefreshCallback(),
		Reset:   resetOpenCallback(resetOriginCreator),
	}
}

func creatorMainMenuMarkup(lang string) *telego.InlineKeyboardMarkup {
	return telegramui.CreatorMainMenuMarkup(lang, creatorMainMenuCallbacks())
}

func (c *Bot) createOAuthState(ctx context.Context, payload core.OAuthStatePayload, ttl time.Duration) (string, error) {
	state, err := NewSecureToken(24)
	if err != nil {
		return "", fmt.Errorf("generate secure token: %w", err)
	}
	if err := c.store.SaveOAuthState(ctx, state, payload, ttl); err != nil {
		return "", fmt.Errorf("save oauth state: %w", err)
	}
	return state, nil
}

func (c *Bot) invalidateOAuthState(ctx context.Context, state string) {
	if state == "" {
		return
	}
	cleanupCtx := context.WithoutCancel(ctx)
	if _, err := c.store.DeleteOAuthState(cleanupCtx, state); err != nil {
		c.log().Warn("deleteOAuthState cleanup failed", "state", state, "error", err)
	}
}

// RegisterTelegramHandlers binds Telegram commands, callbacks, and join-request handlers.
func (c *Bot) RegisterTelegramHandlers() {
	if c.tgHandler == nil {
		return
	}

	privateOnly := func(_ context.Context, update telego.Update) bool {
		return update.Message != nil && update.Message.Chat.Type == telego.ChatTypePrivate && update.Message.From != nil
	}
	groupOnly := func(_ context.Context, update telego.Update) bool {
		return update.Message != nil && update.Message.Chat.Type != telego.ChatTypePrivate && update.Message.From != nil
	}

	c.tgHandler.HandleMessage(c.onRegisterGroup, tghandler.CommandEqual("registergroup"))
	c.tgHandler.HandleMessage(c.onUnregisterCommand, tghandler.And(tghandler.CommandEqual("unregistergroup"), groupOnly))
	c.tgHandler.HandleMessage(c.onStartCommand, tghandler.And(tghandler.CommandEqual("start"), privateOnly))
	c.tgHandler.HandleMessage(c.onCreatorCommand, tghandler.And(tghandler.CommandEqual("creator"), privateOnly))
	c.tgHandler.HandleMessage(c.onResetCommand, tghandler.And(tghandler.CommandEqual("reset"), privateOnly))
	c.tgHandler.HandleCallbackQuery(func(ctx *tghandler.Context, query telego.CallbackQuery) error {
		c.onCallbackQuery(ctx, query)
		return nil
	}, tghandler.AnyCallbackQuery())
	c.tgHandler.HandleChatJoinRequest(c.onChatJoinRequest)
	c.tgHandler.HandleChatMemberUpdated(c.onChatMemberUpdated)
	c.tgHandler.HandleMyChatMemberUpdated(c.onMyChatMemberUpdated)
	c.tgHandler.HandleMessage(c.onGroupMessage, tghandler.And(tghandler.AnyMessage(), groupOnly))
	c.tgHandler.HandleMessage(c.onUnknownMessage, tghandler.And(tghandler.AnyMessage(), privateOnly))
}

func (c *Bot) onCallbackQuery(ctx context.Context, q telego.CallbackQuery) {
	lang := i18n.NormalizeLanguage(q.From.LanguageCode)
	var msgID int
	if q.Message != nil {
		msgID = q.Message.GetMessageID()
	}

	action, ok := parseCallbackAction(q.Data)
	if !ok {
		c.log().Warn("ignore unknown callback data", "telegram_user_id", q.From.ID, "data", q.Data)
		c.answerCallback(ctx, q.ID, "")
		return
	}

	alertErr := c.dispatchCallbackAction(ctx, q.From.ID, msgID, lang, action)
	if alertErr != "" {
		c.answerCallbackAlert(ctx, q.ID, alertErr)
		return
	}

	callbackText := ""
	if action.verb == callbackVerbRefresh {
		callbackText = i18n.Translate(lang, msgCbRefreshed)
	}
	c.answerCallback(ctx, q.ID, callbackText)
}

func (c *Bot) dispatchCallbackAction(ctx context.Context, userID int64, editMsgID int, lang string, action callbackAction) string {
	switch action.domain {
	case callbackDomainViewer:
		return c.handleViewerStart(ctx, userID, editMsgID, lang)
	case callbackDomainCreator:
		return c.handleCreatorCallback(ctx, userID, editMsgID, lang, action)
	case callbackDomainReset:
		return c.handleResetAction(ctx, userID, editMsgID, lang, action)
	}
	c.log().Warn("unsupported callback action", "telegram_user_id", userID, "data", action.String())
	return ""
}

func (c *Bot) onUnknownMessage(ctx *tghandler.Context, message telego.Message) error {
	lang := i18n.NormalizeLanguage(message.From.LanguageCode)
	key := msgCmdHelp
	if message.From != nil {
		var err error
		key, err = c.helpMessageKey(ctx, message.From.ID)
		if err != nil {
			c.log().Warn("Resolve help message key failed", "telegram_user_id", message.From.ID, "error", err)
			key = msgCmdHelp
		}
	}
	view := buildMainMenuTextView(lang, key)
	c.sendMsg(ctx, message.Chat.ID, view.text, &view.opts)
	return nil
}

func (c *Bot) helpMessageKey(ctx context.Context, telegramUserID int64) (string, error) {
	_, hasViewer, err := c.viewerAccess.LoadIdentity(ctx, telegramUserID)
	if err != nil {
		return "", fmt.Errorf("load viewer identity for help message: %w", err)
	}
	_, hasCreator, err := c.creatorStatus.LoadOwnedCreator(ctx, telegramUserID)
	if err != nil {
		return "", fmt.Errorf("load owned creator for help message: %w", err)
	}
	switch {
	case hasViewer && hasCreator:
		return msgCmdHelpBoth, nil
	case hasCreator:
		return msgCmdHelpCreator, nil
	case hasViewer:
		return msgCmdHelpViewer, nil
	default:
		return msgCmdHelp, nil
	}
}

func (c *Bot) onChatJoinRequest(ctx *tghandler.Context, req telego.ChatJoinRequest) error {
	if req.InviteLink == nil || !strings.HasPrefix(req.InviteLink.Name, "imsub-") {
		return nil
	}

	parts := strings.SplitN(req.InviteLink.Name, "-", 3)
	if len(parts) < 3 {
		return nil
	}
	linkUserID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || linkUserID != req.From.ID {
		c.log().Info("join request denied", "link_user", parts[1], "requester_id", req.From.ID, "chat_id", req.Chat.ID)
		if waitErr := c.tgLimiter.Wait(ctx, req.Chat.ID); waitErr != nil {
			c.log().Warn("Decline join request rate limit wait failed", "error", waitErr)
			return nil
		}
		if err := c.tg.DeclineChatJoinRequest(ctx, &telego.DeclineChatJoinRequestParams{
			ChatID: tu.ID(req.Chat.ID),
			UserID: req.From.ID,
		}); err != nil {
			c.log().Warn("Decline join request failed", "user_id", req.From.ID, "chat_id", req.Chat.ID, "error", err)
		}
		return nil
	}

	if waitErr := c.tgLimiter.Wait(ctx, req.Chat.ID); waitErr != nil {
		c.log().Warn("Approve join request rate limit wait failed", "error", waitErr)
		return nil
	}
	if c.shouldDeclineJoinRequest(ctx, req.Chat.ID, req.From.ID) {
		if err := c.tg.DeclineChatJoinRequest(ctx, &telego.DeclineChatJoinRequestParams{
			ChatID: tu.ID(req.Chat.ID),
			UserID: req.From.ID,
		}); err != nil {
			c.log().Warn("Decline blocked join request failed", "user_id", req.From.ID, "chat_id", req.Chat.ID, "error", err)
		}
		return nil
	}
	err = c.tg.ApproveChatJoinRequest(ctx, &telego.ApproveChatJoinRequestParams{
		ChatID: tu.ID(req.Chat.ID),
		UserID: req.From.ID,
	})
	if err != nil {
		c.log().Warn("Approve join request failed", "user_id", req.From.ID, "chat_id", req.Chat.ID, "error", err)
	}
	return nil
}

func (c *Bot) shouldDeclineJoinRequest(ctx context.Context, chatID, telegramUserID int64) bool {
	if c == nil || c.store == nil {
		return false
	}
	group, ok, err := c.store.ManagedGroupByChatID(ctx, chatID)
	if err != nil {
		c.log().Warn("ManagedGroupByChatID for join request failed", "chat_id", chatID, "error", err)
		return false
	}
	if !ok {
		return false
	}
	creator, creatorFound, err := c.store.Creator(ctx, group.CreatorID)
	if err != nil {
		c.log().Warn("Creator for join request failed", "creator_id", group.CreatorID, "error", err)
		return false
	}
	if !creatorFound || !creator.BlocklistSyncEnabled {
		return false
	}
	identity, found, err := c.store.UserIdentity(ctx, telegramUserID)
	if err != nil {
		c.log().Warn("UserIdentity for join request failed", "telegram_user_id", telegramUserID, "error", err)
		return false
	}
	if !found {
		return false
	}
	blocked, err := c.store.IsCreatorBlocked(ctx, group.CreatorID, identity.TwitchUserID)
	if err != nil {
		c.log().Warn("IsCreatorBlocked for join request failed", "creator_id", group.CreatorID, "twitch_user_id", identity.TwitchUserID, "error", err)
		return false
	}
	return blocked
}

const (
	resultSaveFailed          = "save_failed"
	resultStoreFailed         = "store_failed"
	resultTokenExchangeFailed = "token_exchange_failed"
	resultUserInfoFailed      = "userinfo_failed"
	resultLoadStatusFailed    = "load_status_failed"
	resultScopeMissing        = "scope_missing"
	resultSuccess             = "success"
)

var errReconnectNotificationSend = errors.New("send reconnect-required notification")

// HandleSubscriptionEnd revokes Telegram group access after a Twitch subscription ends.
func (c *Bot) HandleSubscriptionEnd(ctx context.Context, broadcasterID, broadcasterLogin, twitchUserID, twitchLogin string) error {
	res, err := c.subscriptionEnd.Prepare(ctx, broadcasterID, broadcasterLogin, twitchUserID, twitchLogin)
	if err != nil {
		c.log().Warn("process subscription end failed", "error", err)
		return fmt.Errorf("prepare subscription end: %w", err)
	}
	if !res.Prepared.Found {
		return nil
	}

	for _, groupChatID := range res.Prepared.GroupChatIDs {
		if err := c.KickFromGroup(ctx, groupChatID, res.Prepared.TelegramUserID); err != nil {
			c.log().Warn("kickFromGroup failed", "telegram_user_id", res.Prepared.TelegramUserID, "group_chat_id", groupChatID, "error", err)
		}
	}

	view := buildSubscriptionEndView(res.Prepared.Language, res.Prepared.ViewerLogin, res.Prepared.BroadcasterLogin)
	c.sendMsg(ctx, res.Prepared.TelegramUserID, view.text, &view.opts)
	return nil
}
