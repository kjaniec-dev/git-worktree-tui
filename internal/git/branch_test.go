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

func TestBuildDeleteBranchArgs(t *testing.T) {
	if got := buildDeleteBranchArgs("feat", false); !equalSlices(got, []string{"branch", "-d", "feat"}) {
		t.Errorf("safe delete args = %v", got)
	}
	if got := buildDeleteBranchArgs("feat", true); !equalSlices(got, []string{"branch", "-D", "feat"}) {
		t.Errorf("force delete args = %v", got)
	}
}

func TestMergedBranchesAndDeleteBranch(t *testing.T) {
	repo := t.TempDir()
	run := func(args ...string) {
		fullArgs := append([]string{"-C", repo}, args...)
		if out, err := exec.Command("git", fullArgs...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "base")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	run("commit", "--allow-empty", "-m", "init")
	run("branch", "merged-branch") // fully merged into base (same commit)
	run("checkout", "-b", "unmerged-branch")
	run("commit", "--allow-empty", "-m", "unmerged commit")
	run("checkout", "base")

	g := NewGitService(repo)

	merged, err := g.MergedBranches("base")
	if err != nil {
		t.Fatalf("MergedBranches: %v", err)
	}
	mergedSet := make(map[string]bool)
	for _, b := range merged {
		mergedSet[b] = true
	}
	if !mergedSet["merged-branch"] {
		t.Errorf("expected merged-branch in MergedBranches(base), got %v", merged)
	}
	if mergedSet["unmerged-branch"] {
		t.Errorf("unmerged-branch should NOT be in MergedBranches(base), got %v", merged)
	}

	if err := g.DeleteBranch("merged-branch", false); err != nil {
		t.Errorf("DeleteBranch(merged-branch, safe) failed: %v", err)
	}
	if err := g.DeleteBranch("unmerged-branch", false); err == nil {
		t.Error("expected safe DeleteBranch to fail on unmerged branch")
	}
	if err := g.DeleteBranch("unmerged-branch", true); err != nil {
		t.Errorf("DeleteBranch(unmerged-branch, force) failed: %v", err)
	}
}
