package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) viewDeleteModal() string {
	if m.selected >= len(m.worktrees) {
		return ""
	}

	wt := m.worktrees[m.selected]

	var b strings.Builder
	b.WriteString(titleStyle.Render("Delete Worktree"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Delete worktree for \"%s\"?\n", wt.Branch))
	b.WriteString(fmt.Sprintf("Path: %s\n\n", wt.Path))

	if wt.Status != nil && wt.Status.IsDirty {
		b.WriteString(errorStyle.Render("⚠ This worktree has uncommitted changes!\n"))
		b.WriteString(errorStyle.Render("⚠ Changes will be lost.\n\n"))
	} else {
		b.WriteString("⚠ This will remove the worktree directory.\n\n")
	}

	confirmHint := "[y]es [n]o"
	if wt.Status != nil && wt.Status.IsDirty {
		confirmHint = "[y] force-remove [n]o"
	}
	b.WriteString(helpStyle.Render(confirmHint))

	return b.String()
}

func (m Model) handleDeleteKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes:
		switch msg.String() {
		case "y":
			if m.selected >= len(m.worktrees) {
				m.errMsg = "No worktree selected"
				return m, nil
			}

			wt := m.worktrees[m.selected]

			if wt.IsMain {
				m.errMsg = "Cannot delete main worktree"
				return m, nil
			}

			if wt.IsLocked {
				m.errMsg = "Worktree is locked, unlock first"
				return m, nil
			}

			force := wt.Status != nil && wt.Status.IsDirty
			err := m.git.RemoveWorktree(wt.Path, force)
			if err != nil {
				m.errMsg = err.Error()
				return m, nil
			}

			m.mode = modeList
			return m, m.loadWorktrees
		case "n":
			m.mode = modeList
			return m, nil
		}
	case tea.KeyEscape:
		m.mode = modeList
		return m, nil
	}
	return m, nil
}
