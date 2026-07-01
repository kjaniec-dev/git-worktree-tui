package tui

import (
	"strings"
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

func TestDeleteModalForceConfirm(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"))
	m.worktrees = []model.Worktree{
		{Path: "/p/clean", Branch: "clean", IsMain: false, Status: &model.WorktreeStatus{IsDirty: false}},
		{Path: "/p/dirty", Branch: "dirty", IsMain: false, Status: &model.WorktreeStatus{IsDirty: true}},
	}

	// Clean modal shows "yes/no"
	m.selected = 0
	m.mode = modeDelete
	if view := m.View(); !strings.Contains(view, "[y]es") || strings.Contains(view, "force-remove") {
		t.Errorf("clean modal text mismatch: %s", view)
	}

	// Dirty modal shows "[y] force-remove"
	m.selected = 1
		if view := m.View(); !strings.Contains(view, "force-remove") || !strings.Contains(view, "Changes will be lost") {
		t.Errorf("dirty modal text mismatch: %s", view)
	}
}

func TestDeleteClearsErrMsgOnEntry(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"))
	m.worktrees = []model.Worktree{{Path: "/p", Branch: "b", IsMain: false}}
	m.selected = 0
	m.errMsg = "stale from before"
	m.mode = modeList

	// Press 'd' to enter delete mode
	result, _ := m.Update(teaKeyPress("d"))
	m = result.(Model)
	if m.errMsg != "" {
		t.Errorf("errMsg should be cleared on entering delete, got %q", m.errMsg)
	}
	if m.mode != modeDelete {
		t.Errorf("mode = %v, want modeDelete", m.mode)
	}
}