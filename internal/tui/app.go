package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
)

type appMode int

const (
	modeList appMode = iota
	modeCreate
	modeDelete
	modeCleanup
)

type Model struct {
	git       *git.GitService
	worktrees []model.Worktree
	selected  int
	mode      appMode
	errMsg    string
	width     int
	height    int
}

func NewModel(gitService *git.GitService) Model {
	return Model{
		git:      gitService,
		selected: 0,
		mode:     modeList,
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadWorktrees
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case worktreesLoadedMsg:
		m.worktrees = msg.worktrees
		m.errMsg = ""
		return m, nil
	case errMsg:
		m.errMsg = string(msg)
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if len(m.worktrees) == 0 {
		return "No worktrees found. Press 'q' to quit."
	}

	var b strings.Builder
	
	// Title
	b.WriteString(titleStyle.Render("git-worktree-tui"))
	b.WriteString("\n\n")

	// Worktree list
	for i, wt := range m.worktrees {
		prefix := "  "
		if i == m.selected {
			prefix = "→ "
		}

		status := "●"
		if wt.IsLocked {
			status = "🔒"
		} else if wt.Status != nil && wt.Status.IsDirty {
			status = "●"
		}

		line := fmt.Sprintf("%s%s %s", prefix, status, wt.Branch)
		if wt.Detached {
			line = fmt.Sprintf("%s%s (detached)", prefix, status)
		}

		if i == m.selected {
			line = selectedStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("    %s • %s", wt.Commit[:7], wt.Path))
		b.WriteString("\n\n")
	}

	// Help
	b.WriteString(helpStyle.Render("[a]dd [d]elete [c]leanup [r]efresh [q]uit"))

	// Error message
	if m.errMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render(m.errMsg))
	}

	return b.String()
}

// Messages
type worktreesLoadedMsg struct {
	worktrees []model.Worktree
}

type errMsg string

// Commands
func (m Model) loadWorktrees() tea.Msg {
	worktrees, err := m.git.ListWorktrees()
	if err != nil {
		return errMsg(err.Error())
	}
	return worktreesLoadedMsg{worktrees: worktrees}
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
		return m, tea.Quit
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if m.selected < len(m.worktrees)-1 {
			m.selected++
		}
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		return m, m.loadWorktrees
	}
	return m, nil
}