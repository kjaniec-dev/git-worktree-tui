package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aymanbagabas/go-osc52/v2"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
)

type appMode int

const (
	modeList appMode = iota
	modeCreate
	modeDelete
	modeCleanup
	modeHelp
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
	git          *git.GitService
	worktrees    []model.Worktree
	selected     int
	mode         appMode
	errMsg       string
	infoMsg      string // populated by Enter (Task 3); rendered with infoStyle
	staleCount   int
	stalePaths   []string
	cwd          string // captured at startup; drives the (here) marker
	width        int
	height       int
	create       createModel
	cleanup      cleanupModel
	filterMode   bool
	filterText   string
	delete       deleteModel
	busy         bool
	busyLabel    string
	spinner      spinner.Model
	selectedPath string // set by 'g'; read by cmd/root.go after the program exits
}

// SelectedPath returns the worktree path chosen via 'g' (select-and-quit),
// or "" if the user quit normally. cmd/root.go prints this to stdout after
// the Bubble Tea program exits so a shell wrapper can `cd` into it — see
// the "Shell integration" section of the README.
func (m Model) SelectedPath() string {
	return m.selectedPath
}

func NewModel(gitService *git.GitService, cwd string) Model {
	branches, _ := gitService.ListBranches()
	baseBranch, baseIndex := initialBase(branches)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = activeFieldStyle

	return Model{
		git:      gitService,
		selected: 0,
		mode:     modeList,
		cwd:      cwd,
		spinner:  sp,
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
		if msg.Type == tea.KeyCtrlC {
			// Universal escape hatch: quit immediately regardless of mode
			// or an in-flight busy operation. Without this, Ctrl+C was
			// swallowed while busy (stuck up to the full command timeout)
			// and inside every modal (create/delete/cleanup/help).
			return m, tea.Quit
		}
		if m.busy {
			// Ignore other input while an async git operation is in flight
			// to avoid double-submission; GitService enforces its own
			// command timeout, so this is never an indefinite lock.
			return m, nil
		}
		switch m.mode {
		case modeDelete:
			return m.handleDeleteKeyPress(msg)
		case modeCreate:
			return m.handleCreateKeyPress(msg)
		case modeCleanup:
			return m.handleCleanupKeyPress(msg)
		case modeHelp:
			return m.handleHelpKeyPress(msg)
		default:
			return m.handleKeyPress(msg)
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tickMsg:
		if !shouldAutoReload(m.mode) {
			// Don't reload out from under an open modal: the cleanup
			// modal caches indices into m.worktrees that would dangle
			// (and panic on render) if the list shrinks, and the delete
			// confirmation would silently start referring to a different
			// worktree than the one on screen. Still reschedule so
			// refresh resumes once the user returns to the list.
			return m, autoRefresh()
		}
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
		if m.mode == modeCleanup {
			// Defense in depth: even though the tick that drives most
			// refreshes is paused during modeCleanup (see shouldAutoReload),
			// a refresh already in flight when 'c' was pressed can still
			// land here. Drop any cached indices that no longer fit rather
			// than let viewCleanupModal index out of range.
			m.cleanup.staleWorktrees, m.cleanup.selected, m.cleanup.reasons =
				clampCleanupIndices(m.cleanup.staleWorktrees, m.cleanup.selected, m.cleanup.reasons, len(m.worktrees))
			if m.cleanup.currentIndex >= len(m.cleanup.staleWorktrees) {
				m.cleanup.currentIndex = 0
			}
		}
		if m.filterText != "" && m.selected < len(m.worktrees) {
			needle := strings.ToLower(m.filterText)
			if !strings.Contains(strings.ToLower(m.worktrees[m.selected].Branch), needle) {
				m.selected = advanceSelected(m.selected, +1, m.worktrees, m.filterText)
			}
		}
		// Recompute stale count for the footer hint + immediate-remove fast path.
		if branches, err := m.git.ListBranches(); err == nil {
			branchSet := make(map[string]bool)
			for _, b := range branches {
				branchSet[b] = true
			}
			base, _ := initialBase(branches)
			mergedSet := m.mergedBranchSet(base)
			staleIdxs := classifyStale(m.worktrees, branchSet, mergedSet)
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
	case spinner.TickMsg:
		if !m.busy {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	case createResultMsg:
		return m.handleCreateResult(msg)
	case deleteResultMsg:
		return m.handleDeleteResult(msg)
	case cleanupResultMsg:
		return m.handleCleanupResult(msg)
	case lockResultMsg:
		return m.handleLockResult(msg)
	}
	return m, nil
}

// startBusy marks the model busy with the given label and returns a command
// batching the spinner's first tick with the async operation itself.
func startBusy(m *Model, label string, op tea.Cmd) tea.Cmd {
	m.busy = true
	m.busyLabel = label
	return tea.Batch(m.spinner.Tick, op)
}

func (m Model) viewBusy() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("git-worktree-tui"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("%s %s", m.spinner.View(), m.busyLabel))
	b.WriteString("\n")
	return b.String()
}

func (m Model) View() string {
	if m.busy {
		return m.viewBusy()
	}
	if m.mode == modeDelete {
		return m.viewDeleteModal()
	}
	if m.mode == modeCreate {
		return m.viewCreateModal()
	}
	if m.mode == modeCleanup {
		return m.viewCleanupModal()
	}
	if m.mode == modeHelp {
		return m.viewHelpModal()
	}

	if len(m.worktrees) == 0 {
		return "No worktrees found. Press 'q' to quit."
	}

	var b strings.Builder

	// Title
	b.WriteString(titleStyle.Render("git-worktree-tui"))
	b.WriteString("\n\n")

	// Worktree list — windowed/scrollable so long lists don't overflow the
	// terminal. rowIdxs holds the indices (into m.worktrees) that pass the
	// active filter, in display order.
	rowIdxs := visibleRowIndices(m.worktrees, m.filterText)
	selectedPos := positionOf(rowIdxs, m.selected)
	maxVisible := maxVisibleRows(m.height, listOverheadLines(m))
	scrollOffset := computeScrollOffset(selectedPos, len(rowIdxs), maxVisible)

	windowEnd := len(rowIdxs)
	if maxVisible > 0 && scrollOffset+maxVisible < windowEnd {
		windowEnd = scrollOffset + maxVisible
	}
	visibleWindow := rowIdxs[scrollOffset:windowEnd]

	if scrollOffset > 0 {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("  ↑ %d more above", scrollOffset)))
		b.WriteString("\n")
	}

	for _, i := range visibleWindow {
		wt := m.worktrees[i]
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

	if remaining := len(rowIdxs) - windowEnd; remaining > 0 {
		b.WriteString(mutedStyle.Render(fmt.Sprintf("  ↓ %d more below", remaining)))
		b.WriteString("\n")
	}

	// Help
	helpText := "[a]dd [d]elete [l]ock [c]leanup [r]efresh [g]o [o]pen (cd) [/]filter [?]help [q]uit"
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

	if m.filterMode {
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("/" + m.filterText + "▏"))
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

// visibleRowIndices returns the indices into wts that pass the filter
// (case-insensitive substring on Branch), in original order. Empty filter
// returns every index.
func visibleRowIndices(wts []model.Worktree, filterText string) []int {
	idxs := make([]int, 0, len(wts))
	needle := strings.ToLower(filterText)
	for i, wt := range wts {
		if filterText == "" || strings.Contains(strings.ToLower(wt.Branch), needle) {
			idxs = append(idxs, i)
		}
	}
	return idxs
}

// positionOf returns the position of target within idxs, or -1 if absent.
func positionOf(idxs []int, target int) int {
	for pos, idx := range idxs {
		if idx == target {
			return pos
		}
	}
	return -1
}

// listOverheadLines estimates how many rendered lines in View() are NOT part
// of the scrollable worktree list (title, footer help line, and any
// conditionally-rendered info/error/filter lines), so maxVisibleRows can
// budget the remaining terminal height for worktree rows.
func listOverheadLines(m Model) int {
	overhead := 4 // title (1 line + 1 blank) + help line (has MarginTop(1))
	if m.infoMsg != "" {
		overhead++
	}
	if m.errMsg != "" {
		overhead += 2
	}
	if m.filterMode {
		overhead++
	}
	return overhead
}

// maxVisibleRows returns how many 3-line worktree rows fit within height,
// after subtracting overheadLines. Returns 0 (meaning "unlimited", i.e. no
// scrolling) when height is unknown (<=0), which happens before the first
// tea.WindowSizeMsg — this keeps output stable for callers/tests that never
// send one.
func maxVisibleRows(height, overheadLines int) int {
	if height <= 0 {
		return 0
	}
	rows := (height - overheadLines) / 3
	if rows < 1 {
		rows = 1
	}
	return rows
}

// computeScrollOffset returns the first visible-row index to render such
// that selectedPos stays inside the [offset, offset+maxVisible) window,
// centering the selection when the list is longer than the window.
// maxVisible <= 0 or a list that already fits disables scrolling (offset 0).
func computeScrollOffset(selectedPos, visibleCount, maxVisible int) int {
	if maxVisible <= 0 || visibleCount <= maxVisible {
		return 0
	}
	if selectedPos < 0 {
		selectedPos = 0
	}
	offset := selectedPos - maxVisible/2
	if offset < 0 {
		offset = 0
	}
	if maxOffset := visibleCount - maxVisible; offset > maxOffset {
		offset = maxOffset
	}
	return offset
}

// shouldAutoReload reports whether the periodic auto-refresh tick should
// actually reload the worktree list for the given mode. It's paused for
// modals that depend on m.worktrees staying stable while they're open —
// see the tickMsg handler in Update for why.
func shouldAutoReload(mode appMode) bool {
	switch mode {
	case modeDelete, modeCleanup:
		return false
	default:
		return true
	}
}

// clampCleanupIndices drops entries from the cleanup modal's parallel
// slices whose worktree index no longer exists in a worktrees slice of the
// given length, keeping idxs/selected/reasons in sync with each other.
func clampCleanupIndices(idxs []int, selected []bool, reasons []string, worktreeCount int) ([]int, []bool, []string) {
	outIdx := make([]int, 0, len(idxs))
	outSel := make([]bool, 0, len(idxs))
	outReason := make([]string, 0, len(idxs))
	for i, idx := range idxs {
		if idx < worktreeCount {
			outIdx = append(outIdx, idx)
			outSel = append(outSel, selected[i])
			outReason = append(outReason, reasons[i])
		}
	}
	return outIdx, outSel, outReason
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

// visibleWorktrees returns the subset of worktrees whose Branch contains filter
// (case-insensitive substring). Empty filter returns all.
func visibleWorktrees(wts []model.Worktree, filter string) []model.Worktree {
	if filter == "" {
		return wts
	}
	needle := strings.ToLower(filter)
	var out []model.Worktree
	for _, wt := range wts {
		if strings.Contains(strings.ToLower(wt.Branch), needle) {
			out = append(out, wt)
		}
	}
	return out
}

// advanceSelected returns the next visible index starting from `sel` moving by
// `dir` (+1 forward, -1 back), skipping worktrees filtered out by filterText.
// Returns `sel` unchanged if no visible match exists in the requested direction.
func advanceSelected(sel, dir int, wts []model.Worktree, filterText string) int {
	if len(wts) == 0 {
		return 0
	}
	needle := strings.ToLower(filterText)
	match := func(i int) bool {
		return filterText == "" || strings.Contains(strings.ToLower(wts[i].Branch), needle)
	}
	for step := 1; ; step++ {
		next := sel + dir*step
		if next < 0 || next >= len(wts) {
			return sel
		}
		if match(next) {
			return next
		}
	}
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filterMode {
		switch msg.Type {
		case tea.KeyRunes:
			m.filterText += string(msg.Runes)
		case tea.KeyBackspace:
			if len(m.filterText) > 0 {
				m.filterText = m.filterText[:len(m.filterText)-1]
			}
		case tea.KeyEscape:
			m.filterMode = false
			m.filterText = ""
			m.selected = 0
		case tea.KeyEnter:
			m.filterMode = false
		}
		return m, nil
	}

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
			m.selected = advanceSelected(m.selected, +1, m.worktrees, m.filterText)
			return m, nil
		case "k":
			if len(m.worktrees) == 0 {
				return m, nil
			}
			m.selected = advanceSelected(m.selected, -1, m.worktrees, m.filterText)
			return m, nil
		case "/":
			m.filterMode = true
			m.filterText = ""
			return m, nil
		case "o":
			if len(m.worktrees) > 0 && m.selected < len(m.worktrees) {
				path := m.worktrees[m.selected].Path
				if tryCopyClipboard("cd " + path) {
					m.infoMsg = fmt.Sprintf("Copied: cd %s", path)
				} else {
					m.infoMsg = fmt.Sprintf("cd %s", path)
				}
				m.errMsg = ""
			}
			return m, nil
		case "r":
			return m, m.loadWorktrees
		case "g":
			if len(m.worktrees) > 0 && m.selected < len(m.worktrees) {
				m.selectedPath = m.worktrees[m.selected].Path
				return m, tea.Quit
			}
			return m, nil
		case "?":
			m.mode = modeHelp
			return m, nil
		case "l":
			if len(m.worktrees) > 0 && m.selected < len(m.worktrees) {
				wt := m.worktrees[m.selected]
				if wt.IsMain {
					m.errMsg = "Cannot lock the main worktree"
					return m, nil
				}
				m.errMsg = ""
				label := "Locking worktree..."
				if wt.IsLocked {
					label = "Unlocking worktree..."
				}
				cmd := startBusy(&m, label, lockWorktreeCmd(m.git, wt.Path, wt.Branch, wt.IsLocked))
				return m, cmd
			}
			return m, nil
		case "d":
			if len(m.worktrees) > 0 && m.selected < len(m.worktrees) {
				wt := m.worktrees[m.selected]
				if wt.IsMain {
					m.errMsg = "Cannot delete main worktree"
					return m, nil
				}
				m.errMsg = ""
				m.delete = deleteModel{}
				m.mode = modeDelete
			}
			return m, nil
		case "a":
			// Refresh the branch list (a branch created since NewModel ran,
			// including via a previous 'a' session, must be selectable as a
			// base) and reset the form so stale input from a prior open
			// doesn't linger.
			branches, _ := m.git.ListBranches()
			baseBranch, baseIndex := initialBase(branches)
			m.create = createModel{
				branches:     branches,
				baseBranch:   baseBranch,
				baseIndex:    baseIndex,
				createBranch: true,
				location:     "inside",
			}
			m.mode = modeCreate
			return m, nil
		case "c":
			if m.staleCount == 1 && len(m.stalePaths) == 1 {
				path := m.stalePaths[0]
				branch := ""
				for _, wt := range m.worktrees {
					if wt.Path == path && !wt.Detached {
						branch = wt.Branch
						break
					}
				}
				m.errMsg = ""
				cmd := startBusy(&m, "Cleaning up 1 worktree...",
					cleanupWorktreesCmd(m.git, []string{path}, []string{branch}))
				return m, cmd
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
		m.selected = advanceSelected(m.selected, +1, m.worktrees, m.filterText)
		return m, nil
	case tea.KeyUp:
		if len(m.worktrees) == 0 {
			return m, nil
		}
		m.selected = advanceSelected(m.selected, -1, m.worktrees, m.filterText)
		return m, nil
	case tea.KeyEscape:
		if m.filterText != "" {
			m.filterText = ""
			m.selected = 0
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
	}
	return m, nil
}
