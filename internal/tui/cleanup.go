package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
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
		if len(m.cleanup.staleWorktrees) == 0 {
			m.mode = modeList
			return m, m.loadWorktrees
		}
		var errs []string
		for i, idx := range m.cleanup.staleWorktrees {
			if !m.cleanup.selected[i] {
				continue
			}
			wt := m.worktrees[idx]
			if wt.IsMain || wt.IsLocked {
				continue
			}
			if wt.Status != nil && wt.Status.IsDirty {
				continue
			}
			if err := m.git.RemoveWorktree(wt.Path, false); err != nil {
				errs = append(errs, fmt.Sprintf("failed to remove %s: %v", wt.Path, err))
			}
		}
		m.mode = modeList
		if len(errs) > 0 {
			m.errMsg = strings.Join(errs, "; ")
		} else {
			m.errMsg = ""
		}
		return m, m.loadWorktrees
	case tea.KeySpace:
		if len(m.cleanup.selected) > 0 {
			m.cleanup.selected[m.cleanup.currentIndex] = !m.cleanup.selected[m.cleanup.currentIndex]
		}
		return m, nil
	case tea.KeyRunes:
		switch msg.String() {
		case "j":
			if m.cleanup.currentIndex < len(m.cleanup.staleWorktrees)-1 {
				m.cleanup.currentIndex++
			}
			return m, nil
		case "k":
			if m.cleanup.currentIndex > 0 {
				m.cleanup.currentIndex--
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
	case tea.KeyDown:
		if m.cleanup.currentIndex < len(m.cleanup.staleWorktrees)-1 {
			m.cleanup.currentIndex++
		}
		return m, nil
	case tea.KeyUp:
		if m.cleanup.currentIndex > 0 {
			m.cleanup.currentIndex--
		}
		return m, nil
	}
	return m, nil
}

// classifyStale returns the indices of worktrees that are stale: non-main,
// non-locked, non-detached, non-dirty, and whose branch is not a local
// branch in branchSet.
func classifyStale(wts []model.Worktree, branchSet map[string]bool) []int {
	var stale []int
	for i, wt := range wts {
		if wt.IsMain || wt.IsLocked || wt.Detached {
			continue
		}
		if wt.Status != nil && wt.Status.IsDirty {
			continue
		}
		if !branchSet[wt.Branch] {
			stale = append(stale, i)
		}
	}
	return stale
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

	m.cleanup.staleWorktrees = classifyStale(m.worktrees, branchSet)
	m.cleanup.selected = make([]bool, len(m.cleanup.staleWorktrees))
}