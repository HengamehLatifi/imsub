package bot

import (
	"strings"
	"testing"

	"imsub/internal/core"
	"imsub/internal/platform/i18n"
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

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure failed: %v", err)
	}

	view := buildCreatorStatusView("en", "", core.Creator{TwitchLogin: "creator"}, core.Status{}, nil)
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildCreatorStatusView() = %+v, want non-empty text and markup", view)
	}
	if strings.Contains(view.text, i18n.Translate("en", msgCreatorBlocklistDisabled)) {
		t.Fatalf("buildCreatorStatusView() text = %q, want no blocklist status for inactive creator", view.text)
	}
	if strings.Contains(view.text, "Cached banned users") {
		t.Fatalf("buildCreatorStatusView() text = %q, want no banned user count for inactive creator", view.text)
	}
	for _, row := range view.opts.Markup.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData == creatorBlocklistToggleCallback() {
				t.Fatalf("buildCreatorStatusView() markup = %+v, want no blocklist toggle for inactive creator", view.opts.Markup.InlineKeyboard)
			}
		}
	}
}

func TestBuildCreatorStatusViewWithSingleGroup(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure failed: %v", err)
	}

	view := buildCreatorStatusView("en", "", core.Creator{TwitchLogin: "creator"}, core.Status{HasBannedUserCount: true, BannedUserCount: 2}, []core.ManagedGroup{{ChatID: 1, GroupName: "VIP"}})
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildCreatorStatusView() = %+v, want non-empty text and markup", view)
	}
	if !strings.Contains(view.text, i18n.Translate("en", msgCreatorBlocklistDisabled)) {
		t.Fatalf("buildCreatorStatusView() text = %q, want disabled blocklist status", view.text)
	}
	if !strings.Contains(view.text, "Cached banned users") {
		t.Fatalf("buildCreatorStatusView() text = %q, want cached banned users line", view.text)
	}
}

func TestBuildCreatorStatusViewWithMultipleGroups(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure failed: %v", err)
	}

	view := buildCreatorStatusView("en", "", core.Creator{TwitchLogin: "creator"}, core.Status{}, []core.ManagedGroup{{ChatID: 1, GroupName: "VIP"}, {ChatID: 2, GroupName: "Patrons"}})
	if view.text == "" || view.opts.Markup == nil {
		t.Fatalf("buildCreatorStatusView() = %+v, want non-empty text and markup", view)
	}
}

func TestBuildCreatorStatusViewWithBlocklistEnabled(t *testing.T) {
	t.Parallel()

	if err := i18n.Ensure(); err != nil {
		t.Fatalf("i18n.Ensure failed: %v", err)
	}

	view := buildCreatorStatusView("en", "", core.Creator{
		TwitchLogin:          "creator",
		BlocklistSyncEnabled: true,
	}, core.Status{HasBannedUserCount: true, BannedUserCount: 4}, []core.ManagedGroup{{ChatID: 1, GroupName: "VIP"}})
	if !strings.Contains(view.text, i18n.Translate("en", msgCreatorBlocklistEnabled)) {
		t.Fatalf("buildCreatorStatusView() text = %q, want enabled blocklist status", view.text)
	}
	if !strings.Contains(view.text, "<b>4</b>") {
		t.Fatalf("buildCreatorStatusView() text = %q, want banned user count", view.text)
	}
	if view.opts.Markup == nil || len(view.opts.Markup.InlineKeyboard) == 0 {
		t.Fatalf("buildCreatorStatusView() markup = %+v, want creator status menu", view.opts.Markup)
	}
	found := false
	for _, row := range view.opts.Markup.InlineKeyboard {
		for _, button := range row {
			if button.CallbackData == creatorBlocklistToggleCallback() {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("buildCreatorStatusView() markup = %+v, want blocklist toggle callback", view.opts.Markup.InlineKeyboard)
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
