package tui

import (
	"testing"

	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
)

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