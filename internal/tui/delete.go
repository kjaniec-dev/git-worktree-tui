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

	b.WriteString(helpStyle.Render("[y]es [n]o"))

	return b.String()
}

func (m Model) handleDeleteKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes:
		switch msg.String() {
		case "y":
			return m, m.deleteWorktree
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

func (m Model) deleteWorktree() tea.Msg {
	if m.selected >= len(m.worktrees) {
		return errMsg("No worktree selected")
	}

	wt := m.worktrees[m.selected]

	if wt.IsMain {
		return errMsg("Cannot delete main worktree")
	}

	if wt.IsLocked {
		return errMsg("Worktree is locked, unlock first")
	}

	err := m.git.RemoveWorktree(wt.Path)
	if err != nil {
		return errMsg(err.Error())
	}

	return worktreesLoadedMsg{worktrees: nil} // Trigger reload
}