package bot

import (
	"testing"

	"imsub/internal/core"
	"imsub/internal/usecase"
)

func TestBuildResetPromptView(t *testing.T) {
	t.Parallel()

	view, ok := buildResetPromptView("en", core.ScopeState{
		HasIdentity: true,
		HasCreator:  true,
	}, resetOriginViewer)
	if !ok || view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildResetPromptView() = (%+v, %t), want populated view", view, ok)
	}
}

func TestBuildResetPromptViewEmpty(t *testing.T) {
	t.Parallel()

	view, ok := buildResetPromptView("en", core.ScopeState{}, resetOriginViewer)
	if !ok || view.text == "" {
		t.Fatalf("buildResetPromptView() = (%+v, %t), want empty-state view", view, ok)
	}
}

func TestBuildResetExecutionView(t *testing.T) {
	t.Parallel()

	view := buildResetExecutionView("en", usecase.ResetResult{Scope: usecase.ResetScopeViewer, ViewerLogin: "viewer", GroupCount: 2})
	if view.text == "" {
		t.Fatalf("buildResetExecutionView() = %+v, want text", view)
	}
}

func TestBuildResetErrorView(t *testing.T) {
	t.Parallel()

	view := buildResetErrorView("en")
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildResetErrorView() = %+v, want text and markup", view)
	}
}
