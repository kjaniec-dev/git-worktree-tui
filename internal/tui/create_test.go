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
		expected string
	}{
		{"/Users/dev/myproject", "feature/auth", "/Users/dev/myproject/.worktrees/feature-auth"},
		{"/Users/dev/myproject", "main", "/Users/dev/myproject/.worktrees/main"},
		{"/Users/dev/myproject", "hotfix/critical/bug", "/Users/dev/myproject/.worktrees/hotfix-critical-bug"},
	}

	for _, tt := range tests {
		result := generateWorktreePath(tt.repoRoot, tt.branch)
		if result != tt.expected {
			t.Errorf("generateWorktreePath(%s, %s) = %s, expected %s", tt.repoRoot, tt.branch, result, tt.expected)
		}
	}
}