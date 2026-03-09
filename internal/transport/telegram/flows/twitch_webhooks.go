package flows

import (
	"context"
	"errors"
	"fmt"

	"imsub/internal/core"
)

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

// --- Viewer OAuth callback ---

// HandleViewerOAuthCallback executes viewer OAuth callback side effects and notifications.
func (c *Controller) HandleViewerOAuthCallback(ctx context.Context, code string, payload core.OAuthStatePayload, lang string) (label string, twitchDisplayName string, err error) {
	res, flowErr := c.app.ViewerOAuth.Complete(ctx, code, payload, lang)
	if flowErr != nil {
		var fe *core.FlowError
		if errors.As(flowErr, &fe) {
			switch fe.Kind {
			case core.KindTokenExchange:
				view := buildViewerOAuthFailureView(lang, msgOAuthExchangeFail)
				c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
				return res.ResultLabel, "", fmt.Errorf("viewer token exchange failed: %w", flowErr)
			case core.KindUserInfo:
				view := buildViewerOAuthFailureView(lang, msgOAuthUserInfoFail)
				c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
				return res.ResultLabel, "", fmt.Errorf("viewer user info failed: %w", flowErr)
			case core.KindSave:
				view := buildViewerOAuthFailureView(lang, msgOAuthSaveFail)
				c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
				return res.ResultLabel, "", fmt.Errorf("viewer save failed: %w", flowErr)
			case core.KindScopeMissing, core.KindStore:
				view := buildViewerOAuthFailureView(lang, msgOAuthSaveFail)
				c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
				return res.ResultLabel, "", fmt.Errorf("viewer other fail: %w", flowErr)
			case core.KindCreatorMismatch:
				view := buildViewerOAuthFailureView(lang, msgOAuthSaveFail)
				c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
				return res.ResultLabel, "", fmt.Errorf("viewer creator mismatch fail: %w", flowErr)
			}
		}
		view := buildViewerOAuthFailureView(lang, msgOAuthSaveFail)
		c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
		return res.ResultLabel, "", fmt.Errorf("viewer unexpected fail: %w", flowErr)
	}
	if res.DisplacedUserID != 0 {
		c.kickDisplacedUser(ctx, res.DisplacedUserID)
	}
	if payload.PromptMessageID != 0 {
		c.deleteMessage(ctx, payload.TelegramUserID, payload.PromptMessageID)
	}

	access, buildErr := c.app.ViewerAccess.LoadAccess(ctx, payload.TelegramUserID)
	if buildErr != nil {
		c.log().Warn("load viewer access failed after viewer oauth callback", "telegram_user_id", payload.TelegramUserID, "error", buildErr)
		view := buildOAuthLoadStatusErrorView(lang)
		c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
		return resultLoadStatusFailed, res.TwitchDisplayName, fmt.Errorf("load viewer access: %w", buildErr)
	}
	view := buildViewerLinkedView(lang, res.TwitchLogin, access.Targets)
	c.reply(ctx, payload.TelegramUserID, 0, view.text, &view.opts)

	return res.ResultLabel, res.TwitchDisplayName, nil
}

// --- Creator OAuth callback ---

// HandleCreatorOAuthCallback executes creator OAuth callback side effects and notifications.
func (c *Controller) HandleCreatorOAuthCallback(ctx context.Context, code string, payload core.OAuthStatePayload, lang string) (label string, creatorName string, err error) {
	res, flowErr := c.app.CreatorOAuth.Complete(ctx, code, payload)
	if flowErr != nil {
		var fe *core.FlowError
		if errors.As(flowErr, &fe) {
			switch fe.Kind {
			case core.KindTokenExchange:
				view := buildCreatorOAuthFailureView(lang, msgCreatorExchangeFail)
				c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
				return res.ResultLabel, "", fmt.Errorf("creator token exchange: %w", flowErr)
			case core.KindScopeMissing:
				view := buildCreatorOAuthFailureView(lang, msgCreatorScopeMissing)
				c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
				return res.ResultLabel, "", fmt.Errorf("creator scope missing: %w", flowErr)
			case core.KindUserInfo:
				view := buildCreatorOAuthFailureView(lang, msgCreatorUserInfoFail)
				c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
				return res.ResultLabel, "", fmt.Errorf("creator user info: %w", flowErr)
			case core.KindStore:
				view := buildCreatorOAuthFailureView(lang, msgCreatorStoreFail)
				c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
				return res.ResultLabel, "", fmt.Errorf("creator store fail: %w", flowErr)
			case core.KindSave:
				view := buildCreatorOAuthFailureView(lang, msgCreatorStoreFail)
				c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
				return res.ResultLabel, "", fmt.Errorf("creator save fail: %w", flowErr)
			case core.KindCreatorMismatch:
				view := buildCreatorOAuthFailureView(lang, msgCreatorReconnectMismatch)
				c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
				return res.ResultLabel, "", fmt.Errorf("creator reconnect mismatch: %w", flowErr)
			}
		}
		view := buildCreatorOAuthFailureView(lang, msgCreatorStoreFail)
		c.sendMsg(ctx, payload.TelegramUserID, view.text, &view.opts)
		return res.ResultLabel, "", fmt.Errorf("creator unexpected fail: %w", flowErr)
	}
	creator := res.Creator
	c.log().Debug("creator oauth exchange success", "creator_id", creator.ID, "creator_login", creator.Name, "owner_telegram_id", creator.OwnerTelegramID)
	if payload.PromptMessageID != 0 {
		c.deleteMessage(ctx, payload.TelegramUserID, payload.PromptMessageID)
	}
	c.replyCreatorStatus(ctx, payload.TelegramUserID, 0, lang)
	return res.ResultLabel, res.BroadcasterDisplayName, nil
}

// NotifyCreatorReconnectRequired sends a one-shot stale-auth notification to a creator owner.
func (c *Controller) NotifyCreatorReconnectRequired(ctx context.Context, creator core.Creator) error {
	lang := "en"
	if identity, ok, err := c.store.UserIdentity(ctx, creator.OwnerTelegramID); err == nil && ok && identity.Language != "" {
		lang = identity.Language
	}
	reconnectURL, err := c.creatorReconnectURL(ctx, creator.OwnerTelegramID, lang)
	if err != nil {
		return fmt.Errorf("creator reconnect url: %w", err)
	}
	view := buildCreatorReconnectRequiredView(lang, reconnectURL)
	if messageID := c.sendMsg(ctx, creator.OwnerTelegramID, view.text, &view.opts); messageID == 0 {
		return errReconnectNotificationSend
	}
	return nil
}

// HandleSubscriptionEnd applies subscription-end side effects for a viewer.
func (c *Controller) HandleSubscriptionEnd(ctx context.Context, broadcasterID, broadcasterLogin, twitchUserID, twitchLogin string) error {
	res, err := c.app.SubscriptionEnd.Prepare(ctx, broadcasterID, broadcasterLogin, twitchUserID, twitchLogin)
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
