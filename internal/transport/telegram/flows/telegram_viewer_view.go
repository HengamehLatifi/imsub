package flows

import (
	"fmt"
	"html"
	"strings"

	"imsub/internal/core"
	"imsub/internal/platform/i18n"
	"imsub/internal/transport/telegram/client"
	"imsub/internal/transport/telegram/ui"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

func buildViewerPromptView(lang, userName, authURL string) sharedView {
	displayName := strings.TrimSpace(userName)
	if displayName == "" {
		displayName = i18n.Translate(lang, msgUserGenericName)
	}

	return sharedView{
		text: fmt.Sprintf(i18n.Translate(lang, msgLinkPromptHTML), html.EscapeString(displayName)),
		opts: client.MessageOptions{
			ParseMode: telego.ModeHTML,
			Markup: tu.InlineKeyboard(
				tu.InlineKeyboardRow(ui.LinkButton(i18n.Translate(lang, btnLinkTwitch), authURL)),
				tu.InlineKeyboardRow(ui.CopyLinkButton(i18n.Translate(lang, btnCopyLink), authURL)),
			),
		},
	}
}

func buildViewerLinkedView(lang, twitchLogin string, targets core.JoinTargets) sharedView {
	joinRows := renderJoinButtons(targets, lang)
	return sharedView{
		text: ui.LinkedStatusWithJoinStateHTML(lang, twitchLogin, targets.ActiveCreatorNames, len(joinRows) > 0),
		opts: client.MessageOptions{
			ParseMode:      telego.ModeHTML,
			Markup:         ui.WithMainMenu(lang, viewerMainMenuCallbacks(), joinRows...),
			DisablePreview: true,
		},
	}
}
