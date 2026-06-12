package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type createField int

const (
	fieldBranch createField = iota
	fieldBase
	fieldCreateBranch
)

type createModel struct {
	branchName    string
	baseBranch    string
	createBranch  bool
	currentField  createField
	branches      []string
	baseIndex     int
	errMsg        string
}

func generateWorktreePath(repoRoot, branch string) string {
	safeBranch := strings.ReplaceAll(branch, "/", "-")
	return filepath.Join(repoRoot, ".worktrees", safeBranch)
}

func (m Model) viewCreateModal() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Create Worktree"))
	b.WriteString("\n\n")

	branchLabel := fmt.Sprintf("Branch name: [%s]", m.create.branchName)
	baseLabel := fmt.Sprintf("Base: [%s]", m.create.baseBranch)
	checkboxLabel := fmt.Sprintf("☐ Create new branch from base: %v", m.create.createBranch)

	if m.create.currentField == fieldBranch {
		b.WriteString(selectedStyle.Render("→ " + branchLabel))
	} else {
		b.WriteString("  " + branchLabel)
	}
	b.WriteString("\n")

	if m.create.currentField == fieldBase {
		b.WriteString(selectedStyle.Render("→ " + baseLabel))
	} else {
		b.WriteString("  " + baseLabel)
	}
	b.WriteString("\n")

	if m.create.currentField == fieldCreateBranch {
		b.WriteString(selectedStyle.Render("→ " + checkboxLabel))
	} else {
		b.WriteString("  " + checkboxLabel)
	}
	b.WriteString("\n\n")

	path := generateWorktreePath(m.git.RepoRoot, m.create.branchName)
	b.WriteString(fmt.Sprintf("Path: %s (auto)\n\n", path))

	if m.create.errMsg != "" {
		b.WriteString(errorStyle.Render(m.create.errMsg))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("[Tab] next field [↑/↓] select base [Enter] create [Esc] cancel"))

	return b.String()
}

func (m Model) handleCreateKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		m.create.currentField = (m.create.currentField + 1) % 3
		return m, nil
	case tea.KeyShiftTab:
		m.create.currentField = (m.create.currentField - 1 + 3) % 3
		return m, nil
	case tea.KeyEscape:
		m.mode = modeList
		return m, nil
	case tea.KeyEnter:
		if m.create.branchName == "" {
			m.create.errMsg = "Branch name is required"
			return m, nil
		}
		path := generateWorktreePath(m.git.RepoRoot, m.create.branchName)
		err := m.git.AddWorktree(path, m.create.branchName, m.create.baseBranch, m.create.createBranch)
		if err != nil {
			m.create.errMsg = err.Error()
			return m, nil
		}
		m.mode = modeList
		return m, m.loadWorktrees
	case tea.KeyUp:
		if m.create.currentField == fieldBase && len(m.create.branches) > 0 {
			if m.create.baseIndex > 0 {
				m.create.baseIndex--
				m.create.baseBranch = m.create.branches[m.create.baseIndex]
			}
		}
		return m, nil
	case tea.KeyDown:
		if m.create.currentField == fieldBase && len(m.create.branches) > 0 {
			if m.create.baseIndex < len(m.create.branches)-1 {
				m.create.baseIndex++
				m.create.baseBranch = m.create.branches[m.create.baseIndex]
			}
		}
		return m, nil
	case tea.KeyBackspace:
		if m.create.currentField == fieldBranch && len(m.create.branchName) > 0 {
			m.create.branchName = m.create.branchName[:len(m.create.branchName)-1]
		} else if m.create.currentField == fieldBase && len(m.create.baseBranch) > 0 {
			m.create.baseBranch = m.create.baseBranch[:len(m.create.baseBranch)-1]
		}
		return m, nil
	case tea.KeySpace:
		if m.create.currentField == fieldCreateBranch {
			m.create.createBranch = !m.create.createBranch
		}
		return m, nil
	case tea.KeyRunes:
		if m.create.currentField == fieldBranch {
			m.create.branchName += string(msg.Runes)
		} else if m.create.currentField == fieldBase {
			m.create.baseBranch += string(msg.Runes)
		}
		return m, nil
	}

	return m, nil
}