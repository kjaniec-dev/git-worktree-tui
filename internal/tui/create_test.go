package tui

import (
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

func TestCreateEnterErrorStaysInCreate(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/nonexistent-repo-12345"), "/tmp/test")
	m.mode = modeCreate
	m.create.branchName = "feat-x"
	m.create.baseBranch = "main"
	m.create.createBranch = true

	out, cmd := m.handleCreateKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	mm := out.(Model)
	if mm.mode != modeCreate {
		t.Errorf("mode = %v; want modeCreate on error", mm.mode)
	}
	if mm.create.errMsg == "" {
		t.Error("expected friendly error on create form, got empty errMsg")
	}
	if cmd != nil {
		t.Errorf("expected no loadWorktrees cmd on error, got %v", cmd)
	}
}
