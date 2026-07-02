# git-worktree-tui UX Improvements Batch 2 — Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available). Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 7 polish + moderate UX features: location visual, ahead/behind display, path truncation, single-stale fast-cleanup, `/` filter mode, `o` cd-clipboard keybind, 10s `tea.Tick` auto-refresh.

**Architecture:** All changes in TUI layer (`internal/tui/`); `cmd/root.go` is unchanged (NewModel is already 2-arg). No git-layer changes. Each chunk extracts a pure helper where possible for unit-testability without shelling out or populating the model bubbletea program.

**Tech Stack:** Go 1.26, Bubble Tea v1.3.10 (provides `tea.Tick`/`tea.Batch`), Lipgloss v1.1.0, go-osc52/v2. `classifyStale`, `tryCopyClipboard`, `infoStyle`/`mainStyle`, `WorktreeStatus.Ahead`/`Behind` all already exist from prior batches.

**Spec:** `docs/superpowers/specs/2026-07-02-worktree-tui-ux-batch2-design.md`

---

## File map

- Modify: `internal/tui/styles.go` — add `activeFieldStyle` (reuse `primaryColor`).
- Modify: `internal/tui/app.go` —
  - Add `filterMode bool`, `filterText string`, `staleCount int`, `stalePaths []string` fields to `Model`.
  - Introduce `tickMsg time.Time` msg + `autoRefresh()` tea.Cmd; `Init()` batches loadWorktrees + autoRefresh; `Update()` schedules another batch on tickMsg.
  - `worktreesLoadedMsg` handler recomputes `staleCount`/`stalePaths` via `classifyStale` (uses `m.git.ListBranches()` — same as `findStaleWorktrees` but stored persistently).
  - `View()` list loop: render `↑N ↓N` muted annotation after the `(here)` marker; render the path line via `truncatePath(wt.Path)` helper; iterate `visibleWorktrees(m.worktrees, m.filterText)` for display only while `m.selected` semantics stay as the true `m.worktrees` index; show filter input at bottom when `filterMode` is active; show footer hint when `staleCount > 0`.
  - `handleKeyPress`:
    - At top: in `filterMode`, runes append to `filterText`; Backspace deletes; Esc clears filter+mode; Enter exits mode (keeps filter).
    - Add `case "/"`: enter filter mode.
    - Add `case "o"`: copy `cd <path>` to clipboard via existing `tryCopyClipboard`, set green `infoMsg`.
    - Modify `case "c"`: when `staleCount == 1`, immediate-remove fast path; otherwise existing modal entry.
    - Modify nav keys (j/k/↑/↓): skip filtered-out indices when filterText non-empty.
- Modify: `internal/tui/create.go` — `viewCreateModal()` location field becomes `<inside | outside>` with active bold+primary, inactive muted.
- Modify: `internal/tui/cleanup.go` — no production change (the `classifyStale` helper is already there); test file possibly updated only via NewModel signature (already updated).
- Modify tests: `internal/tui/app_test.go` — TestVisibleWorktrees, TestTruncatePath, TestStaleHintText, TestOKeybind, TestFilterMode flow, TestAutoRefreshTick. Update existing tests if Init() returns Batch (no test calls Init currently — verify).

---

## Chunk A: Pure helpers + render tweaks (#5, #6, #10)

### Task 1: render improvements + helpers (Tasks 1, 6, 10 from spec) — one implementer dispatch

**Files:**
- Modify: `internal/tui/styles.go` (add `activeFieldStyle`)
- Modify: `internal/tui/create.go` (`viewCreateModal()` location label)
- Modify: `internal/tui/app.go` (View list loop: ahead/behind annotation + truncatePath in commit line)
- Test: `internal/tui/app_test.go` (`TestTruncatePath`), `internal/tui/create_test.go` (`TestLocationVisual` if useful — or covered by existing `TestCreateModal` snapshot)

- [ ] **Step 1: Write the failing tests**

Append to `internal/tui/app_test.go`:

```go
func TestTruncatePath(t *testing.T) {
	tests := []struct{ in, want string }{
		{"/short", "/short"},
		{"", ""},
		{"/Users/kjaniec-dev/dev/projects/git-worktree-tui", ".../projects/git-worktree-tui"},
		{strings.Repeat("/very-long-segment", 5), ".../very-long-segment/very-long-segment"},
	}
	for _, tt := range tests {
		got := truncatePath(tt.in)
		if got != tt.want {
			t.Errorf("truncatePath(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}
```

(For ahead/behind: hard to test rendering without constructing View output; the test will just check `View()` output contains `↑3` for a worktree with `Ahead: 3`.)

Append to `internal/tui/app_test.go`:

```go
func TestAheadBehindAnnotation(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{
		{Path: "/p", Branch: "feat", Status: &model.WorktreeStatus{Ahead: 3, Behind: 0}},
		{Path: "/p2", Branch: "main", Status: &model.WorktreeStatus{Ahead: 0, Behind: 1}},
		{Path: "/p3", Branch: "clean", Status: &model.WorktreeStatus{Ahead: 0, Behind: 0}},
	}
	m.mode = modeList
	view := m.View()
	if !strings.Contains(view, "↑3") {
		t.Errorf("expected ↑3 annotation:\n%s", view)
	}
	if !strings.Contains(view, "↓1") {
		t.Errorf("expected ↓1 annotation:\n%s", view)
	}
	// Clean worktree should NOT have an up/down annotation
	cleanRows := strings.Count(view, "/p3")
	if cleanRows < 1 {
		t.Errorf("clean worktree should still render its row:\n%s", view)
	}
}
```

- [ ] **Step 2: Run tests to verify fail**

Expected: FAIL — `undefined: truncatePath`; no `↑3`/`↓1` in view.

- [ ] **Step 3: Implement helpers + render changes**

In `internal/tui/app.go`, add pure helper above `handleKeyPress`:

```go
// truncatePath shortens paths longer than 40 chars to "..." + last two path segments.
// Paths ≤ 40 chars or single-segment paths are returned as-is.
func truncatePath(path string) string {
	if len(path) <= 40 || path == "" {
		return path
	}
	sep := string(filepath.Separator)
	parts := strings.Split(path, sep)
	if len(parts) < 3 {
		return path
	}
	return "..." + sep + parts[len(parts)-2] + sep + parts[len(parts)-1]
}
```

(`filepath` already imported.)

In `View()` list loop, after the `(here)` marker (i.e. `line += hereMarker`), add:

```go
// Ahead/behind annotation — only when nonzero.
if wt.Status != nil && (wt.Status.Ahead > 0 || wt.Status.Behind > 0) {
	var ab strings.Builder
	if wt.Status.Ahead > 0 {
		ab.WriteString(fmt.Sprintf(" ↑%d", wt.Status.Ahead))
	}
	if wt.Status.Behind > 0 {
		ab.WriteString(fmt.Sprintf(" ↓%d", wt.Status.Behind))
	}
	line += mutedStyle.Render(ab.String())
}
```

In the commit line render (the line below the branch):

```go
b.WriteString(fmt.Sprintf("    %s • %s", commitDisplay, truncatePath(wt.Path)))
b.WriteString("\n\n")
```

In `internal/tui/styles.go` add to the `var (...)` block:

```go
// activeFieldStyle highlights the active option in a segmented control
// (e.g. Location <inside | outside>).
activeFieldStyle = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)
```

In `internal/tui/create.go` `viewCreateModal()`, replace:

```go
locationLabel := fmt.Sprintf("Location: [%s]", m.create.location)
```

with:

```go
var insidePart, outsidePart string
if m.create.location == "inside" {
    insidePart = activeFieldStyle.Render("inside")
    outsidePart = mutedStyle.Render("outside")
} else {
    insidePart = mutedStyle.Render("inside")
    outsidePart = activeFieldStyle.Render("outside")
}
locationLabel := fmt.Sprintf("Location: <%s | %s>", insidePart, outsidePart)
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/tui/... -run "TestTruncatePath|TestAheadBehindAnnotation" -v`
Run: `go test ./... -v && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/tui/styles.go internal/tui/create.go internal/tui/app.go internal/tui/app_test.go
git commit -m "feat: ahead/behind annotation, path truncation, segmented location visual"
```

---

## Chunk B: Single-stale footer + auto-refresh (#7 + #9)

### Task 2: staleCount + stalePaths fields; footer hint + immediate c-remove; tea.Tick — one implementer dispatch

**Files:**
- Modify: `internal/tui/app.go`:
  - `Model` adds `staleCount int` and `stalePaths []string`.
  - Add `tickMsg time.Time` type and `autoRefresh() tea.Cmd` (uses existing `time` import).
  - `Init()` returns `tea.Batch(m.loadWorktrees, autoRefresh())`.
  - `Update()` switch adds `case tickMsg:` returns `m, tea.Batch(m.loadWorktrees, autoRefresh())`.
  - `worktreesLoadedMsg` handler recomputes `staleCount`/`stalePaths` via `classifyStale` using `m.git.ListBranches()`.
  - `View()` help-line builder adds stale hint when `staleCount > 0`.
  - `handleKeyPress` `case "c"`: fast path when `staleCount == 1` and `len(m.stalePaths) == 1`.
- Test: `internal/tui/app_test.go` (`TestStaleHintText`, `TestAutoRefreshTick`).

- [ ] **Step 1: Add fields and helpers, write failing tests**

Add to `Model` struct (insert after `stalePaths`):

```go
staleCount int
stalePaths []string
```

Write failing tests (append to `internal/tui/app_test.go`):

```go
func TestStaleHintText(t *testing.T) {
	tests := []struct{ count, want string }{
		{"0", "  "},
		{"1", "  [c]leanup 1 stale (Enter to remove)  "},
		{"3", "  [c]leanup 3 stale  "},
		{"7", "  [c]leanup 7 stale  "},
	}
	for _, tt := range tests {
		var c int
		fmt.Sscanf(tt.count, "%d", &c)
		got := staleHintText(c)
		if got != tt.want {
			t.Errorf("staleHintText(%s) = %q, want %q", tt.count, got, tt.want)
		}
	}
}

func TestAutoRefreshTick(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	cmds := m.Init()
	// Init() must return a batch including loadWorktrees AND autoRefresh.
	// A tea.Batch decodes into multiple cmds; we inspect by running once.
	if cmds == nil {
		t.Fatal("Init returned nil cmd")
	}
	// Trigger tickMsg: Update must return a Batch (loadWorktrees + next autoRefresh)
	updated, cmd := m.Update(tickMsg{})
	if cmd == nil {
		t.Error("Update on tickMsg must return a refresh cmd")
	}
	_ = updated
}
```

Add `"fmt"` if not present (it is). Add `fmt.Sscanf`-style — note the `fmt` import is shared; just use it.

- [ ] **Step 2: Run tests to verify fail**

Expected: FAIL — `undefined: tickMsg`, `undefined: staleHintText`, `Init()` returns single cmd not Batch.

- [ ] **Step 3: Implement helpers + Update + Init changes**

Add pure helper above `handleKeyPress`:

```go
func staleHintText(count int) string {
	switch {
	case count <= 0:
		return "  "
	case count == 1:
		return "  [c]leanup 1 stale (Enter to remove)  "
	default:
		return fmt.Sprintf("  [c]leanup %d stale  ", count)
	}
}
```

Add at top of app.go (near other message types):

```go
type tickMsg time.Time

func autoRefresh() tea.Cmd {
	return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
```

Update `Init()`:

```go
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.loadWorktrees, autoRefresh())
}
```

Add to `Update()` switch (before `default`):

```go
case tickMsg:
	return m, tea.Batch(m.loadWorktrees, autoRefresh())
```

Update `worktreesLoadedMsg` handler to recompute stale state. After the existing `m.worktrees = msg.worktrees` and selected clamp, add:

```go
// Recompute stale count for the footer hint + immediate-remove fast path.
if branches, err := m.git.ListBranches(); err == nil {
	branchSet := make(map[string]bool)
	for _, b := range branches {
		branchSet[b] = true
	}
	staleIdxs := classifyStale(m.worktrees, branchSet)
	m.staleCount = len(staleIdxs)
	m.stalePaths = make([]string, 0, len(staleIdxs))
	for _, idx := range staleIdxs {
		m.stalePaths = append(m.stalePaths, m.worktrees[idx].Path)
	}
} else {
	m.staleCount = 0
	m.stalePaths = nil
}
```

Update View help-line render to prepend the hint:

```go
helpText := "[a]dd [d]elete [c]leanup [r]efresh [o]pen (cd) [/]filter [q]uit"
hint := staleHintText(m.staleCount)
b.WriteString(helpStyle.Render(hint + helpText))
```

(Other improvements like `[o]pen` and `[/]filter` will be added in Chunk C; for now use the updated help string so the footer reflects the new keybinds even before they're wired — tests look for `[c]leanup` substring. Actually: defer adding `[o]pen (cd) [/]filter` to Chunk C's commit. For Chunk B, keep the existing help line `[a]dd [d]elete [c]leanup [r]efresh [q]uit` and just prepend the hint when staleCount > 0.)

So for Chunk B:

```go
helpText := "[a]dd [d]elete [c]leanup [r]efresh [q]uit"
if m.staleCount > 0 {
	helpText = staleHintText(m.staleCount) + helpText
}
b.WriteString(helpStyle.Render(helpText))
```

Update `handleKeyPress` `case "c"`:

```go
case "c":
	if m.staleCount == 1 && len(m.stalePaths) == 1 {
		path := m.stalePaths[0]
		if err := m.git.RemoveWorktree(path, false); err != nil {
			m.errMsg = fmt.Sprintf("Failed to remove %s: %v", path, err)
		} else {
			m.infoMsg = fmt.Sprintf("Removed: %s", path)
		}
		return m, m.loadWorktrees
	}
	m.findStaleWorktrees()
	m.cleanup.currentIndex = 0
	m.mode = modeCleanup
	return m, nil
```

- [ ] **Step 4: Run tests + full suite**

Run: `go test ./internal/tui/... -run "TestStaleHintText|TestAutoRefreshTick" -v`
Run: `go test ./... -v && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat: 10s auto-refresh via tea.Tick + single-stale cleanup fast path"
```

---

## Chunk C: `o` cd-clipboard + `/` filter mode (#4 + #8)

### Task 3: o keybind + filterMode plumbing — one implementer dispatch

**Files:**
- Modify: `internal/tui/app.go`:
  - `Model` adds `filterMode bool`, `filterText string`.
  - Add pure helpers `visibleWorktrees`, `advanceSelected`.
  - `View()` list loop iterates `visibleWorktrees(m.worktrees, m.filterText)` for display; render filter input at bottom when `filterMode`; help-line update to include `[o]pen (cd)  [/]filter`.
  - `handleKeyPress`:
    - Branch on `m.filterMode` at top: runes append to filterText; backspace deletes; Esc clears filter+mode; Enter exits mode (keeps filter applied).
    - Add `case "/"` to enter filter mode (when not in filterMode).
    - Add `case "o"` to copy `cd <path>`.
    - Nav keys (j/k/↑/↓) advance using `advanceSelected` when filterText non-empty.
- Test: `internal/tui/app_test.go` (`TestVisibleWorktrees`, `TestAdvanceSelected`, `TestOKeybind`, `TestFilterModeFlow`).

- [ ] **Step 1: Write failing tests + add helpers**

Append tests to `internal/tui/app_test.go`:

```go
func TestVisibleWorktrees(t *testing.T) {
	wts := []model.Worktree{
		{Branch: "main"},
		{Branch: "feature/auth"},
		{Branch: "dev/fix"},
		{Branch: "(detached)"},
	}
	if got := visibleWorktrees(wts, ""); len(got) != 4 {
		t.Errorf("empty filter: len %d, want 4", len(got))
	}
	if got := visibleWorktrees(wts, "auth"); len(got) != 1 || got[0].Branch != "feature/auth" {
		t.Errorf("auth filter: %+v", got)
	}
	if got := visibleWorktrees(wts, "MAIN"); len(got) != 1 || got[0].Branch != "main" {
		t.Errorf("case-insensitive MAIN: %+v", got)
	}
	if got := visibleWorktrees(wts, "zzz"); len(got) != 0 {
		t.Errorf("non-matching filter: len %d, want 0", len(got))
	}
	// "(detached)" matches substring "detach"
	if got := visibleWorktrees(wts, "detach"); len(got) != 1 {
		t.Errorf("detach substring: %+v", got)
	}
}

func TestAdvanceSelected(t *testing.T) {
	wts := []model.Worktree{
		{Branch: "main"}, {Branch: "feature/auth"}, {Branch: "dev/fix"},
	}
	// No filter: advance goes 0 → 1 → 2 → 2 (clamp)
	if got := advanceSelected(0, +1, wts, ""); got != 1 {
		t.Errorf("no-filter 0→1: got %d", got)
	}
	if got := advanceSelected(2, +1, wts, ""); got != 2 {
		t.Errorf("no-filter clamp at 2: got %d", got)
	}
	// Filter "auth": only index 1 visible; advance from 0 (main, filtered out) → 1
	// Per spec: skip filtered-out indices in the requested direction.
	if got := advanceSelected(0, +1, wts, "auth"); got != 1 {
		t.Errorf("filter auth from 0 up by 1: got %d, want 1", got)
	}
	if got := advanceSelected(0, +1, wts, "zzz"); got != 0 {
		t.Errorf("no visible match: got %d, want 0 (stay)", got)
	}
}

func TestOKeybind(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{{Path: "/wt/feat", Branch: "feat"}}
	m.selected = 0
	m.mode = modeList
	out, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	mm := out.(Model)
	if !strings.Contains(mm.infoMsg, "cd /wt/feat") {
		t.Errorf("expected infoMsg with 'cd /wt/feat', got %q", mm.infoMsg)
	}
	if mm.errMsg != "" {
		t.Errorf("errMsg should be empty on o, got %q", mm.errMsg)
	}
}

func TestFilterModeFlow(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{
		{Branch: "main"},
		{Branch: "feature/auth"},
		{Branch: "dev/fix"},
	}
	m.mode = modeList

	// Enter filter mode
	out, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	mm := out.(Model)
	if !mm.filterMode {
		t.Error("expected filterMode after /")
	}

	// Type "auth"
	m = mm
	out, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("auth")})
	mm = out.(Model)
	if mm.filterText != "auth" {
		t.Errorf("filterText = %q, want 'auth'", mm.filterText)
	}

	// Esc clears filter + exits filter mode
	m = mm
	out, _ = m.handleKeyPress(tea.KeyMsg{Type: tea.KeyEscape})
	mm = out.(Model)
	if mm.filterMode {
		t.Error("filterMode should be false after Esc")
	}
	if mm.filterText != "" {
		t.Errorf("filterText should be empty after Esc, got %q", mm.filterText)
	}
}
```

- [ ] **Step 2: Run tests to verify fail**

Expected: FAIL — `undefined: visibleWorktrees`, `undefined: advanceSelected`, `o` keybind missing, filter mode plumbing absent.

- [ ] **Step 3: Implement helpers + keybinds + render**

Add to `internal/tui/app.go` (above `handleKeyPress`):

```go
// visibleWorktrees returns worktrees whose Branch contains filter (case-insensitive).
// Empty filter returns all.
func visibleWorktrees(wts []model.Worktree, filter string) []model.Worktree {
	if filter == "" {
		return wts
	}
	needle := strings.ToLower(filter)
	var out []model.Worktree
	for _, wt := range wts {
		if strings.Contains(strings.ToLower(wt.Branch), needle) {
			out = append(out, wt)
		}
	}
	return out
}

// advanceSelected returns the next visible index starting from `sel` in `dir`
// (+1 forward, -1 back). Skips worktrees filtered out by `filterText`. Stays
// at `sel` if no visible match exists in the requested direction.
func advanceSelected(sel, dir int, wts []model.Worktree, filterText string) int {
	if len(wts) == 0 {
		return 0
	}
	needle := strings.ToLower(filterText)
	match := func(i int) bool {
		return filterText == "" || strings.Contains(strings.ToLower(wts[i].Branch), needle)
	}
	// Find next match in dir
	for step := 1; ; step++ {
		next := sel + dir*step
		if next < 0 || next >= len(wts) {
			return sel // clamped; no further visible match
		}
		if match(next) {
			return next
		}
	}
}
```

Update `Model` struct (add fields):

```go
filterMode bool
filterText string
```

Update `handleKeyPress`:

At the entry, before `switch msg.Type` — if `m.filterMode`, delegate filter-mode key handling:

```go
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.filterMode {
		switch msg.Type {
		case tea.KeyRunes:
			m.filterText += string(msg.Runes)
		case tea.KeyBackspace:
			if len(m.filterText) > 0 {
				m.filterText = m.filterText[:len(m.filterText)-1]
			}
		case tea.KeyEscape:
			m.filterMode = false
			m.filterText = ""
			m.selected = 0
		case tea.KeyEnter:
			m.filterMode = false // keep filter applied
		}
		return m, nil
	}

	m.infoMsg = ""
	switch msg.Type {
	case tea.KeyRunes:
		// ... existing cases ...
		// ADD:
		case "/":
			m.filterMode = true
			m.filterText = ""
			return m, nil
		case "o":
			if len(m.worktrees) > 0 && m.selected < len(m.worktrees) {
				path := m.worktrees[m.selected].Path
				if tryCopyClipboard("cd " + path) {
					m.infoMsg = fmt.Sprintf("Copied: cd %s", path)
				} else {
					m.infoMsg = fmt.Sprintf("cd %s", path)
				}
				m.errMsg = ""
			}
			return m, nil
	// ...
	case tea.KeyEscape:
		// In list mode with a non-empty filter (but not in filter mode), Esc clears the filter
		if m.filterText != "" {
			m.filterText = ""
			m.selected = 0
		}
		return m, nil
	// Update nav:
	case tea.KeyDown:
		if len(m.worktrees) == 0 {
			return m, nil
		}
		m.selected = advanceSelected(m.selected, +1, m.worktrees, m.filterText)
		return m, nil
	case tea.KeyUp:
		if len(m.worktrees) == 0 {
			return m, nil
		}
		m.selected = advanceSelected(m.selected, -1, m.worktrees, m.filterText)
		return m, nil
	}
}
```

For `case "j"`/`case "k"` in `tea.KeyRunes`, replace the existing `m.selected++`/`--` with `advanceSelected` calls:

```go
case "j":
	if len(m.worktrees) == 0 {
		return m, nil
	}
	m.selected = advanceSelected(m.selected, +1, m.worktrees, m.filterText)
	return m, nil
case "k":
	if len(m.worktrees) == 0 {
		return m, nil
	}
	m.selected = advanceSelected(m.selected, -1, m.worktrees, m.filterText)
	return m, nil
```

Update `View()`:

The list loop must render only `visibleWorktrees(m.worktrees, m.filterText)` for display, but the selection arrow `→` is shown on the row whose original index == `m.selected` (which is the true `m.worktrees` index, unchanged by Chunk C: per design Alternative A).

Replace the existing `for i, wt := range m.worktrees` loop with:

```go
visible := visibleWorktrees(m.worktrees, m.filterText)
// Build a map from original index → visible for fast lookup
visibleSet := make(map[int]bool)
for _, v := range visible {
	for i, wt := range m.worktrees {
		if wt.Path == v.Path && wt.Branch == v.Branch {
			visibleSet[i] = true
			break
		}
	}
}
// Render
for i, wt := range m.worktrees {
	if m.filterText != "" && !visibleSet[i] {
		continue
	}
	// ... existing row rendering for this worktree ...
	// prefix = "→ " when i == m.selected else "  "
	// ...
}
```

(Simpler: since `m.selected` is the true `m.worktrees` index, the loop iterates `m.worktrees` and skips indices the filter excludes; the `→ ` prefix still works because `i == m.selected` is the true comparison.)

For the filter input, after the help line, when `filterMode`:

```go
if m.filterMode {
	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("/" + m.filterText + "▏"))
}
```

Update the help line:

```go
helpText := "[a]dd [d]elete [c]leanup [r]efresh [o]pen (cd) [/]filter [q]uit"
if m.staleCount > 0 {
	helpText = staleHintText(m.staleCount) + helpText
}
b.WriteString(helpStyle.Render(helpText))
```

Also handle lost-focus case in modals: pressing `Esc` from a modal returns to list mode; filter state should NOT persist from before the modal (modal keystrokes shouldn't be affected). Since modal handlers return `m` before reaching filterMode, this should naturally work. Just keep `m.filterText` as-is across modal entry/exit — if filter was applied before pressing `d`, it remains applied after pressing `n` (cancel). That's reasonable.

When `worktreesLoadedMsg` fires, re-clamp `m.selected` via `advanceSelected` if needed so that selection lands on a now-visible entry. The existing clamp `if m.selected >= len(m.worktrees)` stays; additionally, if `m.filterText` is non-empty and `visibleWorktrees` doesn't contain `m.worktrees[m.selected]`, advance to the next visible:

```go
// In Update() worktreesLoadedMsg handler, AFTER existing selected clamp:
if m.filterText != "" && m.selected < len(m.worktrees) {
	needle := strings.ToLower(m.filterText)
	if !strings.Contains(strings.ToLower(m.worktrees[m.selected].Branch), needle) {
		m.selected = advanceSelected(m.selected, +1, m.worktrees, m.filterText)
		if !strings.Contains(strings.ToLower(m.worktrees[m.selected].Branch), needle) {
			m.selected = 0 // No visible entry: fall back to 0 — view shows "no worktrees" eventually
		}
	}
}
```

- [ ] **Step 4: Run tests + full suite**

Run: `go test ./internal/tui/... -run "TestVisibleWorktrees|TestAdvanceSelected|TestOKeybind|TestFilterModeFlow" -v`
Run: `go test ./... -v && go build ./...`

- [ ] **Step 5: Commit**

```bash
git add internal/tui/app.go internal/tui/app_test.go
git commit -m "feat: o copies cd <path>, / opens branch-name filter mode"
```

---

## Chunk D: Final verification

### Task 4: full-suite + gofmt + final holistic review

- [ ] Run `go test ./... -count=1 -v` (all pass)
- [ ] Run `go vet ./... && go build ./...` (exit 0)
- [ ] Run `gofmt -w` on changed files (fix any whitespace drift)
- [ ] Commit gofmt pass: `git commit -m "style: gofmt batch2 changed files"`
- [ ] Dispatch one Momus review over the entire diff (`base=0a7e8ea` → HEAD)
- [ ] Manual QA checklist (described in spec)

---

## Out of scope

- New terminal spawning (item 4: clipboard-copy approach only).
- fsnotify file watcher (item 9: polling per user approval).
- n/N next-match navigation (item 8: simple substring filter only).
- Color-coded ahead/behind annotation (muted only for now).
- Configurable auto-refresh interval (hardcoded 10s).
- Git-layer changes (internal/git/ untouched).