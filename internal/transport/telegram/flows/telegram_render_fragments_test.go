package flows

import "testing"

func TestJoinNonEmptyLines(t *testing.T) {
	t.Parallel()

	got := joinNonEmptyLines("first", "", " second ", "   ")
	if got != "first\nsecond" {
		t.Fatalf("joinNonEmptyLines() = %q, want %q", got, "first\nsecond")
	}
}

func TestJoinNonEmptySections(t *testing.T) {
	t.Parallel()

	got := joinNonEmptySections(
		textSection{text: "first"},
		textSection{text: ""},
		textSection{text: " second "},
	)
	if got != "first\n\nsecond" {
		t.Fatalf("joinNonEmptySections() = %q, want %q", got, "first\n\nsecond")
	}
}

func TestRenderWarningBlock(t *testing.T) {
	t.Parallel()

	got := renderWarningBlock("Warnings", []string{"one", "two"})
	if got != "Warnings\none\ntwo" {
		t.Fatalf("renderWarningBlock() = %q, want %q", got, "Warnings\none\ntwo")
	}
}
