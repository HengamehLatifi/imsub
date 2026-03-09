package flows

import (
	"strings"
	"testing"

	"imsub/internal/platform/i18n"
)

func TestGroupCapabilityEvaluationIssuesBotMissing(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	got := (groupCapabilityEvaluation{botMissing: true}).issues("en")
	if len(got) != 1 || !strings.Contains(got[0], "bot") {
		t.Fatalf("issues = %v, want bot-missing warning", got)
	}
}

func TestGroupSettingsEvaluationIssues(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	got := (groupSettingsEvaluation{
		botCapabilities: groupCapabilityEvaluation{
			canInviteUsers:   false,
			canRestrictUsers: false,
		},
		isPublic:       true,
		joinByRequest:  false,
		untrackedCount: 3,
	}).issues("en")

	if len(got) != 5 {
		t.Fatalf("issues len = %d, want 5, issues=%v", len(got), got)
	}
}
