package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
)

func TestClassifyStale(t *testing.T) {
	branchSet := map[string]bool{"main": true, "develop": true}
	wts := []model.Worktree{
		{Path: "/main", Branch: "main", IsMain: true},                                  // excluded: main
		{Path: "/locked", Branch: "develop", IsLocked: true},                          // excluded: locked
		{Path: "/detached", Branch: "(detached)", Detached: true},                     // excluded: detached
		{Path: "/dirty", Branch: "develop", Status: &model.WorktreeStatus{IsDirty: true}}, // excluded: dirty
		{Path: "/stale", Branch: "gone-branch"},                                       // stale: branch not in set
		{Path: "/clean", Branch: "develop"},                                           // not stale: branch in set
	}
	got := classifyStale(wts, branchSet)
	want := []int{4}
	if !equalInts(got, want) {
		t.Errorf("classifyStale = %v, want %v", got, want)
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestCleanupModal(t *testing.T) {
	gitService := git.NewGitService("/tmp/test")
	m := NewModel(gitService)
	m.worktrees = []model.Worktree{
		{Path: "/path/to/stale", Branch: "stale-branch", IsMain: false},
	}
	m.mode = modeCleanup

	view := m.View()

	if view == "" {
		t.Error("Expected cleanup modal view")
	}
}

func TestCleanupEnterAccumulatesErrors(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/nonexistent-repo-12345"))
	// Two stale worktrees pointing at nonexistent paths -> both RemoveWorktree calls error.
	m.worktrees = []model.Worktree{
		{Path: "/tmp/no-such-1", Branch: "gone1"},
		{Path: "/tmp/no-such-2", Branch: "gone2"},
	}
	m.cleanup.staleWorktrees = []int{0, 1}
	m.cleanup.selected = []bool{true, true}
	m.cleanup.currentIndex = 0
	m.mode = modeCleanup

	out, cmd := m.handleCleanupKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	mm := out.(Model)

	// Should have returned to list mode with a refresh command regardless of failures.
	if mm.mode != modeList {
		t.Errorf("mode = %v, want modeList", mm.mode)
	}
	if cmd == nil {
		t.Error("expected a loadWorktrees cmd even after partial failures")
	}
	if mm.errMsg == "" {
		t.Error("expected accumulated error message from failed removals")
	}
	if !strings.Contains(mm.errMsg, "/tmp/no-such-1") || !strings.Contains(mm.errMsg, "/tmp/no-such-2") {
		t.Errorf("expected both failed paths in errMsg, got: %s", mm.errMsg)
	}
}