package flows

import (
	"testing"

	"imsub/internal/platform/i18n"

	"github.com/mymmrac/telego"
)

func TestBuildMainMenuTextView(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildMainMenuTextView("en", msgCmdHelp)
	if view.text == "" {
		t.Fatal("text = empty, want translated help text")
	}
	if view.opts.Markup == nil {
		t.Fatal("Markup = nil, want main menu markup")
	}
}

func TestBuildHTMLTextView(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildHTMLTextView("en", msgResetExitHTML)
	if view.text == "" {
		t.Fatal("text = empty, want translated HTML text")
	}
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
}

func TestBuildViewerStatusErrorView(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildViewerStatusErrorView("en")
	if view.text == "" {
		t.Fatal("text = empty, want translated status error text")
	}
	if view.opts.Markup == nil {
		t.Fatal("Markup = nil, want viewer main menu markup")
	}
}

func TestBuildCreatorLinkErrorView(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildCreatorLinkErrorView("en")
	if view.text == "" {
		t.Fatal("text = empty, want translated creator link error text")
	}
	if view.opts.Markup == nil {
		t.Fatal("Markup = nil, want creator main menu markup")
	}
}
