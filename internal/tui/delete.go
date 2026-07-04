package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// deleteModel holds transient state for the delete-confirmation modal.
type deleteModel struct {
	alsoDeleteBranch bool
}

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

	if !wt.Detached && wt.Branch != "" {
		checkbox := "[ ]"
		if m.delete.alsoDeleteBranch {
			checkbox = "[x]"
		}
		b.WriteString(fmt.Sprintf("%s Also delete branch %q\n\n", checkbox, wt.Branch))
	}

	confirmHint := "[y]es [n]o"
	if wt.Status != nil && wt.Status.IsDirty {
		confirmHint = "[y] force-remove [n]o"
	}
	if !wt.Detached && wt.Branch != "" {
		confirmHint += " [b] toggle branch delete"
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
			m.errMsg = ""
			alsoDeleteBranch := m.delete.alsoDeleteBranch && !wt.Detached && wt.Branch != ""
			cmd := startBusy(&m, "Removing worktree...",
				removeWorktreeCmd(m.git, wt.Path, wt.Branch, force, alsoDeleteBranch))
			return m, cmd
		case "n":
			m.mode = modeList
			return m, nil
		case "b":
			m.delete.alsoDeleteBranch = !m.delete.alsoDeleteBranch
			return m, nil
		}
	case tea.KeySpace:
		m.delete.alsoDeleteBranch = !m.delete.alsoDeleteBranch
		return m, nil
	case tea.KeyEscape:
		m.mode = modeList
		return m, nil
	}
	return m, nil
}
