package flows

import (
	"imsub/internal/transport/telegram/client"

	"github.com/mymmrac/telego"
)

func buildGroupReplyView(lang, key string, replyToMessageID int) sharedView {
	view := buildTextView(lang, key)
	view.opts = client.MessageOptions{
		ReplyToMessageID: replyToMessageID,
	}
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
		opts: client.MessageOptions{
			ParseMode: telego.ModeHTML,
		},
	}
}

func buildGroupBotStatusChangedView(lang string) sharedView {
	return buildHTMLTextView(lang, msgGroupBotStatusChanged)
}
