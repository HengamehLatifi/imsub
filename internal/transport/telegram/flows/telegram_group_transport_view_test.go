package flows

import (
	"testing"

	"imsub/internal/platform/i18n"

	"github.com/mymmrac/telego"
)

func TestBuildGroupReplyView(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildGroupReplyView("en", msgGroupNotAdmin, 42)
	if view.text == "" {
		t.Fatal("text = empty, want translated group reply text")
	}
	if view.opts.ReplyToMessageID != 42 {
		t.Fatalf("ReplyToMessageID = %d, want 42", view.opts.ReplyToMessageID)
	}
}

func TestBuildGroupSettingWarningsView(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildGroupSettingWarningsView("en", 99, []string{"issue one"})
	if view.text == "" {
		t.Fatal("text = empty, want warning block")
	}
	if view.opts.ReplyToMessageID != 99 {
		t.Fatalf("ReplyToMessageID = %d, want 99", view.opts.ReplyToMessageID)
	}
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
}

func TestBuildGroupBotStatusChangedView(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildGroupBotStatusChangedView("en")
	if view.text == "" {
		t.Fatal("text = empty, want translated bot status changed text")
	}
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
}
