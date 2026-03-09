package flows

import (
	"fmt"

	"imsub/internal/platform/i18n"
)

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
