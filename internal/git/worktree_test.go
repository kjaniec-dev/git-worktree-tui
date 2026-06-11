package git

import (
	"testing"
)

func TestParseWorktreeList(t *testing.T) {
	output := `worktree /path/to/main
HEAD abc123def456
branch refs/heads/main

worktree /path/to/feature
HEAD def456abc789
branch refs/heads/feature/auth

worktree /path/to/locked
HEAD 789abc123def
branch refs/heads/hotfix
locked

`

	worktrees, err := parseWorktreeList(output)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(worktrees) != 3 {
		t.Fatalf("Expected 3 worktrees, got %d", len(worktrees))
	}

	// First worktree (main)
	if worktrees[0].Path != "/path/to/main" {
		t.Errorf("Expected path /path/to/main, got %s", worktrees[0].Path)
	}
	if worktrees[0].Branch != "main" {
		t.Errorf("Expected branch main, got %s", worktrees[0].Branch)
	}
	if worktrees[0].Commit != "abc123def456" {
		t.Errorf("Expected commit abc123def456, got %s", worktrees[0].Commit)
	}
	if !worktrees[0].IsMain {
		t.Errorf("Expected first worktree to be main")
	}

	// Second worktree (feature)
	if worktrees[1].Branch != "feature/auth" {
		t.Errorf("Expected branch feature/auth, got %s", worktrees[1].Branch)
	}

	// Third worktree (locked)
	if !worktrees[2].IsLocked {
		t.Errorf("Expected third worktree to be locked")
	}
}

func TestParseWorktreeListDetached(t *testing.T) {
	output := `worktree /path/to/detached
HEAD abc123def456
detached

`

	worktrees, err := parseWorktreeList(output)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(worktrees) != 1 {
		t.Fatalf("Expected 1 worktree, got %d", len(worktrees))
	}

	if !worktrees[0].Detached {
		t.Errorf("Expected worktree to be detached")
	}
	if worktrees[0].Branch != "(detached)" {
		t.Errorf("Expected branch (detached), got %s", worktrees[0].Branch)
	}
}

func TestAddWorktreeCommand(t *testing.T) {
	// This is a unit test - we're testing the command construction, not actual git
	// In real usage, this would be an integration test with a real repo
	g := NewGitService("/tmp/test-repo")
	
	// We can't easily test the actual execution without mocking exec.Command
	// So we'll test the error handling path
	err := g.AddWorktree("/tmp/test-worktree", "feature/test", "main", true)
	// This will fail because /tmp/test-repo doesn't exist, but that's expected
	if err == nil {
		t.Error("Expected error when repo doesn't exist")
	}
}

func TestRemoveWorktreeCommand(t *testing.T) {
	g := NewGitService("/tmp/test-repo")
	
	err := g.RemoveWorktree("/tmp/nonexistent-worktree")
	// This will fail because path doesn't exist
	if err == nil {
		t.Error("Expected error when worktree doesn't exist")
	}
}