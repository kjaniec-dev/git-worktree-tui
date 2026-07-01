package tui

import (
	"fmt"
	"strings"
	"time"

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

// initialBase selects the default base branch from the local branch list and
// returns its index so the create-form Base selector and the displayed base
// stay in sync. Prefers "main" over "master" when both exist; falls back to
// the first branch (index 0) or the literal "main" when the list is empty.
func initialBase(branches []string) (base string, index int) {
	base = "main"
	if len(branches) == 0 {
		return
	}
	base = branches[0]
	for i, b := range branches {
		if b == "main" {
			return b, i
		}
	}
	for i, b := range branches {
		if b == "master" {
			return b, i
		}
	}
	return branches[0], 0
}

type Model struct {
	git       *git.GitService
	worktrees []model.Worktree
	selected  int
	mode      appMode
	errMsg    string
	width     int
	height    int
	create    createModel
	cleanup   cleanupModel
}

func NewModel(gitService *git.GitService) Model {
	branches, _ := gitService.ListBranches()
	baseBranch, baseIndex := initialBase(branches)

	return Model{
		git:      gitService,
		selected: 0,
		mode:     modeList,
		create: createModel{
			branches:     branches,
			baseBranch:   baseBranch,
			baseIndex:    baseIndex,
			createBranch: true,
			location:     "inside",
		},
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadWorktrees
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.mode {
		case modeDelete:
			return m.handleDeleteKeyPress(msg)
		case modeCreate:
			return m.handleCreateKeyPress(msg)
		case modeCleanup:
			return m.handleCleanupKeyPress(msg)
		default:
			return m.handleKeyPress(msg)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case worktreesLoadedMsg:
		m.worktrees = msg.worktrees
		m.errMsg = ""
		if m.selected >= len(m.worktrees) {
			m.selected = len(m.worktrees) - 1
		}
		if m.selected < 0 {
			m.selected = 0
		}
		return m, nil
	case errMsg:
		m.errMsg = string(msg)
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if m.mode == modeDelete {
		return m.viewDeleteModal()
	}
	if m.mode == modeCreate {
		return m.viewCreateModal()
	}
	if m.mode == modeCleanup {
		return m.viewCleanupModal()
	}

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
		} else if wt.IsMain {
			status = "★"
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
		
		commitDisplay := wt.Commit
		if len(commitDisplay) > 7 {
			commitDisplay = commitDisplay[:7]
		}
		b.WriteString(fmt.Sprintf("    %s • %s", commitDisplay, wt.Path))
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

	// Load status concurrently
	type result struct {
		index  int
		status *model.WorktreeStatus
		err    error
	}

	resultChan := make(chan result, len(worktrees))

	for i, wt := range worktrees {
		go func(idx int, path string) {
			status, err := m.git.GetWorktreeStatus(path)
			resultChan <- result{index: idx, status: status, err: err}
		}(i, wt.Path)
	}

	// Collect results with timeout
	for i := 0; i < len(worktrees); i++ {
		select {
		case res := <-resultChan:
			if res.err == nil {
				worktrees[res.index].Status = res.status
			}
			// If error, status remains nil (shows as "?")
		case <-time.After(5 * time.Second):
			// Timeout - remaining statuses will be nil
			return worktreesLoadedMsg{worktrees: worktrees}
		}
	}

	return worktreesLoadedMsg{worktrees: worktrees}
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyRunes:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "j":
			if m.selected < len(m.worktrees)-1 {
				m.selected++
			}
			return m, nil
		case "k":
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
		case "r":
			return m, m.loadWorktrees
		case "d":
			if len(m.worktrees) > 0 && m.selected < len(m.worktrees) {
				wt := m.worktrees[m.selected]
				if wt.IsMain {
					m.errMsg = "Cannot delete main worktree"
					return m, nil
				}
				m.errMsg = ""
				m.mode = modeDelete
			}
			return m, nil
		case "a":
			m.mode = modeCreate
			return m, nil
		case "c":
			m.findStaleWorktrees()
			m.cleanup.currentIndex = 0
			m.mode = modeCleanup
			return m, nil
		}
	case tea.KeyDown:
		if m.selected < len(m.worktrees)-1 {
			m.selected++
		}
		return m, nil
	case tea.KeyUp:
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
	case tea.KeyEnter:
		if len(m.worktrees) > 0 && m.selected < len(m.worktrees) {
			path := m.worktrees[m.selected].Path
			m.errMsg = fmt.Sprintf("Path: %s", path)
		}
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}