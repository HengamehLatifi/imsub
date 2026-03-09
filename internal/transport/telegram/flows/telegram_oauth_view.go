package flows

import (
	"imsub/internal/transport/telegram/client"

	"github.com/mymmrac/telego"
)

func buildViewerOAuthFailureView(lang, key string) sharedView {
	return buildTextView(lang, key)
}

func buildCreatorOAuthFailureView(lang, key string) sharedView {
	view := buildTextView(lang, key)
	if key == msgCreatorReconnectMismatch {
		view.opts = client.MessageOptions{
			ParseMode: telego.ModeHTML,
		}
	}
	return view
}

func buildOAuthLoadStatusErrorView(lang string) sharedView {
	return buildViewerStatusErrorView(lang)
}
