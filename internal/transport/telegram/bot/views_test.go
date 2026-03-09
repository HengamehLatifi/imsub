package bot

import "testing"

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

func TestBuildHTMLTextView(t *testing.T) {
	t.Parallel()
	view := buildHTMLTextView("en", msgCmdHelp)
	if view.text == "" {
		t.Fatalf("buildHTMLTextView() = %+v, want text", view)
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

func TestBuildCreatorOAuthFailureViewReconnectMismatchUsesHTML(t *testing.T) {
	t.Parallel()
	view := buildCreatorOAuthFailureView("en", msgCreatorReconnectMismatch)
	if view.opts.ParseMode == "" {
		t.Fatalf("buildCreatorOAuthFailureView() = %+v, want HTML parse mode", view)
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
