package bot

import (
	"strings"
	"testing"

	"imsub/internal/core"
)

func TestCreatorGroupLineEscapesHTML(t *testing.T) {
	t.Parallel()

	line := CreatorGroupLines("en", `name<&>`, []core.ManagedGroup{{GroupName: `group "x"`}})
	if !strings.Contains(line, "name&lt;&amp;&gt;") {
		t.Errorf("CreatorGroupLines() = %q, want escaped creator name", line)
	}
	if !strings.Contains(line, "group &#34;x&#34;") {
		t.Errorf("CreatorGroupLines() = %q, want escaped group name", line)
	}
}

func TestBuildCreatorPromptView(t *testing.T) {
	t.Parallel()

	view := buildCreatorPromptView("en", "https://example.com", false)
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildCreatorPromptView() = %+v, want non-empty text and markup", view)
	}
}

func TestBuildCreatorStatusViewNoGroups(t *testing.T) {
	t.Parallel()

	view := buildCreatorStatusView("en", "", core.Creator{TwitchLogin: "creator"}, core.Status{}, nil)
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildCreatorStatusView() = %+v, want non-empty text and markup", view)
	}
}

func TestBuildCreatorStatusViewWithSingleGroup(t *testing.T) {
	t.Parallel()

	view := buildCreatorStatusView("en", "", core.Creator{TwitchLogin: "creator"}, core.Status{}, []core.ManagedGroup{{ChatID: 1, GroupName: "VIP"}})
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildCreatorStatusView() = %+v, want non-empty text and markup", view)
	}
}

func TestBuildCreatorStatusViewWithMultipleGroups(t *testing.T) {
	t.Parallel()

	view := buildCreatorStatusView("en", "", core.Creator{TwitchLogin: "creator"}, core.Status{}, []core.ManagedGroup{{ChatID: 1, GroupName: "VIP"}, {ChatID: 2, GroupName: "Patrons"}})
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildCreatorStatusView() = %+v, want non-empty text and markup", view)
	}
}

func TestBuildCreatorManagedGroupsView(t *testing.T) {
	t.Parallel()

	view := buildCreatorManagedGroupsView("en", core.Creator{TwitchLogin: "creator"}, []core.ManagedGroup{{ChatID: 1, GroupName: "VIP"}}, "")
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildCreatorManagedGroupsView() = %+v, want non-empty text and markup", view)
	}
}

func TestBuildCreatorGroupUnregisterConfirmView(t *testing.T) {
	t.Parallel()

	view := buildCreatorGroupUnregisterConfirmView("en", core.Creator{TwitchLogin: "creator"}, core.ManagedGroup{ChatID: 1, GroupName: "VIP"}, creatorMenuCallback())
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildCreatorGroupUnregisterConfirmView() = %+v, want non-empty text and markup", view)
	}
}
