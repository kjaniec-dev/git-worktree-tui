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
	parent := filepath.Dir(repoRoot)
	repoName := filepath.Base(repoRoot)
	// Replace slashes with dashes
	safeBranch := strings.ReplaceAll(branch, "/", "-")
	return filepath.Join(parent, fmt.Sprintf("%s-%s", repoName, safeBranch))
}

func (m Model) viewCreateModal() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Create Worktree"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Branch name: [%s]\n", m.create.branchName))
	b.WriteString(fmt.Sprintf("Base: [%s]\n", m.create.baseBranch))
	b.WriteString(fmt.Sprintf("☐ Create new branch from base: %v\n\n", m.create.createBranch))

	path := generateWorktreePath(m.git.RepoRoot, m.create.branchName)
	b.WriteString(fmt.Sprintf("Path: %s (auto)\n\n", path))

	if m.create.errMsg != "" {
		b.WriteString(errorStyle.Render(m.create.errMsg))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("[Tab] next field [Enter] create [Esc] cancel"))

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
		return m, m.createWorktree
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
		}
		return m, nil
	case tea.KeyRunes:
		if m.create.currentField == fieldBranch {
			m.create.branchName += string(msg.Runes)
		}
		return m, nil
	}

	return m, nil
}

func (m Model) createWorktree() tea.Msg {
	if m.create.branchName == "" {
		m.create.errMsg = "Branch name is required"
		return nil
	}

	path := generateWorktreePath(m.git.RepoRoot, m.create.branchName)
	err := m.git.AddWorktree(path, m.create.branchName, m.create.baseBranch, m.create.createBranch)
	if err != nil {
		m.create.errMsg = err.Error()
		return nil
	}

	m.mode = modeList
	return worktreesLoadedMsg{worktrees: nil} // Trigger reload
}