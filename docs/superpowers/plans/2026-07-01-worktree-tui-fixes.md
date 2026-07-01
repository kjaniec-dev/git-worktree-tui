# git-worktree-tui Fixes Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix bugs in the worktree TUI's create, delete, and cleanup flows so the tool is correct, safe for non-experts, and never silently changes semantics.

**Architecture:** Most bug fixes in the git layer (`internal/git/`) and TUI layer (`internal/tui/`). To make command construction and stale-classification unit-testable without mocking or shelling out, extract pure helper functions (`buildAddArgs`, `buildRemoveArgs`, `classifyStale`, `initialBase`) from the side-effecting methods. Methods call the helpers; tests assert on the helpers. No service-interface refactor for the TUI — `Model` keeps using `*git.GitService` directly.

**Tech Stack:** Go 1.26, Cobra CLI, Charmbracelet Bubble Tea (TUI), Lipgloss (styling). Module path `github.com/kjaniec-dev/git-worktree-tui`.

**Spec:** `docs/superpowers/specs/2026-07-01-worktree-tui-fixes-design.md`

---

## File map

- Modify: `internal/git/branch.go` — `BranchExists` → local-branch only (`refs/heads/<branch>`); surface real errors (distinguish nonzero exit from infra failure).
- Modify: `internal/git/worktree.go` — add `buildAddArgs`, `buildRemoveArgs`; rewrite `AddWorktree` (four-case matrix + path-collision pre-check); change `RemoveWorktree` signature to `(path string, force bool) error`.
- Modify: `internal/tui/app.go` — `NewModel` uses `initialBase` helper for `baseIndex` sync; clear `errMsg` on entering delete; guard navigation against empty list.
- Modify: `internal/tui/create.go` — Base field becomes selector-only (remove free-text/backspace on `fieldBase`).
- Modify: `internal/tui/delete.go` — modal shows force/normal confirm; `y` passes `force` based on dirty; uses new `RemoveWorktree` signature.
- Modify: `internal/tui/cleanup.go` — add `classifyStale`; `findStaleWorktrees` excludes detached + dirty; Enter handler accumulates errors, doesn't abort, refreshes on completion.
- Create: none (no new files).
- Modify tests:
  - `internal/git/branch_test.go` — `TestBranchExists` (local/tag/missing) against a temp repo.
  - `internal/git/worktree_test.go` — `TestBuildAddArgs`, `TestBuildRemoveArgs`, update `TestAddWorktreeCommand`/`TestRemoveWorktreeCommand` to new signatures, `TestAddWorktreePathCollision` (temp repo).
  - `internal/tui/app_test.go` — `TestInitialBase` (baseIndex sync).
  - `internal/tui/create_test.go` — `TestBaseFieldSelectorOnly` (↑/↓ sync, runes/backspace ignored on Base).
  - `internal/tui/cleanup_test.go` — `TestClassifyStale` (excludes main/locked/detached/dirty).
  - `internal/tui/delete_test.go` — `TestDeleteModalForceConfirm`.

---

## Chunk 1: git layer — BranchExists, AddWorktree matrix, RemoveWorktree force

### Task 1: BranchExists → local-branch only, surface real errors

**Files:**
- Modify: `internal/git/branch.go:40-44`
- Test: `internal/git/branch_test.go`

- [ ] **Step 1: Write the failing tests** (append to `internal/git/branch_test.go`)

```go
func TestBranchExists(t *testing.T) {
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
```

Add imports `"os"`, `"os/exec"` to `branch_test.go`.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/git/... -run TestBranchExists -v`
Expected: FAIL — `v1` returns true (matches tag) instead of false (current `rev-parse --verify <branch>` matches tags).

- [ ] **Step 3: Implement BranchExists**

Replace the body of `BranchExists` in `internal/git/branch.go`:

```go
// BranchExists reports whether a *local branch* named <branch> exists.
// Tags and other refs do not count. A nonzero exit from rev-parse means
// the branch does not exist (not an error). Real failures (timeout, git
// missing) are surfaced as errors.
func (g *GitService) BranchExists(branch string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), g.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = g.RepoRoot

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil // branch not found
		}
		return false, fmt.Errorf("failed to verify branch %q: %w", branch, err)
	}
	return true, nil
}
```

Add imports to `branch.go`: `"context"`, `"errors"`, `"os/exec"` (some already present: `fmt`, `strings`).

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/git/... -run TestBranchExists -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/git/branch.go internal/git/branch_test.go
git commit -m "fix: BranchExists checks local branches only and surfaces real errors"
```

---

### Task 2: AddWorktree four-case matrix via buildAddArgs

**Files:**
- Modify: `internal/git/worktree.go` (add `buildAddArgs`; rewrite `AddWorktree` core logic)
- Test: `internal/git/worktree_test.go`

- [ ] **Step 1: Write the failing tests** (append to `internal/git/worktree_test.go`)

```go
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/git/... -run TestBuildAddArgs -v`
Expected: FAIL — `undefined: buildAddArgs`.

- [ ] **Step 3: Implement buildAddArgs and rewrite AddWorktree**

In `internal/git/worktree.go`, add (above `AddWorktree`):

```go
// buildAddArgs constructs the `git worktree add` arguments for the four
// (createBranch, branchExists) cases. Returns an error for the two invalid
// combinations without producing args (no git invocation intended).
func buildAddArgs(path, branch, base string, createBranch, branchExists bool) ([]string, error) {
	switch {
	case createBranch && branchExists:
		return nil, fmt.Errorf("branch %q already exists; uncheck 'create new branch' to check it out", branch)
	case !createBranch && !branchExists:
		return nil, fmt.Errorf("branch %q does not exist; check 'create new branch' or select an existing branch", branch)
	case createBranch && !branchExists:
		return []string{"worktree", "add", "-b", branch, path, base}, nil
	default: // !createBranch && branchExists
		return []string{"worktree", "add", path, branch}, nil
	}
}
```

Rewrite `AddWorktree` to use the matrix (full rewrite in Task 4 after collision check; for now only swap the args-construction part):

```go
func (g *GitService) AddWorktree(path, branch, base string, createBranch bool) error {
	branchExists, err := g.BranchExists(branch)
	if err != nil {
		return fmt.Errorf("failed to check branch: %w", err)
	}

	args, err := buildAddArgs(path, branch, base, createBranch, branchExists)
	if err != nil {
		return err
	}

	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", parentDir, err)
	}

	output, err := g.runGitCommand(args...)
	if err != nil {
		return fmt.Errorf("failed to add worktree: %w", err)
	}

	_ = output
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/git/... -run TestBuildAddArgs -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/git/worktree.go internal/git/worktree_test.go
git commit -m "fix: AddWorktree uses explicit four-case matrix with friendly errors"
```

---

### Task 3: AddWorktree path-collision pre-check

**Files:**
- Modify: `internal/git/worktree.go` (`AddWorktree`)
- Test: `internal/git/worktree_test.go`

- [ ] **Step 1: Write the failing test** (append)

```go
func TestAddWorktreePathCollision(t *testing.T) {
	repo := t.TempDir()
	if out, err := exec.Command("git", "-C", repo, "init").CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", repo, "config", "user.email", "t@t").CombinedOutput(); err != nil {
		t.Fatalf("config email: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", repo, "config", "user.name", "t").CombinedOutput(); err != nil {
		t.Fatalf("config name: %v\n%s", err, out)
	}
	if out, err := exec.Command("git", "-C", repo, "commit", "--allow-empty", "-m", "init").CombinedOutput(); err != nil {
		t.Fatalf("commit: %v\n%s", err, out)
	}

	g := NewGitService(repo)
	collidePath := filepath.Join(t.TempDir(), "already-here")
	if err := os.MkdirAll(collidePath, 0755); err != nil {
		t.Fatalf("mkdir collide: %v", err)
	}
	err := g.AddWorktree(collidePath, "feat-x", "main", true)
	if err == nil {
		t.Fatal("expected collision error, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected friendly 'already exists' message, got: %v", err)
	}
}
```

Add imports `"os/exec"`, `"path/filepath"`, `"strings"` as needed (dedupe with existing `"os"`, `"path/filepath"`).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/git/... -run TestAddWorktreePathCollision -v`
Expected: FAIL — error is a raw git message (does not contain "already exists"), or no error (git refuses but with different wording).

- [ ] **Step 3: Insert the path-collision check** into `AddWorktree`, after `buildAddArgs` and before `runGitCommand` (spec: "immediately before the runGitCommand"):

```go
	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", parentDir, err)
	}

	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("path %q already exists", path)
	}

	output, err := g.runGitCommand(args...)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/git/... -run TestAddWorktreePathCollision -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/git/worktree.go internal/git/worktree_test.go
git commit -m "fix: AddWorktree pre-checks target path collision with friendly message"
```

---

### Task 4: RemoveWorktree force parameter and buildRemoveArgs

**Files:**
- Modify: `internal/git/worktree.go:152-161`
- Modify: `internal/tui/delete.go:58` (call site) — done in Chunk 3; to keep build green, update here with `false` and refine in Chunk 3
- Test: `internal/git/worktree_test.go`

- [ ] **Step 1: Write the failing tests** (append)

```go
func TestBuildRemoveArgs(t *testing.T) {
	if got := buildRemoveArgs("/p", false); !equalSlices(got, []string{"worktree", "remove", "/p"}) {
		t.Errorf("no-force = %v", got)
	}
	if got := buildRemoveArgs("/p", true); !equalSlices(got, []string{"worktree", "remove", "--force", "/p"}) {
		t.Errorf("force = %v", got)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/git/... -run TestBuildRemoveArgs -v`
Expected: FAIL — `undefined: buildRemoveArgs`.

- [ ] **Step 3: Implement buildRemoveArgs and change RemoveWorktree signature**

In `internal/git/worktree.go`:

```go
// buildRemoveArgs constructs `git worktree remove` args, including --force
// when force is true.
func buildRemoveArgs(path string, force bool) []string {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	return append(args, path)
}

// RemoveWorktree removes a worktree. When force is true, passes --force so
// git removes worktrees with untracked/modified files.
func (g *GitService) RemoveWorktree(path string, force bool) error {
	output, err := g.runGitCommand(buildRemoveArgs(path, force)...)
	if err != nil {
		return fmt.Errorf("failed to remove worktree %s: %w", path, err)
	}
	_ = output
	return nil
}
```

Update the `internal/tui/delete.go:58` call site now to keep the build green (detailed dirty logic arrives in Chunk 3):

```go
err := m.git.RemoveWorktree(wt.Path, false)
```

Update the `internal/tui/cleanup.go:76` call site (now always non-dirty, force=false):

```go
err := m.git.RemoveWorktree(wt.Path, false)
```

Update existing smoke test `TestRemoveWorktreeCommand` in `internal/git/worktree_test.go:89-95`:

```go
func TestRemoveWorktreeCommand(t *testing.T) {
	g := NewGitService("/tmp/nonexistent-repo-12345")
	if err := g.RemoveWorktree("/tmp/nonexistent-worktree", false); err == nil {
		t.Error("Expected error when worktree doesn't exist (force=false)")
	}
	if err := g.RemoveWorktree("/tmp/nonexistent-worktree", true); err == nil {
		t.Error("Expected error when worktree doesn't exist (force=true)")
	}
}
```

- [ ] **Step 4: Run tests and build to verify they pass**

Run: `go test ./internal/git/... -run "TestBuildRemoveArgs|TestRemoveWorktreeCommand" -v` && `go build ./...`
Expected: PASS + build OK.

- [ ] **Step 5: Commit**

```bash
git add internal/git/worktree.go internal/git/worktree_test.go internal/tui/delete.go internal/tui/cleanup.go
git commit -m "fix: RemoveWorktree gains force param via buildRemoveArgs; update call sites"
```

---

## Chunk 2: TUI create form fixes

### Task 5: NewModel baseIndex sync via initialBase helper

**Files:**
- Modify: `internal/tui/app.go:34-58`
- Test: `internal/tui/app_test.go`

- [ ] **Step 1: Write the failing test** (append to `internal/tui/app_test.go`)

```go
func TestInitialBase(t *testing.T) {
	tests := []struct {
		name     string
		branches []string
		wantBase string
		wantIdx  int
	}{
		{"empty -> main default", []string{}, "main", 0},
		{"main at 0", []string{"main", "develop"}, "main", 0},
		{"main not at 0 -> index of main", []string{"develop", "main", "feat"}, "main", 1},
		{"master preferred", []string{"develop", "master"}, "master", 1},
		{"no main/master -> first branch", []string{"develop", "feat"}, "develop", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, idx := initialBase(tt.branches)
			if base != tt.wantBase {
				t.Errorf("base = %q, want %q", base, tt.wantBase)
			}
			if idx != tt.wantIdx {
				t.Errorf("idx = %d, want %d", idx, tt.wantIdx)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/... -run TestInitialBase -v`
Expected: FAIL — `undefined: initialBase`.

- [ ] **Step 3: Implement initialBase and use it in NewModel**

In `internal/tui/app.go`, add (above `NewModel`):

```go
// initialBase selects the default base branch from the local branch list and
// returns its index so the create-form Base selector and the displayed base
// stay in sync. Prefers "main" then "master"; falls back to the first branch
// (index 0) or the literal "main" when the list is empty.
func initialBase(branches []string) (base string, index int) {
	base = "main"
	if len(branches) == 0 {
		return
	}
	base = branches[0]
	for i, b := range branches {
		if b == "main" || b == "master" {
			return b, i
		}
	}
	return // not found: first branch, index 0
}
```

Refactor `NewModel` to use it. Replace lines 34-58 of `app.go`:

```go
func NewModel(gitService *git.GitService) Model {
	branches, _ := gitService.ListBranches()
	baseBranch, baseIndex := initialBase(branches)

	return Model{
		git:      gitService,
		selected: 0,
		mode:     modeList,
		create: createModel{
			branches:     branches,
			baseBranch:   baseBranch,
			baseIndex:    baseIndex,
			createBranch: true,
			location:     "inside",
		},
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/... -run TestInitialBase -v`
Expected: PASS. (`TestNewModel` still passes since baseIndex of empty list is 0.)

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "fix: NewModel syncs baseIndex with baseBranch via initialBase helper"
```

---

### Task 6: Base field becomes selector-only

**Files:**
- Modify: `internal/tui/create.go` (`handleCreateKeyPress`: KeyBackspace, KeyRunes)
- Test: `internal/tui/create_test.go`

- [ ] **Step 1: Write the failing tests** (append to `internal/tui/create_test.go`)

```go
func TestBaseFieldSelectorOnly(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"))
	m.mode = modeCreate
	m.create.branches = []string{"develop", "main", "feat"}
	m.create.baseBranch = "main"
	m.create.baseIndex = 1
	m.create.currentField = fieldBase

	// Down: baseIndex advances and baseBranch tracks branches[baseIndex]
	m = sendCreate(m, tea.KeyDown).(Model)
	if m.create.baseIndex != 2 || m.create.baseBranch != "feat" {
		t.Errorf("after Down: idx=%d base=%q; want 2,feat", m.create.baseIndex, m.create.baseBranch)
	}
	// Up: back
	m = sendCreate(m, tea.KeyUp).(Model)
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
	m = sendCreate(m, tea.KeyBackspace).(Model)
	if m.create.baseBranch != before {
		t.Errorf("Base accepted backspace: %q changed to %q", before, m.create.baseBranch)
	}
}
```

Add helper at top of `create_test.go` (or a shared `test_helpers_test.go` in package `tui`):

```go
func sendCreate(m Model, msg tea.KeyMsg) tea.Model { return m.handleCreateKeyPress(msg) }

func teaKeyPress(s string) tea.KeyMsg {
	if len(s) == 1 {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{s}}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}
```

(Note: `tea.KeyDown`/`tea.KeyUp`/`tea.KeyBackspace` are consts from bubbletea.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/... -run TestBaseFieldSelectorOnly -v`
Expected: FAIL — typing `x` on Base appends to `baseBranch` (current dual-mode behavior); backspace deletes.

- [ ] **Step 3: Remove free-text handling for fieldBase**

In `internal/tui/create.go` `handleCreateKeyPress`:

For `tea.KeyBackspace` (lines 132-138), remove the `else if m.create.currentField == fieldBase ...` branch:

```go
	case tea.KeyBackspace:
		if m.create.currentField == fieldBranch && len(m.create.branchName) > 0 {
			m.create.branchName = m.create.branchName[:len(m.create.branchName)-1]
		}
		return m, nil
```

For `tea.KeyRunes` (lines 144-158), remove the `else if m.create.currentField == fieldBase` branch; keep branch and location:

```go
	case tea.KeyRunes:
		if m.create.currentField == fieldBranch {
			m.create.branchName += string(msg.Runes)
		} else if m.create.currentField == fieldLocation {
			if msg.String() == "l" {
				if m.create.location == "inside" {
					m.create.location = "outside"
				} else {
					m.create.location = "inside"
				}
			}
		}
		return m, nil
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/... -run TestBaseFieldSelectorOnly -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/create.go internal/tui/create_test.go
git commit -m "fix: create form Base field is selector-only (no free-text)"
```

---

### Task 7: Create Enter with existing branch shows friendly error

**Files:**
- Modify: none (covered by Tasks 2-3); behavior test only
- Test: `internal/tui/create_test.go`

- [ ] **Step 1: Write the test** (append)

This path is hard to drive end-to-end without a real repo (the TUI calls `m.git.AddWorktree` which shells out). Per the spec's testing strategy, the friendly-error matrix is covered by `TestBuildAddArgs` (git layer). Add a light assertion that the create handler sets `errMsg` and stays in `modeCreate` when `AddWorktree` returns any error:

```go
func TestCreateEnterErrorStaysInCreate(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/nonexistent-repo-12345"))
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
```

- [ ] **Step 2: Run test to verify it passes** (the fix is already in place from Tasks 2-3; this is a regression guard)

Run: `go test ./internal/tui/... -run TestCreateEnterErrorStaysInCreate -v`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/create_test.go
git commit -m "test: create Enter with error stays in create mode and shows message"
```

---

## Chunk 3: TUI delete flow

### Task 8: Delete modal shows force/normal confirm and passes force

**Files:**
- Modify: `internal/tui/delete.go`
- Test: `internal/tui/delete_test.go`

- [ ] **Step 1: Write the failing test** (append to `internal/tui/delete_test.go`)

```go
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
	if view := m.View(); !strings.Contains(view, "force-remove") || !strings.Contains(view, "changes will be lost") {
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
	m, _ = m.Update(teaKeyPress("d")).(Model) // via Update, not handleKeyPress direct, to exercise the 'd' branch
	if m.errMsg != "" {
		t.Errorf("errMsg should be cleared on entering delete, got %q", m.errMsg)
	}
	if m.mode != modeDelete {
		t.Errorf("mode = %v, want modeDelete", m.mode)
	}
}
```

Add import `"strings"` and `"github.com/kjaniec-dev/git-worktree-tui/internal/model"` (already present), and the `teaKeyPress` helper (reuse the one from Task 6; if duplicated, put in `test_helpers_test.go`).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/... -run "TestDeleteModalForceConfirm|TestDeleteClearsErrMsgOnEntry" -v`
Expected: FAIL — clean modal shows `[y]es [n]o` (passes current test) but dirty modal also shows `[y]es [n]o` not `force-remove`; errMsg not cleared on `d`.

- [ ] **Step 3: Update the delete modal view and handler**

In `internal/tui/delete.go`, change the help line (lines 31) to be conditional:

```go
	confirmHint := "[y]es [n]o"
	if wt.Status != nil && wt.Status.IsDirty {
		confirmHint = "[y] force-remove [n]o"
	}
	b.WriteString(helpStyle.Render(confirmHint))
```

In the `y` branch of `handleDeleteKeyPress` (around line 58), compute force:

```go
		force := wt.Status != nil && wt.Status.IsDirty
		err := m.git.RemoveWorktree(wt.Path, force)
		if err != nil {
			m.errMsg = err.Error()
			return m, nil
		}
```

- [ ] **Step 4: Clear errMsg when entering delete mode**

In `internal/tui/app.go` `handleKeyPress`, `case "d":` (around lines 232-241), add `m.errMsg = ""` after the IsMain guard / before setting `m.mode = modeDelete`:

```go
		case "d":
			if len(m.worktrees) > 0 && m.selected < len(m.worktrees) {
				wt := m.worktrees[m.selected]
				if wt.IsMain {
					m.errMsg = "Cannot delete main worktree"
					return m, nil
				}
				m.errMsg = ""
				m.mode = modeDelete
			}
			return m, nil
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tui/... -run "TestDeleteModalForceConfirm|TestDeleteClearsErrMsgOnEntry" -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/delete.go internal/tui/app.go internal/tui/delete_test.go
git commit -m "fix: delete modal uses --force on dirty worktree confirm; clear errMsg on entry"
```

---

## Chunk 4: TUI cleanup flow

### Task 9: classifyStale helper excludes main/locked/detached/dirty; findStaleWorktrees uses it

**Files:**
- Modify: `internal/tui/cleanup.go` (add `classifyStale`; rewrite `findStaleWorktrees`)
- Test: `internal/tui/cleanup_test.go`

- [ ] **Step 1: Write the failing test** (append to `internal/tui/cleanup_test.go`)

```go
func TestClassifyStale(t *testing.T) {
	branchSet := map[string]bool{"main": true, "develop": true}
	wts := []model.Worktree{
		{Path: "/main", Branch: "main", IsMain: true},                       // excluded: main
		{Path: "/locked", Branch: "develop", IsLocked: true},                 // excluded: locked
		{Path: "/detached", Branch: "(detached)", Detached: true},            // excluded: detached
		{Path: "/dirty", Branch: "develop", Status: &model.WorktreeStatus{IsDirty: true}}, // excluded: dirty
		{Path: "/stale", Branch: "gone-branch"},                             // stale: branch not in set
		{Path: "/clean", Branch: "develop"},                                 // not stale: branch in set
	}
	got := classifyStale(wts, branchSet)
	want := []int{4}
	if !equalInts(got, want) {
		t.Errorf("classifyStale = %v, want %v", got, want)
	}
}

func equalInts(a, b []int) bool {
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
```

(also ensure existing `TestCleanupModal` still compiles.)

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/... -run TestClassifyStale -v`
Expected: FAIL — `undefined: classifyStale`.

- [ ] **Step 3: Implement classifyStale and rewrite findStaleWorktrees**

In `internal/tui/cleanup.go`, add (above `findStaleWorktrees`):

```go
// classifyStale returns the indices of worktrees that are stale: non-main,
// non-locked, non-detached, non-dirty, and whose branch is not a local
// branch in branchSet.
func classifyStale(wts []model.Worktree, branchSet map[string]bool) []int {
	var stale []int
	for i, wt := range wts {
		if wt.IsMain || wt.IsLocked || wt.Detached {
			continue
		}
		if wt.Status != nil && wt.Status.IsDirty {
			continue
		}
		if !branchSet[wt.Branch] {
			stale = append(stale, i)
		}
	}
	return stale
}
```

Rewrite `findStaleWorktrees`:

```go
func (m *Model) findStaleWorktrees() {
	branches, err := m.git.ListBranches()
	if err != nil {
		return
	}

	branchSet := make(map[string]bool)
	for _, b := range branches {
		branchSet[b] = true
	}

	m.cleanup.staleWorktrees = classifyStale(m.worktrees, branchSet)
	m.cleanup.selected = make([]bool, len(m.cleanup.staleWorktrees))
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/... -run "TestClassifyStale|TestCleanupModal" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/cleanup.go internal/tui/cleanup_test.go
git commit -m "fix: cleanup excludes detached and dirty worktrees from stale set"
```

---

### Task 10: Cleanup Enter handler accumulates errors and refreshes

**Files:**
- Modify: `internal/tui/cleanup.go:51-127` (`handleCleanupKeyPress` Enter branch)
- Test: `internal/tui/cleanup_test.go`

- [ ] **Step 1: Write the failing test** (append)

```go
func TestCleanupEnterAccumulatesErrors(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/nonexistent-repo-12345"))
	// Two stale worktrees pointing at nonexistent paths -> both RemoveWorktree calls error.
	m.worktrees = []model.Worktree{
		{Path: "/tmp/no-such-1", Branch: "gone1"},
		{Path: "/tmp/no-such-2", Branch: "gone2"},
	}
	m.cleanup.staleWorktrees = []int{0, 1}
	m.cleanup.selected = []bool{true, true}
	m.cleanup.currentIndex = 0
	m.mode = modeCleanup

	out, cmd := m.handleCleanupKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	mm := out.(Model)

	// Should have returned to list mode with a refresh command regardless of failures.
	if mm.mode != modeList {
		t.Errorf("mode = %v, want modeList", mm.mode)
	}
	if cmd == nil {
		t.Error("expected a loadWorktrees cmd even after partial failures")
	}
	if mm.errMsg == "" {
		t.Error("expected accumulated error message from failed removals")
	}
	if !strings.Contains(mm.errMsg, "gone1") || !strings.Contains(mm.errMsg, "gone2") {
		t.Errorf("expected both failed paths in errMsg, got: %s", mm.errMsg)
	}
}
```

Add import `"strings"` to `cleanup_test.go` if not present.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/... -run TestCleanupEnterAccumulatesErrors -v`
Expected: FAIL — handler returns on first error (only `gone1` in errMsg, mode stays modeCleanup, no refresh).

- [ ] **Step 3: Rewrite the Enter branch of handleCleanupKeyPress**

In `internal/tui/cleanup.go`, replace the `case tea.KeyEnter:` block (lines 56-84):

```go
	case tea.KeyEnter:
		if len(m.cleanup.staleWorktrees) == 0 {
			m.mode = modeList
			return m, m.loadWorktrees
		}
		var errs []string
		for i, idx := range m.cleanup.staleWorktrees {
			if !m.cleanup.selected[i] {
				continue
			}
			wt := m.worktrees[idx]
			if wt.IsMain || wt.IsLocked {
				continue
			}
			if wt.Status != nil && wt.Status.IsDirty {
				continue
			}
			if err := m.git.RemoveWorktree(wt.Path, false); err != nil {
				errs = append(errs, fmt.Sprintf("failed to remove %s: %v", wt.Path, err))
			}
		}
		m.mode = modeList
		if len(errs) > 0 {
			m.errMsg = strings.Join(errs, "; ")
		} else {
			m.errMsg = ""
		}
		return m, m.loadWorktrees
```

Add import `"strings"` to `cleanup.go`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/... -run "TestCleanupEnterAccumulatesErrors|TestClassifyStale|TestCleanupModal" -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/cleanup.go internal/tui/cleanup_test.go
git commit -m "fix: cleanup accumulates removal errors and refreshes the list"
```

---

## Chunk 5: TUI list safety + final verification

### Task 11: Guard list navigation against empty worktree list

**Files:**
- Modify: `internal/tui/app.go` (`handleKeyPress`: `j`, `k`, KeyUp, KeyDown)
- Test: `internal/tui/app_test.go`

- [ ] **Step 1: Write the failing test** (append)

```go
func TestEmptyListNavigationNoOp(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"))
	m.worktrees = nil
	m.selected = 0
	m.mode = modeList

	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'j'}},
		{Type: tea.KeyRunes, Runes: []rune{'k'}},
		{Type: tea.KeyDown},
		{Type: tea.KeyUp},
	} {
		out, _ := m.handleKeyPress(msg)
		mm := out.(Model)
		if mm.selected != 0 {
			t.Errorf("after %v on empty list, selected = %d, want 0", msg, mm.selected)
		}
	}
}
```

Add import `tea "github.com/charmbracelet/bubbletea"` to `app_test.go` if not present.

- [ ] **Step 2: Run test** (may already pass due to `< len-1` guards; verify deterministic behavior)

Run: `go test ./internal/tui/... -run TestEmptyListNavigationNoOp -v`
Expected:_PASS (guards already prevent increment beyond empty). If PASS, proceed to Step 3 to make the behavior explicit. If FAIL, fix first.

- [ ] **Step 3: Add explicit empty-list guard in handleKeyPress**

In `internal/tui/app.go` `handleKeyPress`, add an early return for navigation keys when the list is empty. In `case tea.KeyRunes` for `j`/`k`, and in `tea.KeyDown`/`tea.KeyUp`, prefix with:

```go
		case "j":
			if len(m.worktrees) == 0 {
				return m, nil
			}
			if m.selected < len(m.worktrees)-1 {
				m.selected++
			}
			return m, nil
		case "k":
			if len(m.worktrees) == 0 {
				return m, nil
			}
			if m.selected > 0 {
				m.selected--
			}
			return m, nil
```

And for `tea.KeyDown` / `tea.KeyUp`:

```go
	case tea.KeyDown:
		if len(m.worktrees) == 0 {
			return m, nil
		}
		if m.selected < len(m.worktrees)-1 {
			m.selected++
		}
		return m, nil
	case tea.KeyUp:
		if len(m.worktrees) == 0 {
			return m, nil
		}
		if m.selected > 0 {
			m.selected--
		}
		return m, nil
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/... -run TestEmptyListNavigationNoOp -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "fix: guard list navigation against empty worktree list"
```

---

### Task 12: Full suite verification

**Files:** none

- [ ] **Step 1: Run the full test suite**

Run: `go test ./... -v`
Expected: ALL PASS. Note any pre-existing failures separately — do NOT fix them (out of scope).

- [ ] **Step 2: Run vet and build**

Run: `go vet ./... && go build ./...`
Expected: exit 0, no output.

- [ ] **Step 3: Manual QA (per spec) in a scratch git repo**

Create a scratch repo with at least: a main worktree, one clean feature worktree, one dirty feature worktree, one detached-HEAD worktree, and one worktree on a branch that has been deleted.

- Delete the clean feature worktree via `d` → `y`: modal says `[y]es [n]o`; removal succeeds.
- Delete the dirty feature worktree via `d` → `y`: modal says `[y] force-remove [n]o` and warns changes lost; removal succeeds (uses `--force`).
- Open cleanup via `c`: the detached worktree does NOT appear in the stale list; the dirty worktree does NOT appear; the deleted-branch worktree does appear.
- Select the stale entry, press Enter: removed; list refreshes; no error shown.
- (If feasible) mark a stale entry's path as non-removable and press Enter with multiple selected: the rest still get removed, list refreshes, accumulated error shows at bottom.

- [ ] **Step 4: Commit any test-only touch-ups** (if none, skip)

```bash
git add -A && git commit -m "test: full suite green for worktree-tui fixes" --allow-empty
```

---

## Out of scope (per spec)

- No new modify/rename/move worktree flow.
- No branch-name legality validation beyond the matrix/collision checks here.
- No remote-tracking branch support in create or cleanup.
- No styling/layout changes.
- No `GitService` interface extraction or TUI mock injection (dirty-delete path verified via git-layer arg tests + manual QA).