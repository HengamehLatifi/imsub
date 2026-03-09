package flows

import (
	"fmt"
	"html"

	"imsub/internal/platform/i18n"
)

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
