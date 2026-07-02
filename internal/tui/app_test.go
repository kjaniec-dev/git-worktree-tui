package tui

import (
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
