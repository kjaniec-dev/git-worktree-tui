package git

import (
	"os"
	"os/exec"
	"testing"
)

func TestParseBranchList(t *testing.T) {
	output := `  feature/auth
* main
  feature/ui
`

	branches := parseBranchList(output)
	
	if len(branches) != 3 {
		t.Fatalf("Expected 3 branches, got %d", len(branches))
	}

	expected := []string{"feature/auth", "main", "feature/ui"}
	for i, branch := range branches {
		if branch != expected[i] {
			t.Errorf("Expected branch %s, got %s", expected[i], branch)
		}
	}
}

func TestBranchExists(t *testing.T) {
	origDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origDir) })

	repo := t.TempDir()
	if err := os.Chdir(repo); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	if out, err := exec.Command("git", "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "config", "user.email", "t@t").CombinedOutput(); err != nil {
		t.Fatalf("config email: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "config", "user.name", "t").CombinedOutput(); err != nil {
		t.Fatalf("config name: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "commit", "--allow-empty", "-m", "init").CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "branch", "develop").CombinedOutput(); err != nil {
		t.Fatalf("branch develop: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "tag", "v1").CombinedOutput(); err != nil {
		t.Fatalf("tag v1: %v\n%s", err, out)
	}

	g := NewGitService(repo)

	if exists, err := g.BranchExists("develop"); err != nil || !exists {
		t.Errorf("BranchExists(develop) = %v, %v; want true, nil", exists, err)
	}
	if exists, err := g.BranchExists("v1"); err != nil || exists {
		t.Errorf("BranchExists(v1/tag) = %v, %v; want false, nil (must not match tags)", exists, err)
	}
	if exists, err := g.BranchExists("nope"); err != nil || exists {
		t.Errorf("BranchExists(nope/missing) = %v, %v; want false, nil", exists, err)
	}
}