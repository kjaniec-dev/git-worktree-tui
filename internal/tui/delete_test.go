package tui

import (
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
)

func TestDeleteModal(t *testing.T) {
	gitService := git.NewGitService("/tmp/test")
	m := NewModel(gitService, "/tmp/test")
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
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
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

func TestRemoveWorktreeCmdNeverForceDeletesBranch(t *testing.T) {
	// A worktree with uncommitted changes (force=true for the worktree
	// removal itself) must NOT cause its branch to be force-deleted if that
	// branch has commits that live nowhere else — force-removing the
	// worktree's dirty files is unrelated to whether the branch's commits
	// are safe to discard.
	repo := t.TempDir()
	run := func(args ...string) {
		fullArgs := append([]string{"-C", repo}, args...)
		if out, err := exec.Command("git", fullArgs...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "t@t")
	run("config", "user.name", "t")
	run("commit", "--allow-empty", "-m", "init")

	g := git.NewGitService(repo)
	wtPath := repo + "-wt"
	if err := g.AddWorktree(wtPath, "feat-unmerged", "main", true); err != nil {
		t.Fatalf("AddWorktree: %v", err)
	}
	// Give the branch a commit that only exists there (unmerged into main).
	run2 := func(args ...string) {
		fullArgs := append([]string{"-C", wtPath}, args...)
		if out, err := exec.Command("git", fullArgs...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run2("commit", "--allow-empty", "-m", "unique unmerged commit")

	result := removeWorktreeCmd(g, wtPath, "feat-unmerged", true /* force worktree removal */, true /* also delete branch */)().(deleteResultMsg)
	if result.removeErr != nil {
		t.Fatalf("worktree removal failed: %v", result.removeErr)
	}
	if result.branchDelErr == nil {
		t.Fatal("expected safe branch delete to fail (branch has unmerged commits), but it succeeded — commits may have been force-deleted")
	}

	exists, err := g.BranchExists("feat-unmerged")
	if err != nil {
		t.Fatalf("BranchExists: %v", err)
	}
	if !exists {
		t.Error("branch with unmerged commits should NOT have been deleted just because the worktree removal was forced")
	}
}

func TestDeleteBranchToggle(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{{Path: "/p", Branch: "feat", IsMain: false}}
	m.selected = 0
	m.mode = modeDelete

	if m.delete.alsoDeleteBranch {
		t.Fatal("expected alsoDeleteBranch to default to false")
	}

	out, _ := m.handleDeleteKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	mm := out.(Model)
	if !mm.delete.alsoDeleteBranch {
		t.Error("expected alsoDeleteBranch = true after pressing b")
	}

	view := mm.View()
	if !strings.Contains(view, "[x] Also delete branch") {
		t.Errorf("expected checked checkbox in view, got:\n%s", view)
	}
}

func TestDeleteResetsBranchToggleOnEntry(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{{Path: "/p", Branch: "feat", IsMain: false}}
	m.selected = 0
	m.delete.alsoDeleteBranch = true
	m.mode = modeList

	result, _ := m.Update(teaKeyPress("d"))
	mm := result.(Model)
	if mm.delete.alsoDeleteBranch {
		t.Error("expected alsoDeleteBranch reset to false on entering delete mode")
	}
}

func TestDeleteClearsErrMsgOnEntry(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
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
