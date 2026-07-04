package tui

import (
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
)

func sendCreate(m Model, msg tea.KeyMsg) tea.Model {
	result, _ := m.handleCreateKeyPress(msg)
	return result
}

func teaKeyPress(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func teaKeyDown() tea.KeyMsg      { return tea.KeyMsg{Type: tea.KeyDown} }
func teaKeyUp() tea.KeyMsg        { return tea.KeyMsg{Type: tea.KeyUp} }
func teaKeyBackspace() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyBackspace} }

func TestCreateModal(t *testing.T) {
	gitService := git.NewGitService("/tmp/test")
	m := NewModel(gitService, "/tmp/test")
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

func TestBaseFieldSelectorOnly(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.mode = modeCreate
	m.create.branches = []string{"develop", "main", "feat"}
	m.create.baseBranch = "main"
	m.create.baseIndex = 1
	m.create.currentField = fieldBase

	// Down: baseIndex advances and baseBranch tracks branches[baseIndex]
	m = sendCreate(m, teaKeyDown()).(Model)
	if m.create.baseIndex != 2 || m.create.baseBranch != "feat" {
		t.Errorf("after Down: idx=%d base=%q; want 2,feat", m.create.baseIndex, m.create.baseBranch)
	}
	// Up: back
	m = sendCreate(m, teaKeyUp()).(Model)
	if m.create.baseIndex != 1 || m.create.baseBranch != "main" {
		t.Errorf("after Up: idx=%d base=%q; want 1,main", m.create.baseIndex, m.create.baseBranch)
	}
	// Typing runes on Base must NOT append to baseBranch
	before := m.create.baseBranch
	m = sendCreate(m, teaKeyPress("x")).(Model)
	if m.create.baseBranch != before {
		t.Errorf("Base accepted typed runes: %q changed to %q", before, m.create.baseBranch)
	}
	// Backspace on Base must NOT delete from baseBranch
	m = sendCreate(m, teaKeyBackspace()).(Model)
	if m.create.baseBranch != before {
		t.Errorf("Base accepted backspace: %q changed to %q", before, m.create.baseBranch)
	}
}

func TestCreateBranchCheckboxReflectsState(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.mode = modeCreate
	m.create.createBranch = true

	view := m.View()
	if !strings.Contains(view, "☑ Create new branch from base") {
		t.Errorf("expected checked box (☑) when createBranch=true, got:\n%s", view)
	}
	if strings.Contains(view, "☐ Create new branch from base") {
		t.Errorf("did not expect unchecked box (☐) when createBranch=true, got:\n%s", view)
	}

	m.create.createBranch = false
	view = m.View()
	if !strings.Contains(view, "☐ Create new branch from base") {
		t.Errorf("expected unchecked box (☐) when createBranch=false, got:\n%s", view)
	}
}

func TestAKeybindResetsAndRefreshesForm(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.mode = modeList
	// Simulate leftover state from a previous create-form session.
	m.create.branchName = "leftover-branch"
	m.create.createBranch = false
	m.create.location = "outside"
	m.create.errMsg = "stale error"

	out, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	mm := out.(Model)

	if mm.mode != modeCreate {
		t.Fatalf("mode = %v, want modeCreate", mm.mode)
	}
	if mm.create.branchName != "" {
		t.Errorf("expected branchName reset to empty, got %q", mm.create.branchName)
	}
	if !mm.create.createBranch {
		t.Error("expected createBranch reset to true (default)")
	}
	if mm.create.location != "inside" {
		t.Errorf("expected location reset to 'inside', got %q", mm.create.location)
	}
	if mm.create.errMsg != "" {
		t.Errorf("expected errMsg reset to empty, got %q", mm.create.errMsg)
	}
}

func TestAKeybindRefreshesBranchList(t *testing.T) {
	repo := t.TempDir()
	for _, args := range [][]string{
		{"-C", repo, "init", "-b", "main"},
		{"-C", repo, "config", "user.email", "t@t"},
		{"-C", repo, "config", "user.name", "t"},
		{"-C", repo, "commit", "--allow-empty", "-m", "init"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	m := NewModel(git.NewGitService(repo), repo)
	m.mode = modeList

	// A branch created after NewModel() ran (e.g. by a previous create-form
	// session) must show up as a base option the next time 'a' is pressed.
	if out, err := exec.Command("git", "-C", repo, "branch", "new-branch").CombinedOutput(); err != nil {
		t.Fatalf("git branch new-branch: %v\n%s", err, out)
	}

	out, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	mm := out.(Model)

	found := false
	for _, b := range mm.create.branches {
		if b == "new-branch" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected refreshed branch list to include 'new-branch', got %v", mm.create.branches)
	}
}

func TestCreateEnterErrorStaysInCreate(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/nonexistent-repo-12345"), "/tmp/test")
	m.mode = modeCreate
	m.create.branchName = "feat-x"
	m.create.baseBranch = "main"
	m.create.createBranch = true

	out, cmd := m.handleCreateKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	mm := out.(Model)
	if !mm.busy {
		t.Fatal("expected busy=true while AddWorktree runs asynchronously")
	}
	if cmd == nil {
		t.Fatal("expected a cmd (spinner tick + create op) while busy")
	}

	// Run the async create op directly and feed its result back through
	// Update, mirroring what bubbletea does when a batched cmd resolves.
	path := generateWorktreePath(mm.git.RepoRoot, mm.create.branchName, mm.create.location)
	result := addWorktreeCmd(mm.git, path, mm.create.branchName, mm.create.baseBranch, mm.create.createBranch)()
	out2, cmd2 := mm.Update(result)
	final := out2.(Model)
	if final.mode != modeCreate {
		t.Errorf("mode = %v; want modeCreate on error", final.mode)
	}
	if final.create.errMsg == "" {
		t.Error("expected friendly error on create form, got empty errMsg")
	}
	if final.busy {
		t.Error("expected busy=false after the result is processed")
	}
	if cmd2 != nil {
		t.Errorf("expected no loadWorktrees cmd on error, got %v", cmd2)
	}
}
