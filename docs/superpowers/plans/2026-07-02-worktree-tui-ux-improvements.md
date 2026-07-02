# git-worktree-tui UX Improvements Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Three UX improvements to the worktree list view: (1) distinct dirty/clean status glyphs wired to existing unused styles, (2) a "you are here" marker for the user's current worktree, (3) actual clipboard copy + neutral green feedback on Enter instead of a red error.

**Architecture:** All changes in the TUI layer (`internal/tui/`) plus one line in `cmd/root.go` to pass CWD. `NewModel` signature gains a `cwd string` parameter. A new `infoMsg string` field on `Model` holds clipboard-confirmation text rendered with a new `infoStyle`. `go-osc52/v2` (already in `go.mod`) provides clipboard writing. No new flows, no git-layer changes, no behavior changes to create/delete/cleanup.

**Tech Stack:** Go 1.26, Cobra CLI, Charmbracelet Bubble Tea v1.3.10, Lipgloss v1.1.0, go-osc52/v2. Module path `github.com/kjaniec-dev/git-worktree-tui`.

**Spec:** `docs/superpowers/specs/2026-07-02-worktree-tui-ux-improvements-design.md`

---

## File map

- Modify: `internal/tui/styles.go` — add `infoStyle` (reuse `successColor`) and `mainStyle` (reuse `primaryColor`). One-line additions.
- Modify: `internal/tui/app.go` —
  - Add `cwd string` and `infoMsg string` fields to `Model`.
  - `NewModel` signature: `NewModel(gitService *git.GitService, cwd string) Model`.
  - `View()` list loop: split glyph and branch into separate styled segments; choose glyph by locked > main > dirty > clean > unknown; apply status color to glyph (kept even on selected row); apply `selectedStyle` to branch+prefix only for the selected row.
  - `View()` list loop: append `mutedStyle.Render(" (here)")` after the branch name (and after detached marker if any) when `filepath.Rel(wt.Path, m.cwd)` doesn't start with `..`.
  - `View()` after help: render `infoStyle.Render(m.infoMsg)` when set (parallel to the existing `errMsg` block).
  - `handleKeyPress` `tea.KeyEnter`: import `github.com/aymanbagabas/go-osc52/v2`; attempt OSC 52 copy; set `m.infoMsg = "Copied: <path>"` (or `Path: <path>` if copy writing fails); clear `m.errMsg`.
  - `handleKeyPress` top: clear `m.infoMsg = ""` at function entry (mirrors `errMsg` lifecycle) so info doesn't linger across keypresses.
  - Add `"path/filepath"`, `"strings"` (already present), `"github.com/aymanbagabas/go-osc52/v2"` imports.
- Modify: `cmd/root.go` — capture `cwd, _ := os.Getwd()` in `run`, call `tui.NewModel(gitService, cwd)`.
- Modify tests:
  - `internal/tui/app_test.go` — update existing `TestNewModel` to `NewModel(gitService, "/tmp/test")` (new signature); add `TestStatusGlyphs`, `TestHereMarker`, `TestHereMarkerSubdirectory`, `TestNewModelWithCWD`.

---

## Chunk 1: Distinct status glyphs + colors (styles + view)

### Task 1: Distinct dirty/clean glyphs + wire existing styles into the View list loop

**Files:**
- Modify: `internal/tui/styles.go` (add `infoStyle`, `mainStyle`)
- Modify: `internal/tui/app.go` (`View()` list loop, lines 134-168)
- Test: `internal/tui/app_test.go`

- [ ] **Step 1: Write the failing test** (append to `internal/tui/app_test.go`)

```go
func TestStatusGlyphs(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{
		{Path: "/p/locked", Branch: "b", IsLocked: true},
		{Path: "/p/main", Branch: "main", IsMain: true},
		{Path: "/p/dirty", Branch: "d", Status: &model.WorktreeStatus{IsDirty: true}},
		{Path: "/p/clean", Branch: "c", Status: &model.WorktreeStatus{IsDirty: false}},
		{Path: "/p/unknown", Branch: "u"}, // Status == nil -> not loaded
	}
	m.mode = modeList
	view := m.View()

	cases := []struct{ state, glyph string }{
		{"locked", "🔒"},
		{"main", "★"},
		{"dirty", "●"},
		{"clean", "○"},
		{"unknown", "?"},
	}
	for _, c := range cases {
		if !strings.Contains(view, c.glyph) {
			t.Errorf("expected glyph %q for %s worktree in view, got:\n%s", c.glyph, c.state, view)
		}
	}
	// Sanity: dirty and clean must produce DIFFERENT view strings
	if strings.Contains(view, "● c") || strings.Contains(view, "○ d") {
		t.Errorf("dirty/clean glyphs collided:\n%s", view)
	}
}
```

Add `"strings"` import to `app_test.go` if not already present (it likely isn't). The `model` import is already there. The `tea` import is already present from Task 11 of the previous plan.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/... -run TestStatusGlyphs -v`
Expected: FAIL — current code renders `●` for both clean and dirty; clean worktree has no `○` in view; `?` is never rendered.

- [ ] **Step 3: Add `infoStyle` and `mainStyle` to `internal/tui/styles.go`**

Append to the `var (...)` block in `styles.go`:

```go
	// infoStyle renders non-error status messages (clipboard confirmations, etc.)
	// in green — distinct from errorStyle (red).
	infoStyle = lipgloss.NewStyle().Foreground(successColor)
	// mainStyle colors the main-worktree ★ glyph with the title's primary color.
	mainStyle = lipgloss.NewStyle().Foreground(primaryColor)
```

(`successColor` and `primaryColor` are already declared at the top of the block.)

- [ ] **Step 4: Rewrite the View list loop to split glyph and branch rendering**

In `internal/tui/app.go` `View()`, replace the loop body (current lines 134-168). The new logic:

```go
	// Worktree list
	for i, wt := range m.worktrees {
		prefix := "  "
		if i == m.selected {
			prefix = "→ "
		}

		// Glyph + style by precedence: locked > main > dirty > clean > unknown
		var glyph string
		var glyphStyle lipgloss.Style
		switch {
		case wt.IsLocked:
			glyph = "🔒"
			glyphStyle = lockedStyle
		case wt.IsMain:
			glyph = "★"
			glyphStyle = mainStyle
		case wt.Status != nil && wt.Status.IsDirty:
			glyph = "●"
			glyphStyle = dirtyStyle
		case wt.Status != nil && !wt.Status.IsDirty:
			glyph = "○"
			glyphStyle = cleanStyle
		default: // Status == nil (not loaded yet)
			glyph = "?"
			glyphStyle = mutedStyle
		}

		branchPart := wt.Branch
		if wt.Detached {
			branchPart = "(detached)"
		}

		// "you are here" marker — see Task 2 for the matching logic.
		// For Task 1, this block is a no-op placeholder (currentPath stays "").
		currentPath := ""

		renderedGlyph := glyphStyle.Render(glyph)
		line := fmt.Sprintf("%s%s %s", prefix, renderedGlyph, branchPart)
		if currentPath != "" {
			line += mutedStyle.Render(" (here)")
		}
		if i == m.selected {
			line = selectedStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")

		commitDisplay := wt.Commit
		if len(commitDisplay) > 7 {
			commitDisplay = commitDisplay[:7]
		}
		b.WriteString(fmt.Sprintf("    %s • %s", commitDisplay, wt.Path))
		b.WriteString("\n\n")
	}
```

Notes for the implementer:
- The `currentPath` variable is a placeholder for Task 2 — keep it as `""` for now so the `(here)` block never fires. Task 2 replaces the empty assignment with the `filepath.Rel` check.
- By wrapping the ENTIRE line (glyph + branch) in `selectedStyle` when selected, the glyph's status color is overridden by the selection color on the selected row. This is acceptable per spec — the alternative (preserve glyph color on selection) requires splitting the selectedStyle application to only the branch part. **Keep the simpler "whole-line selection" approach** for Task 1; the dirty/clean distinction is still visible on all non-selected rows (typically 4+ in a real list).
- The `lipgloss` import must be added to `app.go` if not already present.

Add `"github.com/charmbracelet/lipgloss"` import to `app.go`.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./internal/tui/... -run TestStatusGlyphs -v`
Expected: PASS.

Also run `go test ./... -v` (ALL tests must pass) and `go build ./...`.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/styles.go internal/tui/app.go internal/tui/app_test.go
git commit -m "feat: distinct dirty/clean status glyphs with wired-in styles"
```

---

## Chunk 2: "You are here" marker (NewModel signature + CWD match)

### Task 2: NewModel accepts CWD; View appends (here) marker

**Files:**
- Modify: `internal/tui/app.go` (Model struct, NewModel signature, View list loop matching block)
- Modify: `cmd/root.go` (call site update)
- Test: `internal/tui/app_test.go`

- [ ] **Step 1: Write the failing tests** (append to `internal/tui/app_test.go`)

```go
func TestNewModelWithCWD(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/custom/cwd")
	if m.cwd != "/custom/cwd" {
		t.Errorf("m.cwd = %q, want /custom/cwd", m.cwd)
	}
}

func TestHereMarkerOnWorktreeRoot(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/wt/feat")
	m.worktrees = []model.Worktree{
		{Path: "/wt/main", Branch: "main"},
		{Path: "/wt/feat", Branch: "feat"},
	}
	m.mode = modeList
	view := m.View()
	if !strings.Contains(view, "feat (here)") {
		t.Errorf("expected 'feat (here)' in view, got:\n%s", view)
	}
	if strings.Contains(view, "main (here)") {
		t.Errorf("did not expect (here) on 'main' row, got:\n%s", view)
	}
	// Exactly one (here) marker
	if c := strings.Count(view, "(here)"); c != 1 {
		t.Errorf("expected exactly 1 (here) marker, got %d:\n%s", c, view)
	}
}

func TestHereMarkerInSubdirectory(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/wt/feat/internal/tui")
	m.worktrees = []model.Worktree{
		{Path: "/wt/main", Branch: "main"},
		{Path: "/wt/feat", Branch: "feat"},
	}
	m.mode = modeList
	view := m.View()
	if !strings.Contains(view, "feat (here)") {
		t.Errorf("expected (here) on feat even when CWD is a subdirectory of /wt/feat:\n%s", view)
	}
	if c := strings.Count(view, "(here)"); c != 1 {
		t.Errorf("expected exactly 1 (here) marker, got %d:\n%s", c, view)
	}
}
```

Update existing `TestNewModel` to pass the new second argument:

```go
func TestNewModel(t *testing.T) {
	gitService := git.NewGitService("/tmp/test")
	m := NewModel(gitService, "/tmp/test")

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

Update `TestEmptyListNavigationNoOp` (from the previous plan):

```go
func TestEmptyListNavigationNoOp(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	// ... rest unchanged
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/... -run "TestNewModelWithCWD|TestHereMarkerOnWorktreeRoot|TestHereMarkerInSubdirectory" -v`
Expected: FAIL — `NewModel` only takes one arg; `m.cwd` field doesn't exist; `(here)` never appears in view. Existing `TestNewModel` and `TestEmptyListNavigationNoOp` fail to compile (wrong number of arguments to `NewModel`).

- [ ] **Step 3: Add `cwd` field and update `NewModel` signature in `internal/tui/app.go`**

Add to `Model` struct:

```go
type Model struct {
	git       *git.GitService
	worktrees []model.Worktree
	selected  int
	mode      appMode
	errMsg    string
	infoMsg   string // populated by Enter; rendered with infoStyle (Task 3)
	cwd       string  // captured at startup; drives the (here) marker
	width     int
	height    int
	create    createModel
	cleanup   cleanupModel
}
```

Update `NewModel`:

```go
func NewModel(gitService *git.GitService, cwd string) Model {
	branches, _ := gitService.ListBranches()
	baseBranch, baseIndex := initialBase(branches)

	return Model{
		git:      gitService,
		selected: 0,
		mode:     modeList,
		cwd:      cwd,
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

Add `"path/filepath"`, `"strings"` (already present), `"os"` imports to `app.go`.

- [ ] **Step 4: Add the `filepath.Rel` match in `View()` list loop**

In the View list loop body written in Task 1, replace the placeholder:

```go
		currentPath := ""
```

with:

```go
		// "you are here" — CWD is inside this worktree (or at it)?
		var hereMarker string
		if rel, err := filepath.Rel(wt.Path, m.cwd); err == nil && !strings.HasPrefix(rel, "..") {
			hereMarker = mutedStyle.Render(" (here)")
		}
```

And change the line construction:

```go
		line := fmt.Sprintf("%s%s %s", prefix, renderedGlyph, branchPart)
		if hereMarker != "" {
			line += hereMarker
		}
```

- [ ] **Step 5: Update the `cmd/root.go` call site**

In `cmd/root.go` `run` function, change (around line 50):

```go
	// Detect current working directory before launching the TUI
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	// Create and run TUI
	model := tui.NewModel(gitService, cwd)
```

(`os` is already imported in `cmd/root.go`.)

- [ ] **Step 6: Run tests and build to verify they pass**

Run: `go test ./... -v` (ALL tests pass — including the updated `TestNewModel`, `TestEmptyListNavigationNoOp`, and `TestInitialBase` which don't use NewModel directly... actually `TestInitialBase` doesn't call NewModel — it calls `initialBase` directly. But `TestNewModel`, `TestEmptyListNavigationNoOp`, `TestStatusGlyphs`, `TestDeleteModal*`, `TestCreateEnterErrorStaysInCreate`, `TestCleanupEnterAccumulatesErrors`, `TestBaseFieldSelectorOnly` all call `NewModel` and need the second arg added.)

Update ALL `NewModel(...)` call sites in the test files to pass `"/tmp/test"` as the second argument:
- `internal/tui/app_test.go` — `TestNewModel`, `TestStatusGlyphs`, `TestEmptyListNavigationNoOp` (all updated in Step 1 already)
- `internal/tui/cleanup_test.go` — `TestCleanupModal`, `TestCleanupEnterAccumulatesErrors`
- `internal/tui/create_test.go` — `TestCreateModal`, `TestBaseFieldSelectorOnly`, `TestCreateEnterErrorStaysInCreate`
- `internal/tui/delete_test.go` — `TestDeleteModal`, `TestDeleteModalForceConfirm`, `TestDeleteClearsErrMsgOnEntry`

Run: `go build ./...` (must succeed — proves `cmd/root.go` call site updated).

- [ ] **Step 7: Commit**

```bash
git add internal/tui/app.go cmd/root.go internal/tui/app_test.go internal/tui/cleanup_test.go internal/tui/create_test.go internal/tui/delete_test.go
git commit -m "feat: 'you are here' marker via NewModel cwd + filepath.Rel match"
```

---

## Chunk 3: Clipboard copy + neutral feedback (Enter key)

### Task 3: Wire go-osc52, add infoMsg field, green feedback on Enter

**Files:**
- Modify: `internal/tui/app.go` (imports, `handleKeyPress` Enter, `handleKeyPress` top, `View()` info display)
- Test: `internal/tui/app_test.go`

- [ ] **Step 1: Write the failing test** (append to `internal/tui/app_test.go`)

```go
func TestEnterSetsInfoMsgNotErrMsg(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{
		{Path: "/tmp/mypath", Branch: "feat"},
	}
	m.selected = 0
	m.mode = modeList

	out, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEnter})
	mm := out.(Model)

	if mm.errMsg != "" {
		t.Errorf("errMsg should be empty on Enter, got %q", mm.errMsg)
	}
	if mm.infoMsg == "" {
		t.Error("expected infoMsg to be set on Enter, got empty")
	}
	if !strings.Contains(mm.infoMsg, "/tmp/mypath") {
		t.Errorf("expected infoMsg to contain the path, got %q", mm.infoMsg)
	}

	// View must render infoMsg with infoStyle (green) — at minimum, the text must appear.
	view := mm.View()
	if !strings.Contains(view, mm.infoMsg) {
		t.Errorf("expected infoMsg in view, got:\n%s", view)
	}
	// Must NOT be rendered as errorStyle (red) — at minimum the error block stays empty.
	if strings.Contains(view, "errMsg") {
		t.Errorf("view should not leak errMsg placeholder text: %s", view)
	}
}

func TestInfoMsgClearedOnNextKeypress(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{{Path: "/p", Branch: "b"}}
	m.selected = 0
	m.infoMsg = "lingering from before"
	m.mode = modeList

	// Any keypress should clear infoMsg
	out, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyUp})
	mm := out.(Model)
	if mm.infoMsg != "" {
		t.Errorf("infoMsg should be cleared on next keypress, got %q", mm.infoMsg)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/... -run "TestEnterSetsInfoMsgNotErrMsg|TestInfoMsgClearedOnNextKeypress" -v`
Expected: FAIL — `m.infoMsg` field exists on Model (added in Task 2 Step 3) but is never set by Enter; the Enter handler still sets `m.errMsg`; infoMsg isn't cleared on keypress.

- [ ] **Step 3: Wire clipboard copy + infoMsg in `handleKeyPress`**

In `internal/tui/app.go`:

Add import `"github.com/aymanbagabas/go-osc52/v2"`.

At the top of `handleKeyPress`, clear `m.infoMsg`:

```go
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.infoMsg = ""
	switch msg.Type {
	// ... existing cases
```

Replace the `tea.KeyEnter` case (currently lines 289-294):

```go
	case tea.KeyEnter:
		if len(m.worktrees) > 0 && m.selected < len(m.worktrees) {
			path := m.worktrees[m.selected].Path
			// Best-effort OSC 52 clipboard copy. If the terminal doesn't support
			// it (piped output, headless), the user still sees the path text.
			copied := tryCopyClipboard(path)
			if copied {
				m.infoMsg = fmt.Sprintf("Copied: %s", path)
			} else {
				m.infoMsg = fmt.Sprintf("Path: %s", path)
			}
			m.errMsg = ""
		}
		return m, nil
```

Add the helper function (above `handleKeyPress`):

```go
// tryCopyClipboard writes path to the system clipboard via OSC 52 escape sequences.
// Returns false if the write fails (e.g. output is piped, terminal doesn't support OSC 52).
// The go-osc52/v2 API: osc52.New(path) returns a Sequence that implements fmt.Stringer;
// we emit it by writing to stderr (the package's documented channel — avoids interfering
// with bubbletea's stdout rendering in alt-screen mode).
func tryCopyClipboard(path string) bool {
	if _, err := fmt.Fprint(os.Stderr, osc52.New(path)); err != nil {
		return false
	}
	return true
}
```

Verified via `go doc github.com/aymanbagabas/go-osc52/v2`: `func New(strs ...string) Sequence`. The package's documented usage example is `fmt.Fprint(os.Stderr, osc52.New("hello world"))`. Writing to stderr keeps OSC 52 separate from bubbletea's stdout-rendered TUI frames.

Add `"os"` import to `app.go` if not already present (it isn't — app.go only imports `fmt`, `"strings"`, `"time"`, `tea`, the git and model packages, and the new `"path/filepath"` and lipgloss from Tasks 1-2).

- [ ] **Step 4: Render `infoMsg` in `View()`**

In `View()`, after the help line (`b.WriteString(helpStyle.Render("[a]dd..."))`), add (before the existing `errMsg` block):

```go
	if m.infoMsg != "" {
		b.WriteString("\n")
		b.WriteString(infoStyle.Render(m.infoMsg))
	}
```

The existing `errMsg` block stays as-is:

```go
	if m.errMsg != "" {
		b.WriteString("\n\n")
		b.WriteString(errorStyle.Render(m.errMsg))
	}
```

(The `\n` vs `\n\n` spacing avoids stacked double-newlines when neither or both are set — minor visual detail; adjust if it looks wrong in manual QA.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tui/... -run "TestEnterSetsInfoMsgNotErrMsg|TestInfoMsgClearedOnNextKeypress|TestStatusGlyphs|TestHereMarkerOnWorktreeRoot|TestNewModelWithCWD" -v`
Run: `go test ./... -v` (ALL tests pass)
Run: `go build ./...`

- [ ] **Step 6: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat: Enter copies path to clipboard via OSC 52, shows green infoMsg"
```

---

## Chunk 4: Final verification

### Task 4: Full suite verification + manual QA

**Files:** none

- [ ] **Step 1: Run the full test suite**

Run: `go test ./... -v`
Expected: ALL PASS. Note any pre-existing failures separately — do NOT fix them (out of scope).

- [ ] **Step 2: Run vet and build**

Run: `go vet ./... && go build ./...`
Expected: exit 0, no output.

- [ ] **Step 3: Manual QA (per spec)**

In a real terminal, run `./git-worktree-tui` from a few different working directories:

- Start from the repo root (so your CWD is the main worktree path). Verify `(here)` appears on the main worktree row and nowhere else.
- `cd` into a subdirectory of a worktree (e.g. `internal/tui`) and run again. Verify `(here)` still appears on the correct worktree row (the one whose path is an ancestor of your CWD).
- Verify dirty vs clean glyphs: if any worktree has uncommitted changes, it shows `●` (amber) while clean ones show `○` (green). On a selected row, the color may be overridden by selection style — that's the accepted trade-off per spec.
- Press Enter on a worktree. Verify: (a) green `Copied: <path>` message appears (NOT red error), (b) paste in another app confirms the path is actually in the clipboard. If the terminal doesn't support OSC 52, you should see `Path: <path>` in green instead (still not red).
- Press another key after Enter. Verify the `Copied:`/`Path:` message clears.

- [ ] **Step 4: Commit any test-only touch-ups** (if none, skip)

```bash
git add -A && git commit -m "test: full suite green for worktree-tui UX improvements" --allow-empty
```

---

## Out of scope (per spec)

- No ahead/behind display, location toggle visual, cleanup simplification, search/filter, auto-refresh, or path truncation.
- No git-layer changes (`internal/git/` untouched).
- No new flows or modal styling changes.
- No change to how Status is loaded (the `?` glyph just makes the existing nil-Status state visible).
- No preservation of status color on the selected row (whole-line `selectedStyle` keeps the current selection visual; per-glyph color is the incremental win on unselected rows).