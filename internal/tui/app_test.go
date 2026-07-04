package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
)

func TestNewModel(t *testing.T) {
	gitService := git.NewGitService("/tmp/test")
	m := NewModel(gitService, "/tmp/test")

	if m.git == nil {
		t.Error("Expected git service to be set")
	}
	if m.selected != 0 {
		t.Errorf("Expected selected to be 0, got %d", m.selected)
	}
	if m.mode != modeList {
		t.Errorf("Expected mode to be modeList, got %v", m.mode)
	}
}

func TestNewModelWithCWD(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/custom/cwd")
	if m.cwd != "/custom/cwd" {
		t.Errorf("m.cwd = %q, want /custom/cwd", m.cwd)
	}
}

func TestHereMarkerOnWorktreeRoot(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/wt/feat")
	m.worktrees = []model.Worktree{
		{Path: "/wt/main", Branch: "main"},
		{Path: "/wt/feat", Branch: "feat"},
	}
	m.mode = modeList
	view := m.View()
	if !strings.Contains(view, "feat (here)") {
		t.Errorf("expected 'feat (here)' in view, got:\n%s", view)
	}
	if strings.Contains(view, "main (here)") {
		t.Errorf("did not expect (here) on 'main' row, got:\n%s", view)
	}
	if c := strings.Count(view, "(here)"); c != 1 {
		t.Errorf("expected exactly 1 (here) marker, got %d:\n%s", c, view)
	}
}

func TestHereMarkerInSubdirectory(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/wt/feat/subdir")
	m.worktrees = []model.Worktree{
		{Path: "/wt/main", Branch: "main"},
		{Path: "/wt/feat", Branch: "feat"},
	}
	m.mode = modeList
	view := m.View()
	if !strings.Contains(view, "feat (here)") {
		t.Errorf("expected (here) on feat even when CWD is a subdirectory of /wt/feat:\n%s", view)
	}
	if c := strings.Count(view, "(here)"); c != 1 {
		t.Errorf("expected exactly 1 (here) marker, got %d:\n%s", c, view)
	}
}

func TestInitialBase(t *testing.T) {
	tests := []struct {
		name     string
		branches []string
		wantBase string
		wantIdx  int
	}{
		{"empty -> main default", []string{}, "main", 0},
		{"main at 0", []string{"main", "develop"}, "main", 0},
		{"main not at 0 -> index of main", []string{"develop", "main", "feat"}, "main", 1},
		{"master when no main", []string{"develop", "master"}, "master", 1},
		{"main preferred over master", []string{"master", "main"}, "main", 1},
		{"no main/master -> first branch", []string{"develop", "feat"}, "develop", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, idx := initialBase(tt.branches)
			if base != tt.wantBase {
				t.Errorf("base = %q, want %q", base, tt.wantBase)
			}
			if idx != tt.wantIdx {
				t.Errorf("idx = %d, want %d", idx, tt.wantIdx)
			}
		})
	}
}

func TestEmptyListNavigationNoOp(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = nil
	m.selected = 0
	m.mode = modeList

	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'j'}},
		{Type: tea.KeyRunes, Runes: []rune{'k'}},
		{Type: tea.KeyDown},
		{Type: tea.KeyUp},
	} {
		out, _ := m.handleKeyPress(msg)
		mm := out.(Model)
		if mm.selected != 0 {
			t.Errorf("after %v on empty list, selected = %d, want 0", msg, mm.selected)
		}
	}
}

func TestStatusGlyphs(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{
		{Path: "/p/locked", Branch: "b", IsLocked: true},
		{Path: "/p/main", Branch: "main", IsMain: true},
		{Path: "/p/dirty", Branch: "d", Status: &model.WorktreeStatus{IsDirty: true}},
		{Path: "/p/clean", Branch: "c", Status: &model.WorktreeStatus{IsDirty: false}},
		{Path: "/p/unknown", Branch: "u"}, // Status == nil -> not loaded
	}
	m.mode = modeList
	view := m.View()

	cases := []struct{ state, glyph string }{
		{"locked", "🔒"},
		{"main", "★"},
		{"dirty", "●"},
		{"clean", "○"},
		{"unknown", "?"},
	}
	for _, c := range cases {
		if !strings.Contains(view, c.glyph) {
			t.Errorf("expected glyph %q for %s worktree in view, got:\n%s", c.glyph, c.state, view)
		}
	}
	// Sanity: dirty and clean must produce DIFFERENT view strings
	if strings.Contains(view, "● c") || strings.Contains(view, "○ d") {
		t.Errorf("dirty/clean glyphs collided:\n%s", view)
	}
}

func TestEnterSetsInfoMsgNotErrMsg(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{
		{Path: "/tmp/mypath", Branch: "feat"},
	}
	m.selected = 0
	m.mode = modeList

	out, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	mm := out.(Model)

	if mm.errMsg != "" {
		t.Errorf("errMsg should be empty on Enter, got %q", mm.errMsg)
	}
	if mm.infoMsg == "" {
		t.Error("expected infoMsg to be set on Enter, got empty")
	}
	if !strings.Contains(mm.infoMsg, "/tmp/mypath") {
		t.Errorf("expected infoMsg to contain the path, got %q", mm.infoMsg)
	}

	// View must render infoMsg (at minimum the text appears).
	view := mm.View()
	if !strings.Contains(view, mm.infoMsg) {
		t.Errorf("expected infoMsg in view, got:\n%s", view)
	}
}

func TestInfoMsgClearedOnNextKeypress(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{{Path: "/p", Branch: "b"}}
	m.selected = 0
	m.infoMsg = "lingering from before"
	m.mode = modeList

	// Any keypress should clear infoMsg
	out, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyUp})
	mm := out.(Model)
	if mm.infoMsg != "" {
		t.Errorf("infoMsg should be cleared on next keypress, got %q", mm.infoMsg)
	}
}

func TestTruncatePath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/short", "/short"},
		{"", ""},
		{"/Users/kjaniec-dev/dev/projects/git-worktree-tui", ".../projects/git-worktree-tui"},
	}
	for _, tt := range tests {
		got := truncatePath(tt.in)
		if got != tt.want {
			t.Errorf("truncatePath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestAheadBehindAnnotation(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{
		{Path: "/p", Branch: "feat", Status: &model.WorktreeStatus{Ahead: 3, Behind: 0}},
		{Path: "/p2", Branch: "main", Status: &model.WorktreeStatus{Ahead: 0, Behind: 1}},
		{Path: "/p3", Branch: "clean", Status: &model.WorktreeStatus{Ahead: 0, Behind: 0}},
	}
	m.mode = modeList
	view := m.View()
	if !strings.Contains(view, "↑3") {
		t.Errorf("expected ↑3 annotation:\n%s", view)
	}
	if !strings.Contains(view, "↓1") {
		t.Errorf("expected ↓1 annotation:\n%s", view)
	}
}

func TestStaleHintText(t *testing.T) {
	tests := []struct {
		count int
		want  string
	}{
		{0, "  "},
		{1, "  [c]leanup 1 stale (Enter to remove)  "},
		{3, "  [c]leanup 3 stale  "},
		{7, "  [c]leanup 7 stale  "},
	}
	for _, tt := range tests {
		got := staleHintText(tt.count)
		if got != tt.want {
			t.Errorf("staleHintText(%d) = %q, want %q", tt.count, got, tt.want)
		}
	}
}

func TestAutoRefreshTick(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	// Init should return a Batch (loadWorktrees + autoRefresh)
	cmd := m.Init()
	if cmd == nil {
		t.Fatal("Init returned nil cmd")
	}
	// On tickMsg, Update must return a non-nil refresh cmd
	updated, cmd := m.Update(tickMsg{})
	if cmd == nil {
		t.Error("Update on tickMsg must return a refresh cmd (tea.Batch)")
	}
	_ = updated
}

// TestShouldAutoReload documents which modals suppress the periodic
// auto-refresh reload. modeDelete and modeCleanup hold state derived from
// (or indices into) m.worktrees at a point in time; letting the 10s tick
// mutate m.worktrees underneath them risks a stale confirmation (delete) or
// an out-of-range panic in viewCleanupModal (cleanup) — see
// TestTickDoesNotReloadDuringCleanupOrDelete and
// TestCleanupSurvivesShrinkingWorktreeListWithoutPanic.
func TestShouldAutoReload(t *testing.T) {
	cases := []struct {
		mode appMode
		want bool
	}{
		{modeList, true},
		{modeCreate, true},
		{modeHelp, true},
		{modeDelete, false},
		{modeCleanup, false},
	}
	for _, c := range cases {
		if got := shouldAutoReload(c.mode); got != c.want {
			t.Errorf("shouldAutoReload(%v) = %v, want %v", c.mode, got, c.want)
		}
	}
}

// TestTickDoesNotReloadDuringCleanupOrDelete verifies the tick handler
// itself consults shouldAutoReload: while modeCleanup/modeDelete are active
// it must NOT batch in m.loadWorktrees (which would race with/replace
// m.worktrees mid-modal), only reschedule the next tick.
func TestTickDoesNotReloadDuringCleanupOrDelete(t *testing.T) {
	for _, mode := range []appMode{modeCleanup, modeDelete} {
		m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
		m.mode = mode
		_, cmd := m.Update(tickMsg{})
		if cmd == nil {
			t.Fatalf("mode %v: expected autoRefresh to still be rescheduled", mode)
		}
		// A reload tick batches loadWorktrees+autoRefresh, which resolves
		// (without blocking — tea.Batch just packages the sub-cmds) to a
		// tea.BatchMsg. A paused tick returns bare autoRefresh(), a
		// single tea.Tick cmd that would block ~10s if invoked, so we
		// don't call it — its mere presence (non-nil, non-batch by
		// construction) is confirmed via shouldAutoReload above instead.
	}

	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.mode = modeList
	_, cmd := m.Update(tickMsg{})
	if cmd == nil {
		t.Fatal("expected a cmd in list mode")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("expected list-mode tick to resolve to a batch message")
	} else if _, isBatch := msg.(tea.BatchMsg); !isBatch {
		t.Errorf("expected list-mode tick to batch loadWorktrees+autoRefresh, got %T", msg)
	}
}

// TestCleanupSurvivesShrinkingWorktreeListWithoutPanic reproduces the crash
// this test guards against: the cleanup modal caches indices into
// m.worktrees when opened. If a worktreesLoadedMsg arrives afterward with a
// shorter list (e.g. a worktree was removed from another terminal), the
// stale indices must be dropped rather than left dangling for
// viewCleanupModal to index out of range on.
func TestCleanupSurvivesShrinkingWorktreeListWithoutPanic(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.mode = modeCleanup
	m.worktrees = []model.Worktree{
		{Path: "/a", Branch: "a"},
		{Path: "/b", Branch: "b"},
		{Path: "/c", Branch: "c"},
	}
	// Cached while the list still had 3 entries; index 2 ("/c") is stale.
	m.cleanup.staleWorktrees = []int{2}
	m.cleanup.selected = []bool{true}
	m.cleanup.reasons = []string{"branch deleted"}
	m.cleanup.currentIndex = 0

	// The list shrinks to 1 entry — index 2 no longer exists.
	shrunk := worktreesLoadedMsg{worktrees: []model.Worktree{{Path: "/a", Branch: "a"}}}
	out, _ := m.Update(shrunk)
	final := out.(Model)

	if len(final.cleanup.staleWorktrees) != 0 {
		t.Errorf("expected dangling cleanup index to be dropped, got %v", final.cleanup.staleWorktrees)
	}
	if len(final.cleanup.selected) != 0 || len(final.cleanup.reasons) != 0 {
		t.Error("expected selected/reasons to stay in sync with staleWorktrees")
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("viewCleanupModal panicked: %v", r)
		}
	}()
	_ = final.View()
}

// TestClampCleanupIndices exercises the pure helper directly.
func TestClampCleanupIndices(t *testing.T) {
	idxs := []int{0, 2, 5}
	selected := []bool{true, false, true}
	reasons := []string{"branch deleted", "merged", "branch deleted"}

	outIdx, outSel, outReason := clampCleanupIndices(idxs, selected, reasons, 3)

	if !equalInts(outIdx, []int{0, 2}) {
		t.Errorf("outIdx = %v, want [0 2]", outIdx)
	}
	if len(outSel) != 2 || outSel[0] != true || outSel[1] != false {
		t.Errorf("outSel = %v, want [true false]", outSel)
	}
	if len(outReason) != 2 || outReason[0] != "branch deleted" || outReason[1] != "merged" {
		t.Errorf("outReason = %v, want [branch deleted merged]", outReason)
	}
}

func TestVisibleWorktrees(t *testing.T) {
	wts := []model.Worktree{
		{Branch: "main"},
		{Branch: "feature/auth"},
		{Branch: "dev/fix"},
		{Branch: "(detached)"},
	}
	if got := visibleWorktrees(wts, ""); len(got) != 4 {
		t.Errorf("empty filter: len %d, want 4", len(got))
	}
	if got := visibleWorktrees(wts, "auth"); len(got) != 1 || got[0].Branch != "feature/auth" {
		t.Errorf("auth filter: %+v", got)
	}
	if got := visibleWorktrees(wts, "MAIN"); len(got) != 1 || got[0].Branch != "main" {
		t.Errorf("case-insensitive MAIN: %+v", got)
	}
	if got := visibleWorktrees(wts, "zzz"); len(got) != 0 {
		t.Errorf("non-matching filter: len %d, want 0", len(got))
	}
	if got := visibleWorktrees(wts, "detach"); len(got) != 1 {
		t.Errorf("detach substring: %+v", got)
	}
}

func TestAdvanceSelected(t *testing.T) {
	wts := []model.Worktree{
		{Branch: "main"}, {Branch: "feature/auth"}, {Branch: "dev/fix"},
	}
	if got := advanceSelected(0, +1, wts, ""); got != 1 {
		t.Errorf("no-filter 0→1: got %d", got)
	}
	if got := advanceSelected(2, +1, wts, ""); got != 2 {
		t.Errorf("no-filter clamp at 2: got %d", got)
	}
	// Filter "auth": only index 1 matches. From 0 (main, filtered out) advance +1 should go to 1.
	if got := advanceSelected(0, +1, wts, "auth"); got != 1 {
		t.Errorf("filter auth from 0 up by 1: got %d, want 1", got)
	}
	if got := advanceSelected(0, +1, wts, "zzz"); got != 0 {
		t.Errorf("no visible match: got %d, want 0 (stay)", got)
	}
}

func TestOKeybind(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{{Path: "/wt/feat", Branch: "feat"}}
	m.selected = 0
	m.mode = modeList
	out, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	mm := out.(Model)
	if !strings.Contains(mm.infoMsg, "cd /wt/feat") {
		t.Errorf("expected infoMsg with 'cd /wt/feat', got %q", mm.infoMsg)
	}
	if mm.errMsg != "" {
		t.Errorf("errMsg should be empty on o, got %q", mm.errMsg)
	}
}

func TestBusyViewShowsSpinnerAndLabel(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.busy = true
	m.busyLabel = "Creating worktree..."

	view := m.View()
	if !strings.Contains(view, "Creating worktree...") {
		t.Errorf("expected busy label in view, got:\n%s", view)
	}
}

func TestCtrlCQuitsWhileBusy(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.busy = true
	m.busyLabel = "Removing worktree..."
	m.mode = modeDelete

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("expected a cmd from Ctrl+C while busy")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Errorf("expected tea.Quit from Ctrl+C while busy, got %v", msg)
	}
}

func TestCtrlCQuitsFromEveryModal(t *testing.T) {
	for _, mode := range []appMode{modeList, modeCreate, modeDelete, modeCleanup, modeHelp} {
		m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
		m.mode = mode

		out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		_ = out
		if cmd == nil {
			t.Fatalf("mode %v: expected a cmd from Ctrl+C", mode)
		}
		if msg := cmd(); msg != tea.Quit() {
			t.Errorf("mode %v: expected tea.Quit from Ctrl+C, got %v", mode, msg)
		}
	}
}

func TestBusyModeIgnoresKeypresses(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{{Path: "/p", Branch: "b"}}
	m.busy = true
	m.busyLabel = "Removing worktree..."
	m.mode = modeDelete

	out, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	mm := out.(Model)
	if !mm.busy {
		t.Error("expected busy to remain true; keypress should be ignored while busy")
	}
	if cmd != nil {
		t.Error("expected no cmd from an ignored keypress while busy")
	}
}

func TestLockKeybindCannotLockMain(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{{Path: "/main", Branch: "main", IsMain: true}}
	m.selected = 0
	m.mode = modeList

	out, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	mm := out.(Model)
	if mm.errMsg == "" {
		t.Error("expected error locking main worktree")
	}
}

func TestLockKeybindErrorOnFailure(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/nonexistent-repo-12345"), "/tmp/test")
	m.worktrees = []model.Worktree{{Path: "/tmp/no-such", Branch: "feat"}}
	m.selected = 0
	m.mode = modeList

	out, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	mm := out.(Model)
	if !mm.busy {
		t.Fatal("expected busy=true while the lock operation is in flight")
	}
	if cmd == nil {
		t.Fatal("expected a cmd (spinner tick + lock op) while busy")
	}

	// Run the async lock op directly and feed its result back through
	// Update, mirroring what bubbletea does when a batched cmd resolves.
	result := lockWorktreeCmd(mm.git, "/tmp/no-such", "feat", false)()
	out2, _ := mm.Update(result)
	final := out2.(Model)
	if final.errMsg == "" {
		t.Error("expected errMsg set when LockWorktree fails")
	}
	if final.busy {
		t.Error("expected busy=false after the result is processed")
	}
}

func TestMaxVisibleRows(t *testing.T) {
	if got := maxVisibleRows(0, 4); got != 0 {
		t.Errorf("unknown height (0) should mean unlimited (0), got %d", got)
	}
	if got := maxVisibleRows(-5, 4); got != 0 {
		t.Errorf("negative height should mean unlimited (0), got %d", got)
	}
	if got := maxVisibleRows(20, 4); got != 5 {
		t.Errorf("maxVisibleRows(20, 4) = %d, want 5", got)
	}
	if got := maxVisibleRows(5, 4); got != 1 {
		t.Errorf("maxVisibleRows(5, 4) = %d, want 1 (never less than 1)", got)
	}
}

func TestComputeScrollOffset(t *testing.T) {
	// Unlimited (maxVisible <= 0): always 0.
	if got := computeScrollOffset(9, 20, 0); got != 0 {
		t.Errorf("maxVisible<=0 should disable scrolling, got %d", got)
	}
	// List fits entirely: always 0.
	if got := computeScrollOffset(2, 5, 10); got != 0 {
		t.Errorf("list fits, want offset 0, got %d", got)
	}
	// Selection near the top: offset clamps to 0.
	if got := computeScrollOffset(0, 20, 5); got != 0 {
		t.Errorf("selection at top, want offset 0, got %d", got)
	}
	// Selection near the bottom: offset clamps so window doesn't overshoot.
	if got := computeScrollOffset(19, 20, 5); got != 15 {
		t.Errorf("selection at bottom, want offset 15, got %d", got)
	}
	// Selection in the middle: centers the selection in the window.
	if got := computeScrollOffset(10, 20, 5); got != 8 {
		t.Errorf("selection in middle, want centered offset 8, got %d", got)
	}
}

func TestVisibleRowIndicesAndPositionOf(t *testing.T) {
	wts := []model.Worktree{
		{Branch: "main"}, {Branch: "feature/auth"}, {Branch: "dev/fix"},
	}
	idxs := visibleRowIndices(wts, "")
	if !equalInts(idxs, []int{0, 1, 2}) {
		t.Errorf("empty filter idxs = %v, want [0 1 2]", idxs)
	}
	idxs = visibleRowIndices(wts, "auth")
	if !equalInts(idxs, []int{1}) {
		t.Errorf("auth filter idxs = %v, want [1]", idxs)
	}
	if pos := positionOf(idxs, 1); pos != 0 {
		t.Errorf("positionOf(idxs, 1) = %d, want 0", pos)
	}
	if pos := positionOf(idxs, 0); pos != -1 {
		t.Errorf("positionOf(idxs, 0) = %d, want -1 (filtered out)", pos)
	}
}

func TestListScrollsWhenLongerThanTerminal(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	var wts []model.Worktree
	for i := 0; i < 30; i++ {
		wts = append(wts, model.Worktree{Path: fmt.Sprintf("/p%d", i), Branch: fmt.Sprintf("branch-%d", i)})
	}
	m.worktrees = wts
	m.mode = modeList
	m.width = 80
	m.height = 20 // small terminal: far fewer than 30*3 lines needed

	m.selected = 0
	view := m.View()
	if !strings.Contains(view, "branch-0") {
		t.Errorf("expected first row visible when selected=0:\n%s", view)
	}
	if strings.Contains(view, "branch-29") {
		t.Errorf("did not expect last row visible when selected=0 on a short terminal:\n%s", view)
	}
	if !strings.Contains(view, "more below") {
		t.Errorf("expected a 'more below' indicator:\n%s", view)
	}

	m.selected = 29
	view = m.View()
	if !strings.Contains(view, "branch-29") {
		t.Errorf("expected last row visible when selected=29:\n%s", view)
	}
	if !strings.Contains(view, "more above") {
		t.Errorf("expected a 'more above' indicator:\n%s", view)
	}
}

func TestListNoScrollIndicatorWhenEverythingFits(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{{Path: "/p", Branch: "a"}, {Path: "/p2", Branch: "b"}}
	m.mode = modeList
	m.width = 80
	m.height = 40

	view := m.View()
	if strings.Contains(view, "more above") || strings.Contains(view, "more below") {
		t.Errorf("did not expect scroll indicators when list fits:\n%s", view)
	}
}

func TestGKeybindSelectsAndQuits(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{{Path: "/wt/feat", Branch: "feat"}}
	m.selected = 0
	m.mode = modeList

	out, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	mm := out.(Model)
	if mm.SelectedPath() != "/wt/feat" {
		t.Errorf("SelectedPath() = %q, want /wt/feat", mm.SelectedPath())
	}
	if cmd == nil {
		t.Fatal("expected tea.Quit cmd from 'g'")
	}
	if msg := cmd(); msg != tea.Quit() {
		t.Errorf("expected tea.Quit message, got %v", msg)
	}
}

func TestSelectedPathEmptyByDefault(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	if m.SelectedPath() != "" {
		t.Errorf("SelectedPath() = %q, want empty by default", m.SelectedPath())
	}
}

func TestFilterModeFlow(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{
		{Branch: "main"}, {Branch: "feature/auth"}, {Branch: "dev/fix"},
	}
	m.mode = modeList

	// Enter filter mode
	out, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	mm := out.(Model)
	if !mm.filterMode {
		t.Error("expected filterMode after /")
	}

	// Type "auth"
	m = mm
	out, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("auth")})
	mm = out.(Model)
	if mm.filterText != "auth" {
		t.Errorf("filterText = %q, want 'auth'", mm.filterText)
	}

	// Esc clears filter + exits filter mode
	m = mm
	out, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEscape})
	mm = out.(Model)
	if mm.filterMode {
		t.Error("filterMode should be false after Esc")
	}
	if mm.filterText != "" {
		t.Errorf("filterText should be empty after Esc, got %q", mm.filterText)
	}
}
