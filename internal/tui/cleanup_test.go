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
	mergedSet := map[string]bool{}
	wts := []model.Worktree{
		{Path: "/main", Branch: "main", IsMain: true},                                     // excluded: main
		{Path: "/locked", Branch: "develop", IsLocked: true},                              // excluded: locked
		{Path: "/detached", Branch: "(detached)", Detached: true},                         // excluded: detached
		{Path: "/dirty", Branch: "develop", Status: &model.WorktreeStatus{IsDirty: true}}, // excluded: dirty
		{Path: "/stale", Branch: "gone-branch"},                                           // stale: branch not in set
		{Path: "/clean", Branch: "develop"},                                               // not stale: branch in set
	}
	got := classifyStale(wts, branchSet, mergedSet)
	want := []int{4}
	if !equalInts(got, want) {
		t.Errorf("classifyStale = %v, want %v", got, want)
	}
}

func TestClassifyStaleMerged(t *testing.T) {
	branchSet := map[string]bool{"main": true, "feat-merged": true, "feat-open": true}
	mergedSet := map[string]bool{"feat-merged": true}
	wts := []model.Worktree{
		{Path: "/main", Branch: "main", IsMain: true},
		{Path: "/merged", Branch: "feat-merged"}, // stale: merged into base
		{Path: "/open", Branch: "feat-open"},     // not stale: not merged, branch exists
	}
	got := classifyStale(wts, branchSet, mergedSet)
	want := []int{1}
	if !equalInts(got, want) {
		t.Errorf("classifyStale (merged) = %v, want %v", got, want)
	}
}

func TestStaleReason(t *testing.T) {
	branchSet := map[string]bool{"main": true, "feat-merged": true}
	mergedSet := map[string]bool{"feat-merged": true}

	if got := staleReason("gone-branch", branchSet, mergedSet); got != "branch deleted" {
		t.Errorf("staleReason(deleted) = %q, want %q", got, "branch deleted")
	}
	if got := staleReason("feat-merged", branchSet, mergedSet); got != "merged" {
		t.Errorf("staleReason(merged) = %q, want %q", got, "merged")
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
	m := NewModel(gitService, "/tmp/test")
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
	m := NewModel(git.NewGitService("/tmp/nonexistent-repo-12345"), "/tmp/test")
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
	if !mm.busy {
		t.Fatal("expected busy=true while cleanup runs asynchronously")
	}
	if cmd == nil {
		t.Fatal("expected a cmd (spinner tick + cleanup op) while busy")
	}

	// Run the async cleanup op directly and feed its result back through
	// Update, mirroring what bubbletea does when a batched cmd resolves.
	result := cleanupWorktreesCmd(mm.git, []string{"/tmp/no-such-1", "/tmp/no-such-2"}, []string{"gone1", "gone2"})()
	out2, cmd2 := mm.Update(result)
	final := out2.(Model)

	// Should have returned to list mode with a refresh command regardless of failures.
	if final.mode != modeList {
		t.Errorf("mode = %v, want modeList", final.mode)
	}
	if cmd2 == nil {
		t.Error("expected a loadWorktrees cmd even after partial failures")
	}
	if final.errMsg == "" {
		t.Error("expected accumulated error message from failed removals")
	}
	if !strings.Contains(final.errMsg, "/tmp/no-such-1") || !strings.Contains(final.errMsg, "/tmp/no-such-2") {
		t.Errorf("expected both failed paths in errMsg, got: %s", final.errMsg)
	}
}
