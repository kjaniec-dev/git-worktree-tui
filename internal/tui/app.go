package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aymanbagabas/go-osc52/v2"
	"github.com/charmbracelet/lipgloss"
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
	infoMsg   string // populated by Enter (Task 3); rendered with infoStyle
	staleCount int
	stalePaths []string
	cwd       string  // captured at startup; drives the (here) marker
	width     int
	height    int
	create    createModel
	cleanup   cleanupModel
}

func NewModel(gitService *git.GitService, cwd string) Model {
	branches, _ := gitService.ListBranches()
	baseBranch, baseIndex := initialBase(branches)

	return Model{
		git:      gitService,
		selected: 0,
		mode:     modeList,
		cwd:      cwd,
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
	return tea.Batch(m.loadWorktrees, autoRefresh())
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
	case tickMsg:
		return m, tea.Batch(m.loadWorktrees, autoRefresh())
	case worktreesLoadedMsg:
		m.worktrees = msg.worktrees
		m.errMsg = ""
		if m.selected >= len(m.worktrees) {
			m.selected = len(m.worktrees) - 1
		}
		if m.selected < 0 {
			m.selected = 0
		}
		// Recompute stale count for the footer hint + immediate-remove fast path.
		if branches, err := m.git.ListBranches(); err == nil {
			branchSet := make(map[string]bool)
			for _, b := range branches {
				branchSet[b] = true
			}
			staleIdxs := classifyStale(m.worktrees, branchSet)
			m.staleCount = len(staleIdxs)
			m.stalePaths = make([]string, 0, len(staleIdxs))
			for _, idx := range staleIdxs {
				m.stalePaths = append(m.stalePaths, m.worktrees[idx].Path)
			}
		} else {
			m.staleCount = 0
			m.stalePaths = nil
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

		var glyph string
		var glyphStyle lipgloss.Style
		switch {
		case wt.IsLocked:
			glyph = "🔒"
			glyphStyle = lockedStyle
		case wt.IsMain:
			glyph = "★"
			glyphStyle = mainStyle
		case wt.Status != nil && wt.Status.IsDirty:
			glyph = "●"
			glyphStyle = dirtyStyle
		case wt.Status != nil && !wt.Status.IsDirty:
			glyph = "○"
			glyphStyle = cleanStyle
		default:
			glyph = "?"
			glyphStyle = mutedStyle
		}

		branchPart := wt.Branch
		if wt.Detached {
			branchPart = "(detached)"
		}

		var hereMarker string
		if rel, err := filepath.Rel(wt.Path, m.cwd); err == nil && !strings.HasPrefix(rel, "..") {
			hereMarker = mutedStyle.Render(" (here)")
		}

		renderedGlyph := glyphStyle.Render(glyph)
		line := fmt.Sprintf("%s%s %s", prefix, renderedGlyph, branchPart)
		if hereMarker != "" {
			line += hereMarker
		}
		if wt.Status != nil && (wt.Status.Ahead > 0 || wt.Status.Behind > 0) {
			var ab strings.Builder
			if wt.Status.Ahead > 0 {
				ab.WriteString(fmt.Sprintf(" ↑%d", wt.Status.Ahead))
			}
			if wt.Status.Behind > 0 {
				ab.WriteString(fmt.Sprintf(" ↓%d", wt.Status.Behind))
			}
			line += mutedStyle.Render(ab.String())
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
		b.WriteString(fmt.Sprintf("    %s • %s", commitDisplay, truncatePath(wt.Path)))
		b.WriteString("\n\n")
	}

	// Help
	helpText := "[a]dd [d]elete [c]leanup [r]efresh [q]uit"
	if m.staleCount > 0 {
		helpText = staleHintText(m.staleCount) + helpText
	}
	b.WriteString(helpStyle.Render(helpText))

	if m.infoMsg != "" {
		b.WriteString("\n")
		b.WriteString(infoStyle.Render(m.infoMsg))
	}

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

type tickMsg time.Time

type errMsg string

// autoRefresh schedules a loadWorktrees refresh via tea.Tick every 10 seconds.
// The cycle continues: each tick fires a tickMsg, which schedules the next refresh+tick.
func autoRefresh() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

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

func tryCopyClipboard(path string) bool {
	if _, err := fmt.Fprint(os.Stderr, osc52.New(path)); err != nil {
		return false
	}
	return true
}

// truncatePath shortens paths longer than 40 chars to "..." + last two path segments.
// Paths ≤ 40 chars or paths with fewer than 3 segments are returned as-is.
func truncatePath(path string) string {
	if len(path) <= 40 || path == "" {
		return path
	}
	sep := string(filepath.Separator)
	parts := strings.Split(path, sep)
	if len(parts) < 3 {
		return path
	}
	return "..." + sep + parts[len(parts)-2] + sep + parts[len(parts)-1]
}

func staleHintText(count int) string {
	switch {
	case count <= 0:
		return "  "
	case count == 1:
		return "  [c]leanup 1 stale (Enter to remove)  "
	default:
		return fmt.Sprintf("  [c]leanup %d stale  ", count)
	}
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.infoMsg = ""
	switch msg.Type {
	case tea.KeyRunes:
		switch msg.String() {
		case "q":
			return m, tea.Quit
		case "j":
			if len(m.worktrees) == 0 {
				return m, nil
			}
			if m.selected < len(m.worktrees)-1 {
				m.selected++
			}
			return m, nil
		case "k":
			if len(m.worktrees) == 0 {
				return m, nil
			}
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
			if m.staleCount == 1 && len(m.stalePaths) == 1 {
				path := m.stalePaths[0]
				if err := m.git.RemoveWorktree(path, false); err != nil {
					m.errMsg = fmt.Sprintf("Failed to remove %s: %v", path, err)
				} else {
					m.infoMsg = fmt.Sprintf("Removed: %s", path)
				}
				return m, m.loadWorktrees
			}
			m.findStaleWorktrees()
			m.cleanup.currentIndex = 0
			m.mode = modeCleanup
			return m, nil
		}
	case tea.KeyDown:
		if len(m.worktrees) == 0 {
			return m, nil
		}
		if m.selected < len(m.worktrees)-1 {
			m.selected++
		}
		return m, nil
	case tea.KeyUp:
		if len(m.worktrees) == 0 {
			return m, nil
		}
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
	case tea.KeyEnter:
		if len(m.worktrees) > 0 && m.selected < len(m.worktrees) {
			path := m.worktrees[m.selected].Path
			copied := tryCopyClipboard(path)
			if copied {
				m.infoMsg = fmt.Sprintf("Copied: %s", path)
			} else {
				m.infoMsg = fmt.Sprintf("Path: %s", path)
			}
			m.errMsg = ""
		}
		return m, nil
	case tea.KeyCtrlC:
		return m, tea.Quit
	}
	return m, nil
}
