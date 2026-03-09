package flows

import (
	"testing"

	"imsub/internal/platform/i18n"

	"github.com/mymmrac/telego"
)

func TestBuildCreatorOAuthFailureViewReconnectMismatchUsesHTML(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildCreatorOAuthFailureView("en", msgCreatorReconnectMismatch)
	if view.text == "" {
		t.Fatal("text = empty, want translated reconnect mismatch text")
	}
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
}

func TestBuildCreatorReconnectRequiredViewIncludesMarkup(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildCreatorReconnectRequiredView("en", "https://example.com/reconnect")
	if view.text == "" {
		t.Fatal("text = empty, want translated reconnect-needed text")
	}
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
	if view.opts.Markup == nil {
		t.Fatal("Markup = nil, want reconnect markup")
	}
}

func TestBuildSubscriptionEndViewIncludesSubscribeMarkup(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildSubscriptionEndView("en", "viewer_name", "creator_name")
	if view.text == "" {
		t.Fatal("text = empty, want translated subscription-end text")
	}
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
	if view.opts.Markup == nil {
		t.Fatal("Markup = nil, want subscribe CTA markup")
	}
}
