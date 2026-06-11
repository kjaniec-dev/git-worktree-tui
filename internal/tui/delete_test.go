package tui

import (
	"testing"

	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
)

func TestDeleteModal(t *testing.T) {
	gitService := git.NewGitService("/tmp/test")
	m := NewModel(gitService)
	m.worktrees = []model.Worktree{
		{Path: "/path/to/feature", Branch: "feature/auth", IsMain: false},
	}
	m.selected = 0
	m.mode = modeDelete

	view := m.View()
	
	if view == "" {
		t.Error("Expected delete modal view")
	}
}