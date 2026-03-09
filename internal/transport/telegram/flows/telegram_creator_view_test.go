package flows

import (
	"strings"
	"testing"
	"time"

	"imsub/internal/core"
	"imsub/internal/platform/i18n"

	"github.com/mymmrac/telego"
)

func TestBuildCreatorPromptView(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildCreatorPromptView("en", "https://example.com/auth", false)
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
	if view.opts.Markup == nil {
		t.Fatal("Markup = nil, want creator prompt buttons")
	}
	if !strings.Contains(view.text, "Twitch") && view.text == "" {
		t.Fatalf("text = %q, want non-empty prompt text", view.text)
	}
}

func TestBuildCreatorStatusViewNoGroups(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildCreatorStatusView("en", "", core.Creator{Name: "streamer"}, core.Status{
		Auth:       core.CreatorAuthHealthy,
		LastSyncAt: time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC),
	}, nil)
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
	if !view.opts.DisablePreview {
		t.Fatal("DisablePreview = false, want true")
	}
	if !strings.Contains(view.text, "streamer") {
		t.Fatalf("text = %q, want creator name present", view.text)
	}
}

func TestBuildCreatorStatusViewWithGroups(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildCreatorStatusView("en", "https://example.com/reconnect", core.Creator{Name: "streamer"}, core.Status{
		EventSub:           core.EventSubActive,
		Auth:               core.CreatorAuthReconnectRequired,
		AuthStatusAt:       time.Date(2026, 3, 8, 11, 0, 0, 0, time.UTC),
		LastSyncAt:         time.Date(2026, 3, 8, 12, 0, 0, 0, time.UTC),
		HasSubscriberCount: true,
		SubscriberCount:    42,
	}, []core.ManagedGroup{{GroupName: "VIP"}})
	if view.opts.Markup == nil {
		t.Fatal("Markup = nil, want creator status menu")
	}
	if !strings.Contains(view.text, "streamer") || !strings.Contains(view.text, "VIP") {
		t.Fatalf("text = %q, want creator and group names present", view.text)
	}
}
