package flows

import (
	"fmt"

	"imsub/internal/core"
	"imsub/internal/platform/i18n"
	"imsub/internal/transport/telegram/client"
	"imsub/internal/transport/telegram/ui"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

func buildCreatorPromptView(lang, authURL string, reconnect bool) sharedView {
	openKey := btnRegisterCreatorOpen
	textKey := msgCreatorRegisterInfo
	if reconnect {
		openKey = btnReconnectCreator
		textKey = msgCreatorReconnectInfo
	}

	return sharedView{
		text: i18n.Translate(lang, textKey),
		opts: client.MessageOptions{
			ParseMode: telego.ModeHTML,
			Markup: tu.InlineKeyboard(
				tu.InlineKeyboardRow(ui.LinkButton(i18n.Translate(lang, openKey), authURL)),
				tu.InlineKeyboardRow(ui.CopyLinkButton(i18n.Translate(lang, btnCopyLink), authURL)),
			),
		},
	}
}

func buildCreatorStatusView(lang, reconnectURL string, creator core.Creator, status core.Status, groups []core.ManagedGroup) sharedView {
	profileDisplay := ui.TwitchProfileHTML(creator.Name)
	groupLines := CreatorGroupLines(lang, creator.Name, groups)
	authStatus := creatorAuthStatusText(status, lang)
	statusDetails := creatorStatusDetailsText(status, lang)

	if len(groups) == 0 {
		return sharedView{
			text: fmt.Sprintf(
				i18n.Translate(lang, msgCreatorRegisteredNoGroup),
				profileDisplay,
				authStatus,
				statusDetails,
				groupLines,
			),
			opts: client.MessageOptions{
				ParseMode:      telego.ModeHTML,
				DisablePreview: true,
				Markup:         ui.WithCreatorStatusMenu(lang, reconnectURL, creatorMenuCallbacks()),
			},
		}
	}

	eventSubStatus := creatorEventSubStatusText(status, lang)
	subscriberStatus := creatorSubscriberStatusText(status, lang)
	return sharedView{
		text: fmt.Sprintf(
			i18n.Translate(lang, msgCreatorRegistered),
			profileDisplay,
			eventSubStatus,
			authStatus,
			statusDetails,
			subscriberStatus,
			groupLines,
		),
		opts: client.MessageOptions{
			ParseMode:      telego.ModeHTML,
			DisablePreview: true,
			Markup:         ui.WithCreatorStatusMenu(lang, reconnectURL, creatorMenuCallbacks()),
		},
	}
}
