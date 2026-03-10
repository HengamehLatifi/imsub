package bot

import (
	"strings"
	"testing"

	"imsub/internal/core"
)

func TestJoinNonEmptyLines(t *testing.T) {
	t.Parallel()
	if got := joinNonEmptyLines("a", "", "b"); got != "a\nb" {
		t.Fatalf("joinNonEmptyLines() = %q, want %q", got, "a\nb")
	}
}

func TestJoinNonEmptySections(t *testing.T) {
	t.Parallel()
	if got := joinNonEmptySections(textSection{text: "a"}, textSection{text: ""}, textSection{text: "b"}); got != "a\n\nb" {
		t.Fatalf("joinNonEmptySections() = %q, want %q", got, "a\n\nb")
	}
}

func TestRenderWarningBlock(t *testing.T) {
	t.Parallel()
	if got := renderWarningBlock("title", []string{"x", "y"}); got == "" {
		t.Fatal("renderWarningBlock() = empty, want text")
	}
}

func TestBuildMainMenuTextView(t *testing.T) {
	t.Parallel()
	view := buildMainMenuTextView("en", msgCmdHelp)
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildMainMenuTextView() = %+v, want text and markup", view)
	}
}

func TestBuildViewerStatusErrorView(t *testing.T) {
	t.Parallel()
	view := buildViewerStatusErrorView("en")
	if view.text == "" {
		t.Fatalf("buildViewerStatusErrorView() = %+v, want text", view)
	}
}

func TestBuildCreatorLinkErrorView(t *testing.T) {
	t.Parallel()
	view := buildCreatorLinkErrorView("en")
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildCreatorLinkErrorView() = %+v, want text and markup", view)
	}
}

func TestBuildCreatorOAuthFailureViewReconnectMismatchReturnsText(t *testing.T) {
	t.Parallel()
	view := buildCreatorOAuthFailureView("en", msgCreatorReconnectMismatch)
	if view.text == "" {
		t.Fatalf("buildCreatorOAuthFailureView() = %+v, want text", view)
	}
}

func TestBuildCreatorReconnectRequiredViewIncludesMarkup(t *testing.T) {
	t.Parallel()
	view := buildCreatorReconnectRequiredView("en", "https://example.com")
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildCreatorReconnectRequiredView() = %+v, want text and markup", view)
	}
}

func TestBuildSubscriptionEndViewIncludesSubscribeMarkup(t *testing.T) {
	t.Parallel()
	view := buildSubscriptionEndView("en", "viewer1", "streamer1")
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildSubscriptionEndView() = %+v, want text and markup", view)
	}
}

func TestBuildSubscriptionEndViewEscapesViewerLogin(t *testing.T) {
	t.Parallel()

	view := buildSubscriptionEndView("en", "<viewer>", "streamer1")
	if !strings.Contains(view.text, "&lt;viewer&gt;") {
		t.Fatalf("buildSubscriptionEndView() text = %q, want escaped viewer login", view.text)
	}
	if strings.Contains(view.text, "<viewer>") {
		t.Fatalf("buildSubscriptionEndView() text = %q, did not expect raw viewer login", view.text)
	}
}

func TestBuildSubscriptionStartViewIncludesJoinButtons(t *testing.T) {
	t.Parallel()

	view := buildSubscriptionStartView("en", "streamer1", core.JoinTargets{
		JoinLinks: []core.JoinLink{{
			CreatorName: "streamer1",
			GroupName:   "VIP",
			InviteLink:  "https://t.me/+invite",
		}},
	})
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildSubscriptionStartView() = %+v, want text and markup", view)
	}
}

func TestBuildGroupBotRemovedOwnerViewEscapesGroupName(t *testing.T) {
	t.Parallel()

	view := buildGroupBotRemovedOwnerView("en", "<VIP>", false)
	if !strings.Contains(view.text, "&lt;VIP&gt;") {
		t.Fatalf("buildGroupBotRemovedOwnerView() text = %q, want escaped group name", view.text)
	}
	if strings.Contains(view.text, "<VIP>") {
		t.Fatalf("buildGroupBotRemovedOwnerView() text = %q, did not expect raw group name", view.text)
	}
}
