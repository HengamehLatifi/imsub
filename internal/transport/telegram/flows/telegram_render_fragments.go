package flows

import "strings"

type textSection struct {
	text string
}

func joinNonEmptyLines(lines ...string) string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func joinNonEmptySections(sections ...textSection) string {
	out := make([]string, 0, len(sections))
	for _, section := range sections {
		text := strings.TrimSpace(section.text)
		if text == "" {
			continue
		}
		out = append(out, text)
	}
	return strings.Join(out, "\n\n")
}

func renderWarningBlock(title string, warnings []string) string {
	if len(warnings) == 0 {
		return ""
	}
	lines := make([]string, 0, len(warnings)+1)
	lines = append(lines, title)
	lines = append(lines, warnings...)
	return joinNonEmptyLines(lines...)
}
