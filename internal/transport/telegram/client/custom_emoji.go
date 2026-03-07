package client

import (
	"strings"

	"github.com/mymmrac/telego"
)

type customEmojiReplacement struct {
	standard string
	customID string
}

var htmlCustomEmojiReplacer = strings.NewReplacer(customEmojiHTMLReplacements()...)

var htmlCustomEmojiReplacementTable = []customEmojiReplacement{
	{
		standard: "⏳",
		customID: "5386367538735104399",
	},
}

func customEmojiHTMLReplacements() []string {
	replacements := make([]string, 0, len(htmlCustomEmojiReplacementTable)*2)
	for _, replacement := range htmlCustomEmojiReplacementTable {
		if replacement.customID == "" {
			continue
		}
		replacements = append(
			replacements,
			replacement.standard,
			customEmojiTag(replacement.standard, replacement.customID),
		)
	}
	return replacements
}

func customEmojiTag(fallback, customEmojiID string) string {
	return `<tg-emoji emoji-id="` + customEmojiID + `">` + fallback + `</tg-emoji>`
}

func transformOutgoingText(text string, opts *MessageOptions) string {
	if text == "" || opts == nil || opts.ParseMode != telego.ModeHTML || !opts.EnableCustomEmoji {
		return text
	}
	return htmlCustomEmojiReplacer.Replace(text)
}
