package tui

import (
	"testing"

	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
)

func TestNewModel(t *testing.T) {
	gitService := git.NewGitService("/tmp/test")
	m := NewModel(gitService)

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