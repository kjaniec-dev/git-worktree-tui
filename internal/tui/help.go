package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// helpKeybindings lists the list-mode keybindings shown in the help overlay,
// in the order they should be displayed.
var helpKeybindings = [][2]string{
	{"↑/↓, j/k", "Navigate worktree list"},
	{"Enter", "Copy full path to clipboard"},
	{"a", "Create new worktree"},
	{"d", "Delete selected worktree (b: also delete branch)"},
	{"l", "Lock/unlock selected worktree"},
	{"c", "Cleanup stale/merged worktrees"},
	{"g", "Print path & quit (for shell cd integration)"},
	{"o", "Copy \"cd <path>\" to clipboard"},
	{"/", "Filter by branch name"},
	{"r", "Refresh list"},
	{"?", "Toggle this help screen"},
	{"q", "Quit"},
}

// helpGlyphLegend lists the status glyphs shown in the worktree list, in the
// order they should be displayed.
var helpGlyphLegend = [][2]string{
	{"★", "Main worktree"},
	{"🔒", "Locked worktree"},
	{"●", "Dirty (uncommitted changes)"},
	{"○", "Clean"},
	{"?", "Status unknown (not yet loaded)"},
	{"(here)", "Worktree containing your current directory"},
	{"↑N ↓N", "Commits ahead/behind upstream"},
}

func (m Model) viewHelpModal() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Help"))
	b.WriteString("\n\n")

	b.WriteString(mutedStyle.Render("Keybindings"))
	b.WriteString("\n")
	for _, kb := range helpKeybindings {
		b.WriteString(formatHelpRow(kb[0], kb[1]))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Glyphs"))
	b.WriteString("\n")
	for _, g := range helpGlyphLegend {
		b.WriteString(formatHelpRow(g[0], g[1]))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("[?] or [Esc] close"))

	return b.String()
}

// formatHelpRow left-pads the key column so descriptions line up, without
// depending on a fixed-width font assumption beyond simple ASCII padding.
func formatHelpRow(key, desc string) string {
	const keyColWidth = 14
	padded := key
	if len(key) < keyColWidth {
		padded = key + strings.Repeat(" ", keyColWidth-len(key))
	}
	return "  " + activeFieldStyle.Render(padded) + desc
}

func (m Model) handleHelpKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape, tea.KeyEnter:
		m.mode = modeList
		return m, nil
	case tea.KeyRunes:
		if msg.String() == "?" || msg.String() == "q" {
			m.mode = modeList
			return m, nil
		}
	}
	return m, nil
}
