package flows

import (
	"context"

	"imsub/internal/platform/i18n"

	"github.com/mymmrac/telego"
	tghandler "github.com/mymmrac/telego/telegohandler"
)

// onCreatorCommand handles /creator by initiating the creator registration
// or status flow.
func (c *Controller) onCreatorCommand(ctx *tghandler.Context, msg telego.Message) error {
	lang := i18n.NormalizeLanguage(msg.From.LanguageCode)
	c.handleCreatorRegistrationStart(ctx, msg.From.ID, 0, lang)
	return nil
}

func (c *Controller) handleCreatorCallback(ctx context.Context, userID int64, editMsgID int, lang string, action callbackAction) string {
	switch action.verb {
	case callbackVerbRefresh, callbackVerbRegister:
		return c.handleCreatorRegistrationStart(ctx, userID, editMsgID, lang)
	case callbackVerbReconnect:
		return c.handleCreatorReconnectStart(ctx, userID, editMsgID, lang)
	case callbackVerbOpen, callbackVerbPick, callbackVerbBack, callbackVerbMenu, callbackVerbCancel, callbackVerbExecute:
		// parseCallbackAction rejects these verbs for creator callbacks.
		c.log().Warn("unsupported creator callback verb", "telegram_user_id", userID, "verb", action.verb)
		return ""
	default:
		c.log().Warn("unsupported creator callback verb", "telegram_user_id", userID, "verb", action.verb)
		return ""
	}
}
