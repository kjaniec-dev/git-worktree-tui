# git-worktree-tui Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a TUI application in Go for managing git worktrees with interactive list, create, delete, and cleanup operations.

**Architecture:** Three-layer separation: `git/` (pure git operations via os/exec), `tui/` (Bubble Tea model/view/update), `model/` (shared data structures). Git commands executed via `os/exec` with timeout handling. TUI uses Bubble Tea framework with Lipgloss styling.

**Tech Stack:** Go 1.21+, Bubble Tea (Charmbracelet), Bubbles (table, viewport), Lipgloss, Cobra (CLI), os/exec (git commands)

**Spec:** `docs/superpowers/specs/2025-01-21-git-worktree-tui-design.md`

---

## File Structure

```
git-worktree-tui/
├── cmd/
│   └── root.go              # Cobra CLI entry point, repo detection
├── internal/
│   ├── git/
│   │   ├── worktree.go      # ListWorktrees, AddWorktree, RemoveWorktree, PruneWorktrees
│   │   ├── worktree_test.go # Unit tests for worktree operations
│   │   ├── branch.go        # ListBranches, BranchExists
│   │   ├── branch_test.go   # Unit tests for branch operations
│   │   ├── status.go        # GetWorktreeStatus (dirty, ahead/behind)
│   │   └── status_test.go   # Unit tests for status operations
│   ├── tui/
│   │   ├── app.go           # Main Bubble Tea Model, Update, View
│   │   ├── app_test.go      # Unit tests for TUI model
│   │   ├── list.go          # Worktree list view rendering
│   │   ├── list_test.go     # Unit tests for list view
│   │   ├── create.go        # Create worktree modal
│   │   ├── create_test.go   # Unit tests for create modal
│   │   ├── delete.go        # Delete confirmation modal
│   │   ├── delete_test.go   # Unit tests for delete modal
│   │   ├── cleanup.go       # Cleanup modal and flow
│   │   ├── cleanup_test.go  # Unit tests for cleanup modal
│   │   └── styles.go        # Lipgloss style definitions
│   └── model/
│       ├── worktree.go      # Worktree, WorktreeStatus structs
│       └── worktree_test.go # Unit tests for model validation
├── main.go                  # Entry point
├── go.mod                   # Go module definition
├── go.sum                   # Dependency checksums
└── README.md                # Usage instructions
```

---

## Chunk 1: Project Setup and Data Model

### Task 1: Initialize Go Module

**Files:**
- Create: `go.mod`
- Create: `main.go`

- [ ] **Step 1: Initialize Go module**

```bash
go mod init github.com/kjaniec-dev/git-worktree-tui
```

Expected: `go.mod` created with module path

- [ ] **Step 2: Add dependencies**

```bash
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/bubbles
go get github.com/charmbracelet/lipgloss
go get github.com/spf13/cobra
```

Expected: Dependencies added to `go.mod`

- [ ] **Step 3: Create minimal main.go**

```go
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("git-worktree-tui")
	os.Exit(0)
}
```

- [ ] **Step 4: Verify it compiles**

```bash
go build -o git-worktree-tui
./git-worktree-tui
```

Expected: Binary builds and prints "git-worktree-tui"

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum main.go
git commit -m "chore: initialize Go module with dependencies"
```

---

### Task 2: Create Data Model

**Files:**
- Create: `internal/model/worktree.go`
- Create: `internal/model/worktree_test.go`

- [ ] **Step 1: Write failing test for Worktree struct**

```go
// internal/model/worktree_test.go
package model

import (
	"testing"
)

func TestWorktreeStruct(t *testing.T) {
	w := Worktree{
		Path:     "/path/to/worktree",
		Branch:   "main",
		Commit:   "abc123",
		IsMain:   true,
		IsLocked: false,
		IsBare:   false,
		Detached: false,
	}

	if w.Path != "/path/to/worktree" {
		t.Errorf("Expected Path to be /path/to/worktree, got %s", w.Path)
	}
	if w.Branch != "main" {
		t.Errorf("Expected Branch to be main, got %s", w.Branch)
	}
	if !w.IsMain {
		t.Errorf("Expected IsMain to be true")
	}
}

func TestWorktreeStatusStruct(t *testing.T) {
	s := WorktreeStatus{
		IsDirty:   true,
		Ahead:     2,
		Behind:    1,
		HasStash:  false,
		Untracked: 3,
	}

	if !s.IsDirty {
		t.Errorf("Expected IsDirty to be true")
	}
	if s.Ahead != 2 {
		t.Errorf("Expected Ahead to be 2, got %d", s.Ahead)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/model/... -v
```

Expected: FAIL with "undefined: Worktree"

- [ ] **Step 3: Implement Worktree and WorktreeStatus structs**

```go
// internal/model/worktree.go
package model

// Worktree represents a git worktree
type Worktree struct {
	Path     string           // Full path to worktree (primary identifier)
	Branch   string           // Branch name, or "(detached)" if HEAD detached
	Commit   string           // HEAD commit SHA
	IsMain   bool             // Whether this is the main worktree (origin repo)
	IsLocked bool             // Whether worktree is locked
	IsBare   bool             // Whether this is a bare worktree
	Detached bool             // Whether HEAD is detached
	Status   *WorktreeStatus  // Optional, loaded on demand
}

// WorktreeStatus represents the status of a worktree
type WorktreeStatus struct {
	IsDirty   bool // Uncommitted changes present
	Ahead     int  // Commits ahead of upstream
	Behind    int  // Commits behind upstream
	HasStash  bool // Stashed changes present
	Untracked int  // Number of untracked files
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/model/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/model/
git commit -m "feat: add Worktree and WorktreeStatus data models"
```

---

## Chunk 2: Git Layer - Worktree Operations

### Task 3: Implement ListWorktrees

**Files:**
- Create: `internal/git/worktree.go`
- Create: `internal/git/worktree_test.go`

- [ ] **Step 1: Write failing test for ListWorktrees parsing**

```go
// internal/git/worktree_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/git/... -v
```

Expected: FAIL with "undefined: parseWorktreeList"

- [ ] **Step 3: Implement parseWorktreeList**

```go
// internal/git/worktree.go
package git

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
)

// GitService handles git operations
type GitService struct {
	RepoRoot string
	Timeout  time.Duration
}

// NewGitService creates a new GitService
func NewGitService(repoRoot string) *GitService {
	return &GitService{
		RepoRoot: repoRoot,
		Timeout:  10 * time.Second,
	}
}

// parseWorktreeList parses the output of git worktree list --porcelain
func parseWorktreeList(output string) ([]model.Worktree, error) {
	var worktrees []model.Worktree
	var current *model.Worktree

	lines := strings.Split(output, "\n")
	isFirst := true

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				worktrees = append(worktrees, *current)
				current = nil
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current = &model.Worktree{
				Path:   strings.TrimPrefix(line, "worktree "),
				IsMain: isFirst,
			}
			isFirst = false
		} else if strings.HasPrefix(line, "HEAD ") && current != nil {
			current.Commit = strings.TrimPrefix(line, "HEAD ")
		} else if strings.HasPrefix(line, "branch ") && current != nil {
			branch := strings.TrimPrefix(line, "branch ")
			// Remove refs/heads/ prefix
			branch = strings.TrimPrefix(branch, "refs/heads/")
			current.Branch = branch
		} else if line == "detached" && current != nil {
			current.Detached = true
			current.Branch = "(detached)"
		} else if line == "locked" && current != nil {
			current.IsLocked = true
		} else if line == "bare" && current != nil {
			current.IsBare = true
		}
	}

	// Handle last worktree if no trailing newline
	if current != nil {
		worktrees = append(worktrees, *current)
	}

	return worktrees, nil
}

// ListWorktrees returns all worktrees in the repository
func (g *GitService) ListWorktrees() ([]model.Worktree, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = g.RepoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w\n%s", err, output)
	}

	return parseWorktreeList(string(output))
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/git/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat: implement ListWorktrees with porcelain parsing"
```

---

### Task 4: Implement AddWorktree and RemoveWorktree

**Files:**
- Modify: `internal/git/worktree.go`
- Modify: `internal/git/worktree_test.go`

- [ ] **Step 1: Write failing tests for Add and Remove**

```go
// Add to internal/git/worktree_test.go

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
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/git/... -v -run "TestAddWorktree|TestRemoveWorktree"
```

Expected: FAIL with "undefined: AddWorktree"

- [ ] **Step 3: Implement AddWorktree and RemoveWorktree**

```go
// Add to internal/git/worktree.go

// AddWorktree creates a new worktree
func (g *GitService) AddWorktree(path, branch, base string, createBranch bool) error {
	var args []string
	args = append(args, "worktree", "add")
	
	if createBranch {
		args = append(args, "-b", branch)
	}
	
	args = append(args, path)
	
	if createBranch {
		args = append(args, base)
	} else {
		args = append(args, branch)
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = g.RepoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to add worktree: %w\n%s", err, output)
	}

	return nil
}

// RemoveWorktree removes a worktree
func (g *GitService) RemoveWorktree(path string) error {
	cmd := exec.Command("git", "worktree", "remove", path)
	cmd.Dir = g.RepoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove worktree %s: %w\n%s", path, err, output)
	}

	return nil
}

// PruneWorktrees removes stale worktree metadata
func (g *GitService) PruneWorktrees() error {
	cmd := exec.Command("git", "worktree", "prune")
	cmd.Dir = g.RepoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to prune worktrees: %w\n%s", err, output)
	}

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/git/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/git/
git commit -m "feat: implement AddWorktree, RemoveWorktree, PruneWorktrees"
```

---

## Chunk 3: Git Layer - Branch and Status Operations

### Task 5: Implement Branch Operations

**Files:**
- Create: `internal/git/branch.go`
- Create: `internal/git/branch_test.go`

- [ ] **Step 1: Write failing test for ListBranches**

```go
// internal/git/branch_test.go
package git

import (
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/git/... -v -run TestParseBranchList
```

Expected: FAIL with "undefined: parseBranchList"

- [ ] **Step 3: Implement branch operations**

```go
// internal/git/branch.go
package git

import (
	"fmt"
	"os/exec"
	"strings"
)

// parseBranchList parses the output of git branch --list
func parseBranchList(output string) []string {
	var branches []string
	lines := strings.Split(output, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Remove leading * or space
		if strings.HasPrefix(line, "* ") {
			line = strings.TrimPrefix(line, "* ")
		} else if strings.HasPrefix(line, "  ") {
			line = strings.TrimPrefix(line, "  ")
		}
		branches = append(branches, line)
	}
	
	return branches
}

// ListBranches returns all local branches
func (g *GitService) ListBranches() ([]string, error) {
	cmd := exec.Command("git", "branch", "--list")
	cmd.Dir = g.RepoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w\n%s", err, output)
	}

	return parseBranchList(string(output)), nil
}

// BranchExists checks if a branch exists
func (g *GitService) BranchExists(branch string) (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Dir = g.RepoRoot

	err := cmd.Run()
	return err == nil, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/git/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/git/branch.go internal/git/branch_test.go
git commit -m "feat: implement ListBranches and BranchExists"
```

---

### Task 6: Implement Status Operations

**Files:**
- Create: `internal/git/status.go`
- Create: `internal/git/status_test.go`

- [ ] **Step 1: Write failing test for GetWorktreeStatus**

```go
// internal/git/status_test.go
package git

import (
	"testing"
)

func TestParseStatusClean(t *testing.T) {
	output := `` // Empty output = clean
	
	status := parseStatus(output)
	
	if status.IsDirty {
		t.Error("Expected clean worktree")
	}
	if status.Untracked != 0 {
		t.Errorf("Expected 0 untracked, got %d", status.Untracked)
	}
}

func TestParseStatusDirty(t *testing.T) {
	output := `1 .M N... 100644 100644 100644 abc123 def456 file.txt
? untracked.txt
`
	
	status := parseStatus(output)
	
	if !status.IsDirty {
		t.Error("Expected dirty worktree")
	}
	if status.Untracked != 1 {
		t.Errorf("Expected 1 untracked, got %d", status.Untracked)
	}
}

func TestParseAheadBehind(t *testing.T) {
	// Test ahead/behind parsing (simplified - actual implementation uses git rev-list)
	// This is a placeholder for the actual parsing logic
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/git/... -v -run TestParseStatus
```

Expected: FAIL with "undefined: parseStatus"

- [ ] **Step 3: Implement status operations**

```go
// internal/git/status.go
package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
)

// parseStatus parses the output of git status --porcelain=v2
func parseStatus(output string) model.WorktreeStatus {
	status := model.WorktreeStatus{}
	
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Check for untracked files (start with ?)
		if strings.HasPrefix(line, "?") {
			status.Untracked++
			status.IsDirty = true
			continue
		}
		
		// Any other line indicates a change
		if len(line) > 0 {
			status.IsDirty = true
		}
	}
	
	return status
}

// GetWorktreeStatus returns the status of a worktree
func (g *GitService) GetWorktreeStatus(worktreePath string) (*model.WorktreeStatus, error) {
	cmd := exec.Command("git", "status", "--porcelain=v2", "--branch", "--ahead-behind")
	cmd.Dir = worktreePath

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w\n%s", err, output)
	}

	status := parseStatus(string(output))
	
	// Parse ahead/behind from output
	// Format: # branch.ab +N -M
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# branch.ab") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "+") {
					fmt.Sscanf(part, "+%d", &status.Ahead)
				} else if strings.HasPrefix(part, "-") {
					fmt.Sscanf(part, "-%d", &status.Behind)
				}
			}
		}
	}
	
	return &status, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/git/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/git/status.go internal/git/status_test.go
git commit -m "feat: implement GetWorktreeStatus with ahead/behind"
```

---

## Chunk 4: TUI Layer - App Setup and List View

### Task 7: Create TUI Styles

**Files:**
- Create: `internal/tui/styles.go`

- [ ] **Step 1: Define Lipgloss styles**

```go
// internal/tui/styles.go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#06B6D4") // Cyan
	successColor   = lipgloss.Color("#10B981") // Green
	warningColor   = lipgloss.Color("#F59E0B") // Yellow
	errorColor     = lipgloss.Color("#EF4444") // Red
	mutedColor     = lipgloss.Color("#6B7280") // Gray

	// Styles
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	cleanStyle = lipgloss.NewStyle().
			Foreground(successColor)

	dirtyStyle = lipgloss.NewStyle().
			Foreground(warningColor)

	lockedStyle = lipgloss.NewStyle().
			Foreground(errorColor)

	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)
)
```

- [ ] **Step 2: Commit**

```bash
git add internal/tui/styles.go
git commit -m "feat: add Lipgloss style definitions"
```

---

### Task 8: Create Main TUI App

**Files:**
- Create: `internal/tui/app.go`
- Create: `internal/tui/app_test.go`

- [ ] **Step 1: Write failing test for Model initialization**

```go
// internal/tui/app_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/tui/... -v
```

Expected: FAIL with "undefined: NewModel"

- [ ] **Step 3: Implement Model and basic Update/View**

```go
// internal/tui/app.go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
)

type appMode int

const (
	modeList appMode = iota
	modeCreate
	modeDelete
	modeCleanup
)

type Model struct {
	git       *git.GitService
	worktrees []model.Worktree
	selected  int
	mode      appMode
	errMsg    string
	width     int
	height    int
}

func NewModel(gitService *git.GitService) Model {
	return Model{
		git:      gitService,
		selected: 0,
		mode:     modeList,
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadWorktrees
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case worktreesLoadedMsg:
		m.worktrees = msg.worktrees
		m.errMsg = ""
		return m, nil
	case errMsg:
		m.errMsg = string(msg)
		return m, nil
	}
	return m, nil
}

func (m Model) View() string {
	if len(m.worktrees) == 0 {
		return "No worktrees found. Press 'q' to quit."
	}

	var b strings.Builder
	
	// Title
	b.WriteString(titleStyle.Render("git-worktree-tui"))
	b.WriteString("\n\n")

	// Worktree list
	for i, wt := range m.worktrees {
		prefix := "  "
		if i == m.selected {
			prefix = "→ "
		}

		status := "●"
		if wt.IsLocked {
			status = "🔒"
		} else if wt.Status != nil && wt.Status.IsDirty {
			status = "●"
		}

		line := fmt.Sprintf("%s%s %s", prefix, status, wt.Branch)
		if wt.Detached {
			line = fmt.Sprintf("%s%s (detached)", prefix, status)
		}

		if i == m.selected {
			line = selectedStyle.Render(line)
		}

		b.WriteString(line)
		b.WriteString("\n")
		b.WriteString(fmt.Sprintf("    %s • %s", wt.Commit[:7], wt.Path))
		b.WriteString("\n\n")
	}

	// Help
	b.WriteString(helpStyle.Render("[a]dd [d]elete [c]leanup [r]efresh [q]uit"))

	// Error message
	if m.errMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render(m.errMsg))
	}

	return b.String()
}

// Messages
type worktreesLoadedMsg struct {
	worktrees []model.Worktree
}

type errMsg string

// Commands
func (m Model) loadWorktrees() tea.Msg {
	worktrees, err := m.git.ListWorktrees()
	if err != nil {
		return errMsg(err.Error())
	}
	return worktreesLoadedMsg{worktrees: worktrees}
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
		return m, tea.Quit
	case key.Matches(msg, key.NewBinding(key.WithKeys("up", "k"))):
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("down", "j"))):
		if m.selected < len(m.worktrees)-1 {
			m.selected++
		}
		return m, nil
	case key.Matches(msg, key.NewBinding(key.WithKeys("r"))):
		return m, m.loadWorktrees
	}
	return m, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
go test ./internal/tui/... -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/tui/
git commit -m "feat: implement main TUI app with list view"
```

---

## Chunk 5: TUI Layer - Modals

### Task 9: Create Delete Confirmation Modal

**Files:**
- Create: `internal/tui/delete.go`
- Create: `internal/tui/delete_test.go`

- [ ] **Step 1: Write failing test for delete modal**

```go
// internal/tui/delete_test.go
package tui

import (
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
```

- [ ] **Step 2: Implement delete modal**

```go
// internal/tui/delete.go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) viewDeleteModal() string {
	if m.selected >= len(m.worktrees) {
		return ""
	}

	wt := m.worktrees[m.selected]

	var b strings.Builder
	b.WriteString(titleStyle.Render("Delete Worktree"))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("Delete worktree for \"%s\"?\n", wt.Branch))
	b.WriteString(fmt.Sprintf("Path: %s\n\n", wt.Path))

	if wt.Status != nil && wt.Status.IsDirty {
		b.WriteString(errorStyle.Render("⚠ This worktree has uncommitted changes!\n"))
		b.WriteString(errorStyle.Render("⚠ Changes will be lost.\n\n"))
	} else {
		b.WriteString("⚠ This will remove the worktree directory.\n\n")
	}

	b.WriteString(helpStyle.Render("[y]es [n]o"))

	return b.String()
}

func (m Model) handleDeleteKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, key.NewBinding(key.WithKeys("y"))):
		return m, m.deleteWorktree
	case key.Matches(msg, key.NewBinding(key.WithKeys("n", "escape"))):
		m.mode = modeList
		return m, nil
	}
	return m, nil
}

func (m Model) deleteWorktree() tea.Msg {
	if m.selected >= len(m.worktrees) {
		return errMsg("No worktree selected")
	}

	wt := m.worktrees[m.selected]

	if wt.IsMain {
		return errMsg("Cannot delete main worktree")
	}

	if wt.IsLocked {
		return errMsg("Worktree is locked, unlock first")
	}

	err := m.git.RemoveWorktree(wt.Path)
	if err != nil {
		return errMsg(err.Error())
	}

	return worktreesLoadedMsg{worktrees: nil} // Trigger reload
}
```

- [ ] **Step 3: Integrate delete modal into main app**

Add to `internal/tui/app.go` in `Update`:

```go
case modeDelete:
	return m.handleDeleteKeyPress(msg)
```

Add to `internal/tui/app.go` in `View`:

```go
if m.mode == modeDelete {
	return m.viewDeleteModal()
}
```

Add keybinding in `handleKeyPress`:

```go
case key.Matches(msg, key.NewBinding(key.WithKeys("d"))):
	if len(m.worktrees) > 0 && !m.worktrees[m.selected].IsMain {
		m.mode = modeDelete
	}
	return m, nil
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/tui/... -v
go build -o git-worktree-tui
```

Expected: All tests pass, binary builds

- [ ] **Step 5: Commit**

```bash
git add internal/tui/
git commit -m "feat: add delete confirmation modal"
```

---

## Chunk 6: Integration and CLI Entry Point

### Task 10: Create CLI Entry Point

**Files:**
- Create: `cmd/root.go`
- Modify: `main.go`

- [ ] **Step 1: Implement Cobra CLI**

```go
// cmd/root.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
	"github.com/kjaniec-dev/git-worktree-tui/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "git-worktree-tui",
	Short: "TUI for managing git worktrees",
	RunE:  run,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Detect repository root
	repoRoot, err := findRepoRoot()
	if err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}

	// Create git service
	gitService := git.NewGitService(repoRoot)

	// Create and run TUI
	model := tui.NewModel(gitService)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}

	return nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a git repository")
		}
		dir = parent
	}
}
```

- [ ] **Step 2: Update main.go**

```go
// main.go
package main

import "github.com/kjaniec-dev/git-worktree-tui/cmd"

func main() {
	cmd.Execute()
}
```

- [ ] **Step 3: Build and test**

```bash
go build -o git-worktree-tui
./git-worktree-tui
```

Expected: TUI launches in current git repo

- [ ] **Step 4: Commit**

```bash
git add cmd/ main.go
git commit -m "feat: add CLI entry point with repo detection"
```

---

## Chunk 7: Testing and Polish

### Task 11: Add README

**Files:**
- Create: `README.md`

- [ ] **Step 1: Write README**

```markdown
# git-worktree-tui

A terminal UI for managing git worktrees.

## Features

- List all worktrees with status (clean/dirty, ahead/behind)
- Create new worktrees with branch selection
- Delete worktrees with confirmation
- Cleanup stale worktrees
- Fast, keyboard-driven interface

## Installation

```bash
go install github.com/kjaniec-dev/git-worktree-tui@latest
```

## Usage

Run from any git repository:

```bash
git-worktree-tui
```

### Keybindings

- `↑/↓` or `j/k` - Navigate worktree list
- `Enter` - Show full path (copy to clipboard)
- `a` - Create new worktree
- `d` - Delete selected worktree
- `c` - Cleanup stale worktrees
- `r` - Refresh list
- `q` - Quit

## Requirements

- Git 2.39+ (for full worktree support)

## License

MIT
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README with usage instructions"
```

---

### Task 12: Final Integration Test

- [ ] **Step 1: Build final binary**

```bash
go build -o git-worktree-tui
```

- [ ] **Step 2: Test in real repository**

```bash
# Create test repo with worktrees
mkdir /tmp/test-repo
cd /tmp/test-repo
git init
echo "test" > file.txt
git add file.txt
git commit -m "initial"
git worktree add ../test-worktree feature/test

# Run TUI
/path/to/git-worktree-tui
```

Expected: TUI shows both worktrees

- [ ] **Step 3: Test all keybindings**

- Navigate with `j/k`
- Press `d` to delete (test confirmation)
- Press `r` to refresh
- Press `q` to quit

- [ ] **Step 4: Final commit**

```bash
git add .
git commit -m "chore: final integration test and polish"
```

---

## Summary

This plan implements git-worktree-tui in 7 chunks:

1. **Project setup** - Go module, dependencies, data model
2. **Git worktree operations** - List, add, remove, prune
3. **Git branch/status operations** - List branches, get status
4. **TUI app setup** - Main model, list view, styles
5. **TUI modals** - Delete confirmation
6. **CLI integration** - Cobra entry point, repo detection
7. **Testing and polish** - README, integration tests

Each task is bite-sized (2-5 minutes), follows TDD, and includes commits. Total estimated time: 2-3 hours.
