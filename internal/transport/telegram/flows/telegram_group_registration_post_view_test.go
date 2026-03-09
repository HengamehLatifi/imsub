package flows

import (
	"strings"
	"testing"

	"imsub/internal/platform/i18n"
)

func TestFormatGroupSettingsResultWithWarnings(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	got := formatGroupSettingsResult("en", []string{"warn one", "warn two"})
	if !strings.Contains(got, "warn one") || !strings.Contains(got, "warn two") {
		t.Fatalf("formatGroupSettingsResult() = %q, want warnings included", got)
	}
	if !strings.Contains(got, i18n.Translate("en", msgGroupWarnSettingsIntro)) {
		t.Fatalf("formatGroupSettingsResult() = %q, want warning intro included", got)
	}
}

func TestRenderPostRegistrationCopy(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	got := renderPostRegistrationCopy(postRegistrationCopyInput{
		lang:          "en",
		groupName:     "VIP <Crew>",
		creatorName:   "streamer <name>",
		groupBaseText: "<b>group base</b>",
	}, []string{"warning"})

	if !strings.Contains(got.draftDM, i18n.Translate("en", msgGroupCheckingSettings)) {
		t.Fatalf("draftDM = %q, want checking status", got.draftDM)
	}
	if !strings.Contains(got.finalDM, "VIP &lt;Crew&gt;") {
		t.Fatalf("finalDM = %q, want escaped group name", got.finalDM)
	}
	if !strings.Contains(got.finalDM, "streamer &lt;name&gt;") {
		t.Fatalf("finalDM = %q, want escaped creator name", got.finalDM)
	}
	if !strings.Contains(got.finalDM, "warning") {
		t.Fatalf("finalDM = %q, want warning included", got.finalDM)
	}
	if got.groupMessage != "<b>group base</b>\n\n"+formatGroupSettingsResult("en", []string{"warning"}) {
		t.Fatalf("groupMessage = %q, want base text plus settings result", got.groupMessage)
	}
}
