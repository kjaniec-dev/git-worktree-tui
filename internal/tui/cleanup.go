package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type cleanupModel struct {
	staleWorktrees []int
	selected       []bool
	currentIndex   int
}

func (m Model) viewCleanupModal() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Cleanup Stale Worktrees"))
	b.WriteString("\n\n")

	if len(m.cleanup.staleWorktrees) == 0 {
		b.WriteString("No stale worktrees found.\n\n")
		b.WriteString(helpStyle.Render("[Esc] back"))
		return b.String()
	}

	b.WriteString("Worktrees with missing branches:\n\n")

	for i, idx := range m.cleanup.staleWorktrees {
		wt := m.worktrees[idx]
		prefix := "[ ]"
		if m.cleanup.selected[i] {
			prefix = "[x]"
		}
		
		line := fmt.Sprintf("%s %s (%s)", prefix, wt.Branch, wt.Path)
		if i == m.cleanup.currentIndex {
			line = selectedStyle.Render(line)
		}
		
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("[Space] toggle [a]ll [Esc] back [Enter] remove selected"))

	return b.String()
}

func (m Model) handleCleanupKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEscape:
		m.mode = modeList
		return m, nil
	case tea.KeyEnter:
		for i, idx := range m.cleanup.staleWorktrees {
			if !m.cleanup.selected[i] {
				continue
			}

			wt := m.worktrees[idx]

			if wt.IsLocked {
				continue
			}

			if wt.Status != nil && wt.Status.IsDirty {
				continue
			}

			err := m.git.RemoveWorktree(wt.Path)
			if err != nil {
				m.errMsg = fmt.Sprintf("Failed to remove %s: %v", wt.Path, err)
				return m, nil
			}
		}

		m.mode = modeList
		return m, m.loadWorktrees
	case tea.KeySpace:
		if len(m.cleanup.selected) > 0 {
			m.cleanup.selected[m.cleanup.currentIndex] = !m.cleanup.selected[m.cleanup.currentIndex]
		}
		return m, nil
	case tea.KeyRunes:
		switch msg.String() {
		case "up", "k":
			if m.cleanup.currentIndex > 0 {
				m.cleanup.currentIndex--
			}
			return m, nil
		case "down", "j":
			if m.cleanup.currentIndex < len(m.cleanup.staleWorktrees)-1 {
				m.cleanup.currentIndex++
			}
			return m, nil
		case "a":
			allSelected := true
			for _, s := range m.cleanup.selected {
				if !s {
					allSelected = false
					break
				}
			}
			for i := range m.cleanup.selected {
				m.cleanup.selected[i] = !allSelected
			}
			return m, nil
		}
	}
	return m, nil
}

func (m *Model) findStaleWorktrees() {
	branches, err := m.git.ListBranches()
	if err != nil {
		return
	}

	branchSet := make(map[string]bool)
	for _, b := range branches {
		branchSet[b] = true
	}

	m.cleanup.staleWorktrees = nil
	m.cleanup.selected = nil

	for i, wt := range m.worktrees {
		if wt.IsMain {
			continue
		}
		if !branchSet[wt.Branch] {
			m.cleanup.staleWorktrees = append(m.cleanup.staleWorktrees, i)
			m.cleanup.selected = append(m.cleanup.selected, false)
		}
	}
}