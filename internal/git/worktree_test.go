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
	g := NewGitService("/tmp/nonexistent-repo-12345")
	err := g.AddWorktree("/tmp/test-worktree", "feature/test", "main", true)
	if err == nil {
		t.Error("Expected error when repo doesn't exist")
	}
}

func TestRemoveWorktreeCommand(t *testing.T) {
	g := NewGitService("/tmp/nonexistent-repo-12345")
	err := g.RemoveWorktree("/tmp/nonexistent-worktree")
	if err == nil {
		t.Error("Expected error when worktree doesn't exist")
	}
}

func equalSlices(a, b []string) bool {
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

func TestBuildAddArgs(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		branch       string
		base         string
		createBranch bool
		branchExists bool
		wantErr      bool
		wantArgs     []string
	}{
		{"create new + branch missing", "/p", "feat", "main", true, false, false,
			[]string{"worktree", "add", "-b", "feat", "/p", "main"}},
		{"checkout existing + branch exists", "/p", "feat", "main", false, true, false,
			[]string{"worktree", "add", "/p", "feat"}},
		{"create new + branch exists -> error", "/p", "feat", "main", true, true, true, nil},
		{"checkout existing + branch missing -> error", "/p", "feat", "main", false, false, true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, err := buildAddArgs(tt.path, tt.branch, tt.base, tt.createBranch, tt.branchExists)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tt.wantErr)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if !equalSlices(args, tt.wantArgs) {
				t.Errorf("args = %v, want %v", args, tt.wantArgs)
			}
		})
	}
}