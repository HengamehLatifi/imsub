package flows

import (
	"strings"
	"testing"

	"imsub/internal/core"
	"imsub/internal/platform/i18n"

	"github.com/mymmrac/telego"
)

func TestBuildViewerPromptView(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildViewerPromptView("en", "Viewer <Name>", "https://example.com/auth")
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
	if !strings.Contains(view.text, "Viewer &lt;Name&gt;") {
		t.Fatalf("text = %q, want escaped user name", view.text)
	}
	if view.opts.Markup == nil {
		t.Fatal("Markup = nil, want inline keyboard")
	}
}

func TestBuildViewerLinkedView(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildViewerLinkedView("en", "viewer_login", core.JoinTargets{
		ActiveCreatorNames: []string{"alpha"},
		JoinLinks: []core.JoinLink{
			{CreatorName: "alpha", GroupName: "VIP", InviteLink: "https://invite"},
		},
	})
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
	if !view.opts.DisablePreview {
		t.Fatal("DisablePreview = false, want true")
	}
	if !strings.Contains(view.text, "viewer_login") {
		t.Fatalf("text = %q, want twitch login present", view.text)
	}
	if view.opts.Markup == nil {
		t.Fatal("Markup = nil, want inline keyboard")
	}
}
