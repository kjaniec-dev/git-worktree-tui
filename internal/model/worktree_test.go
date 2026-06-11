package model

import "testing"

func TestWorktreeStatus(t *testing.T) {
	status := &WorktreeStatus{
		IsDirty:   true,
		Ahead:     2,
		Behind:    1,
		HasStash:  true,
		Untracked: 5,
	}

	if !status.IsDirty {
		t.Error("expected IsDirty to be true")
	}
	if status.Ahead != 2 {
		t.Errorf("expected Ahead to be 2, got %d", status.Ahead)
	}
	if status.Behind != 1 {
		t.Errorf("expected Behind to be 1, got %d", status.Behind)
	}
	if !status.HasStash {
		t.Error("expected HasStash to be true")
	}
	if status.Untracked != 5 {
		t.Errorf("expected Untracked to be 5, got %d", status.Untracked)
	}
}

func TestWorktree(t *testing.T) {
	status := &WorktreeStatus{
		IsDirty:   false,
		Ahead:     0,
		Behind:    0,
		HasStash:  false,
		Untracked: 0,
	}

	wt := &Worktree{
		Path:     "/path/to/worktree",
		Branch:   "main",
		Commit:   "abc1234",
		IsMain:   true,
		IsLocked: false,
		IsBare:   false,
		Detached: false,
		Status:   status,
	}

	if wt.Path != "/path/to/worktree" {
		t.Errorf("expected Path to be /path/to/worktree, got %s", wt.Path)
	}
	if wt.Branch != "main" {
		t.Errorf("expected Branch to be main, got %s", wt.Branch)
	}
	if wt.Commit != "abc1234" {
		t.Errorf("expected Commit to be abc1234, got %s", wt.Commit)
	}
	if !wt.IsMain {
		t.Error("expected IsMain to be true")
	}
	if wt.IsLocked {
		t.Error("expected IsLocked to be false")
	}
	if wt.IsBare {
		t.Error("expected IsBare to be false")
	}
	if wt.Detached {
		t.Error("expected Detached to be false")
	}
	if wt.Status == nil {
		t.Error("expected Status to be non-nil")
	}
}

func TestWorktreeWithNilStatus(t *testing.T) {
	wt := &Worktree{
		Path:     "/path/to/worktree",
		Branch:   "feature",
		Commit:   "def5678",
		IsMain:   false,
		IsLocked: true,
		IsBare:   false,
		Detached: false,
		Status:   nil,
	}

	if wt.Status != nil {
		t.Error("expected Status to be nil")
	}
	if !wt.IsLocked {
		t.Error("expected IsLocked to be true")
	}
}