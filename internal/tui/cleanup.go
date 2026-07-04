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
	reasons        []string // parallel to staleWorktrees; "branch deleted" or "merged"
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
		if i < len(m.cleanup.reasons) && m.cleanup.reasons[i] != "" {
			line += " " + mutedStyle.Render("["+m.cleanup.reasons[i]+"]")
		}
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
		var paths, branches []string
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
			paths = append(paths, wt.Path)
			branch := ""
			if !wt.Detached {
				branch = wt.Branch
			}
			branches = append(branches, branch)
		}
		if len(paths) == 0 {
			m.mode = modeList
			return m, m.loadWorktrees
		}
		m.errMsg = ""
		cmd := startBusy(&m, fmt.Sprintf("Cleaning up %d worktree(s)...", len(paths)),
			cleanupWorktreesCmd(m.git, paths, branches))
		return m, cmd
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
// non-locked, non-detached, non-dirty, and whose branch is either not a
// local branch anymore (deleted) or is fully merged into the base branch
// (mergedSet).
func classifyStale(wts []model.Worktree, branchSet map[string]bool, mergedSet map[string]bool) []int {
	var stale []int
	for i, wt := range wts {
		if wt.IsMain || wt.IsLocked || wt.Detached {
			continue
		}
		if wt.Status != nil && wt.Status.IsDirty {
			continue
		}
		if !branchSet[wt.Branch] || mergedSet[wt.Branch] {
			stale = append(stale, i)
		}
	}
	return stale
}

// staleReason returns a short human-readable reason a branch was classified
// as stale, for display in the cleanup modal. Empty string if neither
// condition applies (should not happen for indices returned by
// classifyStale).
func staleReason(branch string, branchSet, mergedSet map[string]bool) string {
	switch {
	case !branchSet[branch]:
		return "branch deleted"
	case mergedSet[branch]:
		return "merged"
	default:
		return ""
	}
}

// mergedBranchSet returns the set of local branches fully merged into base,
// excluding base itself. Returns an empty (non-nil) set on error so callers
// can treat merge-detection as best-effort without special-casing errors.
func (m *Model) mergedBranchSet(base string) map[string]bool {
	mergedSet := make(map[string]bool)
	merged, err := m.git.MergedBranches(base)
	if err != nil {
		return mergedSet
	}
	for _, b := range merged {
		if b != base {
			mergedSet[b] = true
		}
	}
	return mergedSet
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
	base, _ := initialBase(branches)
	mergedSet := m.mergedBranchSet(base)

	m.cleanup.staleWorktrees = classifyStale(m.worktrees, branchSet, mergedSet)
	m.cleanup.selected = make([]bool, len(m.cleanup.staleWorktrees))
	m.cleanup.reasons = make([]string, len(m.cleanup.staleWorktrees))
	for i, idx := range m.cleanup.staleWorktrees {
		m.cleanup.reasons[i] = staleReason(m.worktrees[idx].Branch, branchSet, mergedSet)
	}
}
