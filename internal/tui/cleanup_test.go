package tui

import (
	"testing"

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