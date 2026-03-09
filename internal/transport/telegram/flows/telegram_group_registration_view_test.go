package flows

import (
	"strings"
	"testing"

	"imsub/internal/core"
	"imsub/internal/platform/i18n"
	"imsub/internal/usecase"

	"github.com/mymmrac/telego"
)

func TestBuildGroupRegistrationViewTakenByOther(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view, ok := buildGroupRegistrationView("en", 12, usecase.RegisterGroupResult{
		Outcome:          usecase.RegisterGroupOutcomeTakenByOther,
		OtherCreatorName: "other",
	})
	if !ok {
		t.Fatal("buildGroupRegistrationView() ok = false, want true")
	}
	if view.opts.ReplyToMessageID != 12 {
		t.Fatalf("ReplyToMessageID = %d, want 12", view.opts.ReplyToMessageID)
	}
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
	if !strings.Contains(view.text, "other") {
		t.Fatalf("text = %q, want creator name present", view.text)
	}
	if view.dispatchFollowUp {
		t.Fatal("dispatchFollowUp = true, want false")
	}
}

func TestBuildGroupRegistrationViewAlreadyLinked(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view, ok := buildGroupRegistrationView("en", 12, usecase.RegisterGroupResult{
		Outcome: usecase.RegisterGroupOutcomeAlreadyLinked,
		Creator: core.Creator{Name: "streamer"},
	})
	if !ok {
		t.Fatal("buildGroupRegistrationView() ok = false, want true")
	}
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
	if !strings.Contains(view.groupBaseText, "streamer") {
		t.Fatalf("groupBaseText = %q, want creator name present", view.groupBaseText)
	}
	if !strings.Contains(view.text, view.groupBaseText) {
		t.Fatalf("text = %q, want base text included", view.text)
	}
	if !strings.Contains(view.text, i18n.Translate("en", msgGroupCheckingSettings)) {
		t.Fatalf("text = %q, want checking settings suffix", view.text)
	}
	if !view.dispatchFollowUp {
		t.Fatal("dispatchFollowUp = false, want true")
	}
}

func TestBuildGroupRegistrationViewRegistered(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view, ok := buildGroupRegistrationView("en", 12, usecase.RegisterGroupResult{
		Outcome: usecase.RegisterGroupOutcomeRegistered,
		Creator: core.Creator{Name: "streamer"},
	})
	if !ok {
		t.Fatal("buildGroupRegistrationView() ok = false, want true")
	}
	if !strings.Contains(view.groupBaseText, "streamer") {
		t.Fatalf("groupBaseText = %q, want creator name present", view.groupBaseText)
	}
	if !view.dispatchFollowUp {
		t.Fatal("dispatchFollowUp = false, want true")
	}
}

func TestBuildGroupRegistrationViewUnsupportedOutcome(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	_, ok := buildGroupRegistrationView("en", 12, usecase.RegisterGroupResult{
		Outcome: usecase.RegisterGroupOutcome("unsupported"),
	})
	if ok {
		t.Fatal("buildGroupRegistrationView() ok = true, want false")
	}
}
