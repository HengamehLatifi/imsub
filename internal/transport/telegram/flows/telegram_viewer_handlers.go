package flows

import (
	"imsub/internal/platform/i18n"

	"github.com/mymmrac/telego"
	tghandler "github.com/mymmrac/telego/telegohandler"
)

// onStartCommand handles /start by initiating the viewer flow.
func (c *Controller) onStartCommand(ctx *tghandler.Context, msg telego.Message) error {
	lang := i18n.NormalizeLanguage(msg.From.LanguageCode)
	c.handleViewerStartForUser(ctx, msg.From.ID, 0, lang, msg.From.FirstName)
	return nil
}
