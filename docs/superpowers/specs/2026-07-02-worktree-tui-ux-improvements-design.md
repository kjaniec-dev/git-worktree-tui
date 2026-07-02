# git-worktree-tui UX Improvements — Design Spec

- Date: 2026-07-02
- Status: Approved (pending spec review)
- Scope: Three UX improvements to `internal/tui/app.go`, `internal/tui/styles.go`, `cmd/root.go`. No new flows, no git-layer changes, no behavioral changes to create/delete/cleanup.

## Problem

The git-worktree-tui list view has three UX friction points that make it harder to use than it should be:

1. **Dirty and clean worktrees share the same glyph (`●`).** `internal/tui/app.go:141-148` falls through to `●` for both clean and `IsDirty` worktrees — the `else if wt.Status != nil && wt.Status.IsDirty` branch sets `status = "●"`, identical to the default. A user scanning the list cannot tell at a glance which worktrees have uncommitted changes. Furthermore, `internal/tui/styles.go` declares `cleanStyle` (green), `dirtyStyle` (amber), and `lockedStyle` (red) — all unused, despite the rendering already having the infrastructure to colorize.

2. **No marker for the worktree the user is currently inside.** The list shows `★` for main and `🔒` for locked, but nothing indicates "this is your current working directory." A user with multiple worktrees open across terminals has to read the path line to recognize their own CWD, which is easy to miss when paths share long prefixes. `cmd/root.go`'s `findRepoRoot` already calls `os.Getwd()` to locate the repo, but that information is discarded — the TUI never knows the user's CWD.

3. **Enter displays the selected path as a red error.** `internal/tui/app.go:289-294` sets `m.errMsg = fmt.Sprintf("Path: %s", path)`, which the view renders with `errorStyle` (red, bold, `styles.go:19`). The README claims "Enter - Show full path (copy to clipboard)" but no clipboard code exists — `go-osc52/v2` is present in `go.mod` (indirect, via bubbletea) yet never imported or used. A user pressing Enter expecting "select" or "copy" sees a path flash in red error style, which reads as a failure.

## Design

### A. Distinct dirty vs clean status glyphs + color

Wire the existing `cleanStyle`, `dirtyStyle`, `lockedStyle` (and `mutedStyle` for unknown) into the status glyph rendering by splitting the line into a styled glyph segment and a branch segment, then concatenating.

**Glyphs and precedence** (same precedence chain as current code, just with distinct glyphs and colors applied):

| State | Glyph | Style |
|-------|-------|-------|
| `IsLocked` | `🔒` | `lockedStyle` (red) |
| `IsMain` | `★` | `primaryColor` via `titleStyle`-equivalent (reuse `selectedStyle` or a new `mainStyle`; reuse `primaryColor` to match the title) |
| `Status != nil && IsDirty` | `●` | `dirtyStyle` (amber) |
| `Status != nil && !IsDirty` | `○` | `cleanStyle` (green) |
| `Status == nil` (not loaded) | `?` | `mutedStyle` (gray) |

**Rendering change in `View()` list loop** (`internal/tui/app.go:141-160`): instead of building `line := fmt.Sprintf("%s%s %s", prefix, status, wt.Branch)` as one string, render the glyph as `glyphStyle.Render(glyph)` and the branch as `branchPart`, then concatenate. When the row is selected, apply `selectedStyle` to the branch name (not the glyph — the glyph keeps its status color so dirty/clean stays scannable even on the selected row). If applying selection to the whole line is preferred for consistency with the current code, the glyph color is lost on selection — acceptable trade-off; flag in the plan as an explicit choice.

**Recommendation:** keep the glyph's status color even on the selected row (render glyph and branch separately), so dirty/clean is always scannable. The `prefix` (`→ ` / `  `) stays unstyled.

No precedence change: locked > main > dirty > clean > unknown. A main worktree that's dirty shows `★` (main wins the glyph), colored with `primaryColor`. This is the current behavior, just with distinct glyphs and colors wired in.

### B. "You are here" marker for current worktree

**CWD capture:** `NewModel` signature changes from `NewModel(gitService *git.GitService) Model` to `NewModel(gitService *git.GitService, cwd string) Model`. `cmd/root.go`'s `run` function captures `cwd, _ := os.Getwd()` and passes it. A new `cwd string` field on `Model` stores it. The CWD is captured once at startup (the TUI runs in alt-screen mode and never changes the process working directory during the session).

**Matching:** for each worktree in `View()`, compute `rel, err := filepath.Rel(wt.Path, m.cwd)`. If `err == nil` and `!strings.HasPrefix(rel, "..")`, the user's CWD is inside (or at) that worktree — show the marker. This correctly handles subdirectories (the user may be in `wt.Path/internal/tui`, not `wt.Path` itself) and avoids the naive-prefix false positive where `/foo/bar` looks like a prefix of `/foo/barbaz`.

**Rendering:** the marker is a muted `(here)` annotation appended after the branch name (and after a detached marker if present): `★ main (here)`. Rendered via `mutedStyle` so it doesn't compete with branch names. Exactly one worktree gets the marker (CWD can only be inside one worktree at a time).

**Detached handling:** a detached-HEAD worktree renders as `(detached)` — the `(here)` annotation goes after: `(detached) (here)`. Slightly awkward but correct; detached + here is rare.

### C. Clipboard copy + neutral feedback (Enter key)

**Clipboard:** import `github.com/aymanbagabas/go-osc52/v2` in `app.go`. In the `tea.KeyEnter` handler, attempt `osc52.Copy(path)` (writes OSC 52 escape sequences to stdout via the bubbletea program). The copy is best-effort — if it fails (piped output, unsupported terminal), the UI still shows the path text so the user isn't left with nothing.

**Integration with bubbletea:** OSC 52 sequences must be written through the bubbletea program's output, not directly to `os.Stdout` (the TUI owns the terminal in alt-screen mode). Use `tea.NewProgram`'s output writer. The cleanest approach: emit a `tea.Printf` or a custom command that writes the OSC 52 sequence via the program's output. If `tea.Printf` is unavailable in this bubbletea version, fall back to returning a `tea.Cmd` that writes to `os.Stdout` directly (works when not piped). The plan will specify the exact mechanism after checking the bubbletea API.

**Feedback field:** add a new `infoMsg string` field to `Model`. Render it in `View()` after the help line, using `successStyle` (green, already declared in `styles.go:8`) — or a new `infoStyle = lipgloss.NewStyle().Foreground(successColor)` if `successStyle` isn't already defined (it's declared as `cleanStyle` pointing at `successColor`; define `infoStyle` explicitly for clarity, reusing `successColor`).

On Enter: set `m.infoMsg = fmt.Sprintf("Copied: %s", path)`. Clear `m.errMsg` if set (the two don't coexist). Clear `m.infoMsg` on the next keypress (in `handleKeyPress`, set `m.infoMsg = ""` at the top of the function before dispatching, similar to how `errMsg` is managed).

**Rendering in `View()`:** after the help line, if `m.infoMsg != ""`, render `successStyle.Render(m.infoMsg)` (parallel to the existing `errMsg` block which uses `errorStyle`). The `errMsg` block stays for actual errors.

**Clearing:** `m.infoMsg` is cleared on the next keypress (in `handleKeyPress` and the modal handlers). This mirrors the existing `errMsg` lifecycle — ephemeral, replaced by newer messages or cleared by user input.

## Behavior contracts

- `View()` list loop: renders a status glyph with status-appropriate style (locked/red, main/primary, dirty/amber, clean/green, unknown/muted), keeping the glyph color even when the row is selected.
- `NewModel(gitService *git.GitService, cwd string) Model`: accepts CWD, stores it on `Model.cwd`. `cmd/root.go` passes `os.Getwd()`.
- `View()` list loop: appends `mutedStyle.Render(" (here)")` after the branch name for at most one worktree where `filepath.Rel(wt.Path, m.cwd)` doesn't start with `..`.
- `handleKeyPress` `tea.KeyEnter`: attempts `osc52.Copy(path)`, sets `m.infoMsg = "Copied: <path>"` (or `Path: <path>` if copy failed), clears `m.errMsg`.
- `Model.infoMsg string`: new field, rendered with `successStyle`/`infoStyle` in `View()`, cleared on next keypress.

## Testing

- `internal/tui/app_test.go`:
  - `TestStatusGlyphs`: construct `Model` with worktrees in each state (locked, main, dirty, clean, status-nil), assert `View()` output contains the expected glyph (`🔒`, `★`, `●`, `○`, `?`) and that dirty/clean produce distinct strings.
  - `TestHereMarker`: construct `Model` with `cwd` inside one of two worktrees, assert `View()` contains `(here)` exactly once and on the correct line.
  - `TestHereMarkerSubdirectory`: `cwd` is a subdirectory of a worktree path, assert the marker still appears (_filepath.Rel_ doesn't start with `..`).
  - `TestNewModelWithCWD`: assert `NewModel(svc, "/some/cwd")` sets `m.cwd == "/some/cwd"`.
- `internal/tui/styles.go`: `infoStyle` added (reuse `successColor`); no test needed (pure style definition).
- `cmd/root.go`: `tui.NewModel(gitService)` call site updated to `tui.NewModel(gitService, cwd)`; covered by build (no new test file for cmd).
- Run `go test ./...`, `go vet ./...`, `go build ./...` — all must pass.

### Manual / visual QA

Because clipboard and terminal rendering aren't unit-testable:
- Run the TUI in a real terminal. Press Enter on a worktree. Verify: (a) the path appears as green `Copied: <path>` (not red error), (b) the path is actually in the clipboard (paste in another app).
- Run the TUI with CWD inside a non-main worktree. Verify the `(here)` marker appears on that worktree's row and nowhere else.
- Create or find a dirty and a clean worktree. Verify the list shows `●` (amber) for dirty and `○` (green) for clean, visually distinct.
- Run the TUI with CWD at the repo root (main worktree path). Verify `(here)` appears on the main worktree row.

## Out of scope

- No ahead/behind display, location toggle visual, cleanup simplification, search/filter, auto-refresh, or path truncation (the medium and nice-to-have items from the UX discussion).
- No git-layer changes (`internal/git/` untouched).
- No new flows (create/delete/cleanup logic untouched).
- No styling of the create/delete/cleanup modals (only the list view glyph + marker).
- No changes to how Status is loaded (the goroutine path in `loadWorktrees` stays as-is; the `?` glyph just makes the existing nil-Status state visible instead of rendering as `●`).