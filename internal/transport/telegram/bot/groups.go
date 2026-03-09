package bot

import (
	"context"
	"errors"
	"fmt"
	"html"
	"strings"
	"time"

	"imsub/internal/core"
	"imsub/internal/platform/i18n"
	"imsub/internal/transport/telegram/client"
	"imsub/internal/usecase"

	"github.com/mymmrac/telego"
	tghandler "github.com/mymmrac/telego/telegohandler"
	"github.com/mymmrac/telego/telegoutil"
)

type botGroupCapabilities struct {
	isAdmin            bool
	canInviteUsers     bool
	canRestrictMembers bool
}

var errTelegramBotNotConfigured = errors.New("telegram bot not configured")

func (c botGroupCapabilities) evaluation() groupCapabilityEvaluation {
	if !c.isAdmin {
		return groupCapabilityEvaluation{botMissing: true}
	}
	return groupCapabilityEvaluation{
		canInviteUsers:   c.canInviteUsers,
		canRestrictUsers: c.canRestrictMembers,
	}
}

// onRegisterGroup handles /registergroup by binding the current Telegram group
// to the caller's creator account. The caller must be a group admin and have
// a linked creator record.
func (c *Bot) onRegisterGroup(ctx *tghandler.Context, msg telego.Message) error {
	if msg.From == nil {
		return nil
	}
	lang := i18n.NormalizeLanguage(msg.From.LanguageCode)
	threadID := groupMessageThreadID(msg)

	if msg.Chat.Type == telego.ChatTypePrivate {
		view := buildGroupReplyView(lang, msgGroupNotGroup, msg.MessageID)
		view.opts.MessageThreadID = threadID
		c.sendMsg(ctx, msg.Chat.ID, view.text, &view.opts)
		return nil
	}

	if waitErr := c.tgLimiter.Wait(ctx, msg.Chat.ID); waitErr != nil {
		c.log().Warn("Get chat member rate limit wait failed", "error", waitErr)
		view := buildGroupReplyView(lang, msgGroupNotAdmin, msg.MessageID)
		view.opts.MessageThreadID = threadID
		c.sendMsg(ctx, msg.Chat.ID, view.text, &view.opts)
		return nil
	}
	member, err := c.tg.GetChatMember(ctx, &telego.GetChatMemberParams{
		ChatID: telegoutil.ID(msg.Chat.ID),
		UserID: msg.From.ID,
	})
	isAdmin := err == nil && IsAdmin(member)

	_, ok, err := c.creatorStatus.LoadOwnedCreator(ctx, msg.From.ID)
	if err != nil {
		c.log().Warn("OnRegisterGroup getOwnedCreator failed", "error", err)
		return nil
	}

	if !isAdmin && !ok {
		return nil
	}
	if !isAdmin {
		view := buildGroupReplyView(lang, msgGroupNotAdmin, msg.MessageID)
		view.opts.MessageThreadID = threadID
		c.sendMsg(ctx, msg.Chat.ID, view.text, &view.opts)
		return nil
	}
	if !ok || c.groupRegistration == nil {
		view := buildGroupReplyView(lang, msgGroupNotCreator, msg.MessageID)
		view.opts.MessageThreadID = threadID
		c.sendMsg(ctx, msg.Chat.ID, view.text, &view.opts)
		return nil
	}
	if eval := c.evaluateBotGroupCapabilities(ctx, msg.Chat.ID); len(eval.issues(lang)) > 0 {
		view := buildGroupSettingWarningsView(lang, msg.MessageID, eval.issues(lang))
		view.opts.MessageThreadID = threadID
		c.sendMsg(ctx, msg.Chat.ID, view.text, &view.opts)
		return nil
	}

	regRes, err := c.groupRegistration.RegisterGroup(ctx, msg.From.ID, msg.Chat.ID, msg.Chat.Title)
	if err != nil {
		c.log().Warn("RegisterGroup failed", "chat_id", msg.Chat.ID, "owner_telegram_id", msg.From.ID, "error", err)
		return nil
	}
	view, ok := buildGroupRegistrationView(lang, msg.MessageID, regRes)
	if !ok {
		c.log().Warn("unsupported group registration outcome", "chat_id", msg.Chat.ID, "outcome", regRes.Outcome)
		return nil
	}
	view.opts.MessageThreadID = threadID

	groupMsgID := c.sendMsg(ctx, msg.Chat.ID, view.text, &view.opts)
	if view.dispatchFollowUp {
		c.dispatchGroupRegistrationFollowUp(ctx, msg, lang, regRes, view, groupMsgID, threadID)
	}
	return nil
}

// onUnregisterCommand handles /unregistergroup by unbinding the current Telegram group.
func (c *Bot) onUnregisterCommand(ctx *tghandler.Context, msg telego.Message) error {
	if msg.From == nil {
		return nil
	}
	lang := i18n.NormalizeLanguage(msg.From.LanguageCode)
	threadID := groupMessageThreadID(msg)
	view := buildGroupReplyView(lang, msgGroupUnregisterNotOwner, msg.MessageID)
	view.opts.MessageThreadID = threadID

	if msg.Chat.Type == telego.ChatTypePrivate {
		notGroup := buildGroupReplyView(lang, msgGroupNotGroup, msg.MessageID)
		notGroup.opts.MessageThreadID = threadID
		c.sendMsg(ctx, msg.Chat.ID, notGroup.text, &notGroup.opts)
		return nil
	}

	if c.groupUnregistration == nil {
		c.log().Warn("group unregistration use case unavailable")
		return nil
	}

	res, err := c.groupUnregistration.UnregisterGroup(ctx, msg.From.ID, msg.Chat.ID)
	if err != nil {
		c.log().Warn("UnregisterGroup failed", "chat_id", msg.Chat.ID, "owner_telegram_id", msg.From.ID, "error", err)
		return nil
	}
	switch res.Outcome {
	case usecase.UnregisterGroupOutcomeNotManaged:
		return nil
	case usecase.UnregisterGroupOutcomeNotOwner:
		c.sendMsg(ctx, msg.Chat.ID, view.text, &view.opts)
		return nil
	case usecase.UnregisterGroupOutcomeUnregistered, usecase.UnregisterGroupOutcomeUnregisteredCleanupLag:
	}
	if res.CleanupFailed {
		c.log().Warn("group unregistered but eventsub cleanup deferred to reconciliation", "creator_id", res.Creator.ID, "chat_id", msg.Chat.ID)
	}

	success := buildTextView(lang, msgGroupUnregistered)
	success.opts.ReplyToMessageID = msg.MessageID
	success.opts.MessageThreadID = threadID
	c.sendMsg(ctx, msg.Chat.ID, success.text, &success.opts)
	return nil
}

func (c *Bot) activateCreatorOnFirstGroupRegistration(parent context.Context, creator core.Creator, groupChatID int64, messageThreadID int, lang string) {
	if parent == nil {
		c.log().Warn("Activate creator called with nil context", "creator_id", creator.ID)
		return
	}
	baseCtx := context.WithoutCancel(parent)
	ctx, cancel := context.WithTimeout(baseCtx, 3*time.Minute)
	defer cancel()
	res, err := c.creatorActivation.Activate(ctx, creator)
	if err != nil {
		c.log().Warn("creator activation failed after first group registration", "creator_id", creator.ID, "error", err)
		view := buildTextView(lang, msgCreatorEventSubFail)
		view.opts.MessageThreadID = messageThreadID
		c.sendMsg(baseCtx, groupChatID, view.text, &view.opts)
		return
	}
	c.log().Info("creator activated on first group registration", "creator_id", creator.ID, "group_chat_id", groupChatID, "subscriber_count", res.SubscriberCount)
}

func (c *Bot) onMyChatMemberUpdated(ctx *tghandler.Context, update telego.ChatMemberUpdated) error {
	if update.Chat.Type == telego.ChatTypePrivate {
		return nil
	}

	switch update.NewChatMember.MemberStatus() {
	case telego.MemberStatusMember, telego.MemberStatusAdministrator, telego.MemberStatusCreator:
		lang := i18n.NormalizeLanguage(update.From.LanguageCode)
		view := buildGroupBotStatusChangedView(lang)
		groupMsgID := c.sendMsg(ctx, update.Chat.ID, view.text, &view.opts)
		if groupMsgID != 0 {
			c.runBackground(context.WithoutCancel(ctx), func(bg context.Context) {
				c.sendPostRegistrationSettingsCheck(bg, update.Chat.ID, groupMsgID, 0, lang, view.text)
			})
		}
	case telego.MemberStatusLeft, telego.MemberStatusBanned:
		if c.groupMatchesActiveCreator(ctx, update.Chat.ID) {
			c.log().Info("bot removed from registered group; auto-unregister should be the next step", "chat_id", update.Chat.ID, "new_status", update.NewChatMember.MemberStatus())
		}
	}

	return nil
}

func (c *Bot) onChatMemberUpdated(ctx *tghandler.Context, update telego.ChatMemberUpdated) error {
	group, ok, err := c.store.ManagedGroupByChatID(ctx, update.Chat.ID)
	if err != nil {
		c.log().Warn("ManagedGroupByChatID for chat_member failed", "chat_id", update.Chat.ID, "error", err)
		return nil
	}
	if !ok {
		return nil
	}

	memberUser := update.NewChatMember.MemberUser()
	if memberUser.IsBot || IsAdmin(update.NewChatMember) {
		return nil
	}

	status := update.NewChatMember.MemberStatus()
	switch status {
	case telego.MemberStatusMember, telego.MemberStatusRestricted:
		c.observeGroupMember(ctx, group.ChatID, memberUser.ID, "chat_member", status)
	case telego.MemberStatusLeft, telego.MemberStatusBanned:
		c.removeObservedGroupMember(ctx, group.ChatID, memberUser.ID)
	}
	return nil
}

func (c *Bot) onGroupMessage(ctx *tghandler.Context, msg telego.Message) error {
	if msg.From == nil || msg.From.IsBot || strings.HasPrefix(msg.Text, "/") {
		return nil
	}
	group, ok, err := c.store.ManagedGroupByChatID(ctx, msg.Chat.ID)
	if err != nil {
		c.log().Warn("ManagedGroupByChatID for group message failed", "chat_id", msg.Chat.ID, "error", err)
		return nil
	}
	if !ok {
		return nil
	}
	c.observeGroupMember(ctx, group.ChatID, msg.From.ID, "message", telego.MemberStatusMember)
	return nil
}

func (c *Bot) sendPostRegistrationSettingsCheck(ctx context.Context, groupChatID int64, groupMsgID int, messageThreadID int, lang, groupBaseText string) {
	warnings := c.evaluateGroupSettings(ctx, groupChatID).issues(lang)
	if groupMsgID != 0 {
		view := buildGroupSettingsCheckResultView(lang, groupBaseText, warnings)
		view.opts.MessageThreadID = messageThreadID
		c.reply(ctx, groupChatID, groupMsgID, view.text, &view.opts)
	}
}

func (c *Bot) sendPostRegistrationMessages(ctx context.Context, opts postRegistrationMessageOptions) {
	const draftID = 1

	rendered := renderPostRegistrationCopy(postRegistrationCopyInput{
		lang:          opts.lang,
		groupName:     opts.groupName,
		creatorName:   opts.creatorName,
		groupBaseText: opts.groupBaseText,
	}, nil)

	c.sendDraft(ctx, opts.ownerUserID, draftID, rendered.draftDM, &client.MessageOptions{ParseMode: telego.ModeHTML})

	warnings := c.evaluateGroupSettings(ctx, opts.groupChatID).issues(opts.lang)
	rendered = renderPostRegistrationCopy(postRegistrationCopyInput{
		lang:          opts.lang,
		groupName:     opts.groupName,
		creatorName:   opts.creatorName,
		groupBaseText: opts.groupBaseText,
	}, warnings)
	c.sendDraft(ctx, opts.ownerUserID, draftID, rendered.finalDM, &client.MessageOptions{ParseMode: telego.ModeHTML})
	c.sendMsg(ctx, opts.ownerUserID, rendered.finalDM, &client.MessageOptions{ParseMode: telego.ModeHTML})

	if opts.groupMsgID != 0 {
		c.reply(ctx, opts.groupChatID, opts.groupMsgID, rendered.groupMessage, &client.MessageOptions{
			ParseMode:       telego.ModeHTML,
			MessageThreadID: opts.messageThreadID,
		})
	}
}

type postRegistrationMessageOptions struct {
	groupChatID     int64
	groupMsgID      int
	messageThreadID int
	ownerUserID     int64
	groupName       string
	creatorName     string
	lang            string
	groupBaseText   string
}

type groupRegistrationView struct {
	text             string
	opts             client.MessageOptions
	groupBaseText    string
	dispatchFollowUp bool
}

func buildGroupRegistrationView(lang string, replyToMessageID int, regRes usecase.RegisterGroupResult) (groupRegistrationView, bool) {
	view := groupRegistrationView{opts: client.MessageOptions{ReplyToMessageID: replyToMessageID}}

	switch regRes.Outcome {
	case usecase.RegisterGroupOutcomeNotCreator:
		view.text = i18n.Translate(lang, msgGroupNotCreator)
	case usecase.RegisterGroupOutcomeTakenByOther:
		view.text = fmt.Sprintf(i18n.Translate(lang, msgGroupTakenByOther), html.EscapeString(regRes.OtherCreatorName))
		view.opts.ParseMode = telego.ModeHTML
	case usecase.RegisterGroupOutcomeAlreadyLinked:
		view.groupBaseText = fmt.Sprintf(i18n.Translate(lang, msgGroupAlreadyLinked), html.EscapeString(regRes.Creator.TwitchLogin))
		view.text = joinNonEmptySections(
			textSection{text: view.groupBaseText},
			textSection{text: i18n.Translate(lang, msgGroupCheckingSettings)},
		)
		view.opts.ParseMode = telego.ModeHTML
		view.dispatchFollowUp = true
	case usecase.RegisterGroupOutcomeRegistered:
		view.groupBaseText = fmt.Sprintf(i18n.Translate(lang, msgGroupRegistered), html.EscapeString(regRes.Creator.TwitchLogin))
		view.text = joinNonEmptySections(
			textSection{text: view.groupBaseText},
			textSection{text: i18n.Translate(lang, msgGroupCheckingSettings)},
		)
		view.opts.ParseMode = telego.ModeHTML
		view.dispatchFollowUp = true
	default:
		return groupRegistrationView{}, false
	}

	return view, true
}

func (c *Bot) dispatchGroupRegistrationFollowUp(ctx context.Context, msg telego.Message, lang string, regRes usecase.RegisterGroupResult, view groupRegistrationView, groupMsgID int, messageThreadID int) {
	if !regRes.FollowUp.NeedsActivation && !regRes.FollowUp.NeedsSettingsCheck {
		return
	}
	if regRes.FollowUp.NeedsActivation {
		c.runBackground(context.WithoutCancel(ctx), func(bg context.Context) {
			c.activateCreatorOnFirstGroupRegistration(bg, regRes.Creator, msg.Chat.ID, messageThreadID, lang)
		})
	}
	if !regRes.FollowUp.NeedsSettingsCheck {
		return
	}
	if regRes.FollowUp.NotifyOwner {
		c.runBackground(context.WithoutCancel(ctx), func(bg context.Context) {
			c.sendPostRegistrationMessages(bg, postRegistrationMessageOptions{
				groupChatID:     msg.Chat.ID,
				groupMsgID:      groupMsgID,
				messageThreadID: messageThreadID,
				ownerUserID:     msg.From.ID,
				groupName:       msg.Chat.Title,
				creatorName:     regRes.Creator.TwitchLogin,
				lang:            lang,
				groupBaseText:   view.groupBaseText,
			})
		})
		return
	}
	c.runBackground(context.WithoutCancel(ctx), func(bg context.Context) {
		c.sendPostRegistrationSettingsCheck(bg, msg.Chat.ID, groupMsgID, messageThreadID, lang, view.groupBaseText)
	})
}

type postRegistrationCopyInput struct {
	lang          string
	groupName     string
	creatorName   string
	groupBaseText string
}

type postRegistrationRendered struct {
	draftDM      string
	finalDM      string
	groupMessage string
}

func formatGroupSettingWarnings(lang string, issues []string) string {
	return renderWarningBlock(i18n.Translate(lang, msgGroupWarnSettingsIntro), issues)
}

func formatGroupSettingsResult(lang string, issues []string) string {
	if len(issues) > 0 {
		return formatGroupSettingWarnings(lang, issues)
	}
	return i18n.Translate(lang, msgGroupSettingsOK)
}

func renderPostRegistrationCopy(in postRegistrationCopyInput, issues []string) postRegistrationRendered {
	settingsResult := formatGroupSettingsResult(in.lang, issues)
	dmBase := fmt.Sprintf(
		i18n.Translate(in.lang, msgGroupRegisteredDM),
		html.EscapeString(in.groupName),
		html.EscapeString(in.creatorName),
	)

	return postRegistrationRendered{
		draftDM: joinNonEmptySections(
			textSection{text: dmBase},
			textSection{text: i18n.Translate(in.lang, msgGroupCheckingSettings)},
		),
		finalDM: joinNonEmptySections(
			textSection{text: dmBase},
			textSection{text: settingsResult},
		),
		groupMessage: joinNonEmptySections(
			textSection{text: in.groupBaseText},
			textSection{text: settingsResult},
		),
	}
}

type groupSettingsEvaluation struct {
	botCapabilities groupCapabilityEvaluation
	isPublic        bool
	joinByRequest   bool
	untrackedCount  int
}

type groupCapabilityEvaluation struct {
	botMissing       bool
	canInviteUsers   bool
	canRestrictUsers bool
}

func (e groupCapabilityEvaluation) issues(lang string) []string {
	if e.botMissing {
		return []string{i18n.Translate(lang, msgGroupWarnBotNotAdmin)}
	}

	var issues []string
	if !e.canInviteUsers {
		issues = append(issues, i18n.Translate(lang, msgGroupWarnBotNoInvite))
	}
	if !e.canRestrictUsers {
		issues = append(issues, i18n.Translate(lang, msgGroupWarnBotNoRestrict))
	}
	return issues
}

func (e groupSettingsEvaluation) issues(lang string) []string {
	issues := e.botCapabilities.issues(lang)
	if e.isPublic {
		issues = append(issues, i18n.Translate(lang, msgGroupWarnPublic))
	}
	if !e.joinByRequest {
		issues = append(issues, i18n.Translate(lang, msgGroupWarnJoinByReq))
	}
	if e.untrackedCount > 0 {
		issues = append(issues, fmt.Sprintf(i18n.Translate(lang, msgGroupWarnUntrackedUsers), e.untrackedCount))
	}
	return issues
}

func (c *Bot) evaluateGroupSettings(ctx context.Context, chatID int64) groupSettingsEvaluation {
	if waitErr := c.tgLimiter.Wait(ctx, chatID); waitErr != nil {
		c.log().Warn("GetChat rate limit wait failed", "error", waitErr)
		return groupSettingsEvaluation{}
	}
	chat, err := c.tg.GetChat(ctx, &telego.GetChatParams{ChatID: telegoutil.ID(chatID)})
	if err != nil {
		c.log().Warn("GetChat for group settings check failed", "chat_id", chatID, "error", err)
		return groupSettingsEvaluation{}
	}

	return groupSettingsEvaluation{
		botCapabilities: c.evaluateBotGroupCapabilities(ctx, chatID),
		isPublic:        chat.Username != "" || len(chat.ActiveUsernames) > 0,
		joinByRequest:   chat.JoinByRequest,
		untrackedCount:  c.countUntrackedMembers(ctx, chatID),
	}
}

func (c *Bot) evaluateBotGroupCapabilities(ctx context.Context, chatID int64) groupCapabilityEvaluation {
	caps, err := c.loadBotGroupCapabilities(ctx, chatID)
	if err != nil {
		c.log().Warn("load bot group capabilities failed", "chat_id", chatID, "error", err)
		return groupCapabilityEvaluation{botMissing: true}
	}
	return caps.evaluation()
}

func (c *Bot) loadBotGroupCapabilities(ctx context.Context, chatID int64) (botGroupCapabilities, error) {
	if c.tg == nil {
		return botGroupCapabilities{}, errTelegramBotNotConfigured
	}
	me, err := c.tg.GetMe(ctx)
	if err != nil {
		return botGroupCapabilities{}, fmt.Errorf("get bot profile: %w", err)
	}
	if c.tgLimiter != nil {
		if waitErr := c.tgLimiter.Wait(ctx, chatID); waitErr != nil {
			return botGroupCapabilities{}, fmt.Errorf("wait get chat member: %w", waitErr)
		}
	}
	member, err := c.tg.GetChatMember(ctx, &telego.GetChatMemberParams{
		ChatID: telegoutil.ID(chatID),
		UserID: me.ID,
	})
	if err != nil {
		return botGroupCapabilities{}, fmt.Errorf("get bot chat member: %w", err)
	}

	switch m := member.(type) {
	case *telego.ChatMemberOwner:
		return botGroupCapabilities{isAdmin: true, canInviteUsers: true, canRestrictMembers: true}, nil
	case *telego.ChatMemberAdministrator:
		return botGroupCapabilities{
			isAdmin:            true,
			canInviteUsers:     m.CanInviteUsers,
			canRestrictMembers: m.CanRestrictMembers,
		}, nil
	default:
		return botGroupCapabilities{}, nil
	}
}

func (c *Bot) groupMatchesActiveCreator(ctx context.Context, chatID int64) bool {
	_, ok, err := c.store.ManagedGroupByChatID(ctx, chatID)
	if err != nil {
		c.log().Warn("ManagedGroupByChatID for my_chat_member check failed", "chat_id", chatID, "error", err)
		return false
	}
	return ok
}

func (c *Bot) countUntrackedMembers(ctx context.Context, chatID int64) int {
	count, err := c.store.CountUntrackedGroupMembers(ctx, chatID)
	if err == nil {
		return count
	}
	c.log().Warn("CountUntrackedGroupMembers failed, falling back to estimate", "chat_id", chatID, "error", err)

	if waitErr := c.tgLimiter.Wait(ctx, chatID); waitErr != nil {
		c.log().Warn("GetChatMemberCount rate limit wait failed", "error", waitErr)
		return 0
	}
	total, err := c.tg.GetChatMemberCount(ctx, &telego.GetChatMemberCountParams{ChatID: telegoutil.ID(chatID)})
	if err != nil || total == nil {
		c.log().Warn("GetChatMemberCount failed", "chat_id", chatID, "error", err)
		return 0
	}
	if waitErr := c.tgLimiter.Wait(ctx, chatID); waitErr != nil {
		c.log().Warn("GetChatAdministrators rate limit wait failed", "error", waitErr)
		return 0
	}
	admins, err := c.tg.GetChatAdministrators(ctx, &telego.GetChatAdministratorsParams{ChatID: telegoutil.ID(chatID)})
	if err != nil {
		c.log().Warn("GetChatAdministrators failed", "chat_id", chatID, "error", err)
		return 0
	}
	untracked := *total - len(admins)
	if untracked < 0 {
		return 0
	}
	return untracked
}

func (c *Bot) observeGroupMember(ctx context.Context, chatID, telegramUserID int64, source, status string) {
	tracked, err := c.store.IsTrackedGroupMember(ctx, chatID, telegramUserID)
	if err != nil {
		c.log().Warn("IsTrackedGroupMember failed", "chat_id", chatID, "telegram_user_id", telegramUserID, "error", err)
		return
	}
	now := time.Now().UTC()
	if tracked {
		if err := c.store.AddTrackedGroupMember(ctx, chatID, telegramUserID, source, now); err != nil {
			c.log().Warn("AddTrackedGroupMember refresh failed", "chat_id", chatID, "telegram_user_id", telegramUserID, "error", err)
		}
		if err := c.store.RemoveUntrackedGroupMember(ctx, chatID, telegramUserID); err != nil {
			c.log().Warn("RemoveUntrackedGroupMember refresh failed", "chat_id", chatID, "telegram_user_id", telegramUserID, "error", err)
		}
		return
	}
	if err := c.store.UpsertUntrackedGroupMember(ctx, chatID, telegramUserID, source, status, now); err != nil {
		c.log().Warn("UpsertUntrackedGroupMember failed", "chat_id", chatID, "telegram_user_id", telegramUserID, "source", source, "error", err)
	}
}

func (c *Bot) removeObservedGroupMember(ctx context.Context, chatID, telegramUserID int64) {
	if err := c.store.RemoveTrackedGroupMember(ctx, chatID, telegramUserID); err != nil {
		c.log().Warn("RemoveTrackedGroupMember failed", "chat_id", chatID, "telegram_user_id", telegramUserID, "error", err)
	}
	if err := c.store.RemoveUntrackedGroupMember(ctx, chatID, telegramUserID); err != nil {
		c.log().Warn("RemoveUntrackedGroupMember failed", "chat_id", chatID, "telegram_user_id", telegramUserID, "error", err)
	}
}

// IsAdmin reports whether member has Administrator or Creator status.
func IsAdmin(member telego.ChatMember) bool {
	if member == nil {
		return false
	}
	switch member.MemberStatus() {
	case telego.MemberStatusAdministrator, telego.MemberStatusCreator:
		return true
	}
	return false
}

func buildGroupReplyView(lang, key string, replyToMessageID int) sharedView {
	view := buildTextView(lang, key)
	view.opts = client.MessageOptions{ReplyToMessageID: replyToMessageID}
	return view
}

func buildGroupSettingWarningsView(lang string, replyToMessageID int, issues []string) sharedView {
	return sharedView{
		text: formatGroupSettingWarnings(lang, issues),
		opts: client.MessageOptions{
			ReplyToMessageID: replyToMessageID,
			ParseMode:        telego.ModeHTML,
		},
	}
}

func buildGroupSettingsCheckResultView(lang, groupBaseText string, issues []string) sharedView {
	return sharedView{
		text: groupBaseText + "\n\n" + formatGroupSettingsResult(lang, issues),
		opts: client.MessageOptions{ParseMode: telego.ModeHTML},
	}
}

func buildGroupBotStatusChangedView(lang string) sharedView {
	return buildHTMLTextView(lang, msgGroupBotStatusChanged)
}

func groupMessageThreadID(msg telego.Message) int {
	if msg.IsTopicMessage && msg.MessageThreadID > 0 {
		return msg.MessageThreadID
	}
	return 0
}
