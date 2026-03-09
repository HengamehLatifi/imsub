package flows

import (
	"strings"
	"testing"

	"imsub/internal/core"
	"imsub/internal/platform/i18n"
	"imsub/internal/usecase"

	"github.com/mymmrac/telego"
)

func TestBuildResetPromptView(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view, ok := buildResetPromptView("en", core.ScopeState{
		HasIdentity: true,
		Identity:    core.UserIdentity{TwitchLogin: "viewer"},
		HasCreator:  true,
		Creator:     core.Creator{Name: "streamer"},
	}, resetOriginViewer)
	if !ok {
		t.Fatal("buildResetPromptView() ok = false, want true")
	}
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
	if view.opts.Markup == nil {
		t.Fatal("Markup = nil, want reset scope picker")
	}
	if !strings.Contains(view.text, "viewer") || !strings.Contains(view.text, "streamer") {
		t.Fatalf("text = %q, want viewer and creator names present", view.text)
	}
}

func TestBuildResetPromptViewEmpty(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view, ok := buildResetPromptView("en", core.ScopeState{}, resetOriginViewer)
	if !ok {
		t.Fatal("buildResetPromptView(empty) ok = false, want true")
	}
	if view.text != i18n.Translate("en", msgResetNothingHTML) {
		t.Fatalf("text = %q, want reset-nothing text", view.text)
	}
}

func TestBuildResetExecutionView(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildResetExecutionView("en", usecase.ResetResult{
		Scope:       usecase.ResetScopeViewer,
		ViewerLogin: "viewer_login",
		GroupCount:  2,
	})
	if view.opts.ParseMode != telego.ModeHTML {
		t.Fatalf("ParseMode = %q, want %q", view.opts.ParseMode, telego.ModeHTML)
	}
	if !strings.Contains(view.text, "viewer_login") {
		t.Fatalf("text = %q, want viewer login present", view.text)
	}
}

func TestBuildResetErrorView(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure() error = %v", err)
	}

	view := buildResetErrorView("en")
	if view.text != i18n.Translate("en", msgErrReset) {
		t.Fatalf("text = %q, want err_reset translation", view.text)
	}
	if view.opts.Markup == nil {
		t.Fatal("Markup = nil, want viewer main menu")
	}
}
