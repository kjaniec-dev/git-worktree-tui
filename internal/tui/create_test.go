package tui

import (
	"testing"

	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
)

func TestCreateModal(t *testing.T) {
	gitService := git.NewGitService("/tmp/test")
	m := NewModel(gitService)
	m.mode = modeCreate

	view := m.View()
	
	if view == "" {
		t.Error("Expected create modal view")
	}
}

func TestPathAutoGeneration(t *testing.T) {
	tests := []struct {
		repoRoot string
		branch   string
		location string
		expected string
	}{
		{"/Users/dev/myproject", "feature/auth", "inside", "/Users/dev/myproject/.worktrees/feature-auth"},
		{"/Users/dev/myproject", "main", "inside", "/Users/dev/myproject/.worktrees/main"},
		{"/Users/dev/myproject", "hotfix/critical/bug", "inside", "/Users/dev/myproject/.worktrees/hotfix-critical-bug"},
		{"/Users/dev/myproject", "feature/auth", "outside", "/Users/dev/myproject-feature-auth"},
		{"/Users/dev/myproject", "main", "outside", "/Users/dev/myproject-main"},
		{"/Users/dev/myproject", "hotfix/critical/bug", "outside", "/Users/dev/myproject-hotfix-critical-bug"},
	}

	for _, tt := range tests {
		result := generateWorktreePath(tt.repoRoot, tt.branch, tt.location)
		if result != tt.expected {
			t.Errorf("generateWorktreePath(%s, %s, %s) = %s, expected %s", tt.repoRoot, tt.branch, tt.location, result, tt.expected)
		}
	}
}