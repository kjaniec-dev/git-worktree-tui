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
	fieldLocation
)

type createModel struct {
	branchName    string
	baseBranch    string
	createBranch  bool
	currentField  createField
	branches      []string
	baseIndex     int
	errMsg        string
	location      string // "inside" or "outside"
}

func generateWorktreePath(repoRoot, branch, location string) string {
	safeBranch := strings.ReplaceAll(branch, "/", "-")
	if location == "outside" {
		parent := filepath.Dir(repoRoot)
		repoName := filepath.Base(repoRoot)
		return filepath.Join(parent, fmt.Sprintf("%s-%s", repoName, safeBranch))
	}
	return filepath.Join(repoRoot, ".worktrees", safeBranch)
}

func (m Model) viewCreateModal() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Create Worktree"))
	b.WriteString("\n\n")

	branchLabel := fmt.Sprintf("Branch name: [%s]", m.create.branchName)
	baseLabel := fmt.Sprintf("Base: [%s]", m.create.baseBranch)
	checkboxLabel := fmt.Sprintf("☐ Create new branch from base: %v", m.create.createBranch)
	var insidePart, outsidePart string
	if m.create.location == "inside" {
		insidePart = activeFieldStyle.Render("inside")
		outsidePart = mutedStyle.Render("outside")
	} else {
		insidePart = mutedStyle.Render("inside")
		outsidePart = activeFieldStyle.Render("outside")
	}
	locationLabel := fmt.Sprintf("Location: <%s | %s>", insidePart, outsidePart)

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
	b.WriteString("\n")

	if m.create.currentField == fieldLocation {
		b.WriteString(selectedStyle.Render("→ " + locationLabel))
	} else {
		b.WriteString("  " + locationLabel)
	}
	b.WriteString("\n\n")

	path := generateWorktreePath(m.git.RepoRoot, m.create.branchName, m.create.location)
	b.WriteString(fmt.Sprintf("Path: %s (auto)\n\n", path))

	if m.create.errMsg != "" {
		b.WriteString(errorStyle.Render(m.create.errMsg))
		b.WriteString("\n\n")
	}

	b.WriteString(helpStyle.Render("[Tab] next field [↑/↓] select base [l] toggle location [Enter] create [Esc] cancel"))

	return b.String()
}

func (m Model) handleCreateKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyTab:
		m.create.currentField = (m.create.currentField + 1) % 4
		return m, nil
	case tea.KeyShiftTab:
		m.create.currentField = (m.create.currentField - 1 + 4) % 4
		return m, nil
	case tea.KeyEscape:
		m.mode = modeList
		return m, nil
	case tea.KeyEnter:
		if m.create.branchName == "" {
			m.create.errMsg = "Branch name is required"
			return m, nil
		}
		path := generateWorktreePath(m.git.RepoRoot, m.create.branchName, m.create.location)
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
		} else if m.create.currentField == fieldLocation {
			if msg.String() == "l" {
				if m.create.location == "inside" {
					m.create.location = "outside"
				} else {
					m.create.location = "inside"
				}
			}
		}
		return m, nil
	}

	return m, nil
}