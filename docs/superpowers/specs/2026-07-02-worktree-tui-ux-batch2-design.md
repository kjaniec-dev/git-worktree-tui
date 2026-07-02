# git-worktree-tui UX Improvements — Batch 2 Design Spec

- Date: 2026-07-02
- Status: Approved (inline, pending spec review)
- Scope: 7 polish + moderate features spread across `internal/tui/app.go`, `internal/tui/create.go`, `internal/tui/cleanup.go`, `internal/tui/styles.go`, `cmd/root.go`. No git-layer changes. No new flows beyond an inline search/filter and a non-modal single-stale fast path.

## Problem

Following the first UX pass (distinct glyphs, ` (here)` marker, clipboard + green `infoMsg` on Enter), seven remaining friction points medium-to-nice-to-have in priority remain:

4. **No "take me there" action.** After listing/selecting a worktree, the user has to quit the TUI and manually `cd` into the chosen path. A one-keybinding shortcut closes the loop.
5. **Location toggle has no visual hint.** The create form renders `Location: [inside]` with no indication the field is interactive. The `l` keybind lives only in the footer help text, easy to miss.
6. **Ahead/behind counts are fetched but never displayed.** `internal/model/worktree.go` already declares `Ahead` and `Behind` fields on `WorktreeStatus`, and `internal/tui/app.go`'s `loadWorktrees` already populates them via goroutines. The list view's commit line only shows short SHA + path; users with stale branches and up-to-date branches look identical.
7. **Cleanup modal fatigue for the common single-stale case.** Pressing `c` always enters cleanup mode (scan → Space-select → Enter-remove → Esc-back). With exactly 1 stale worktree, that's 3 keystrokes too many.
8. **No search/filter for long lists.** With 10+ worktrees, j/k scrolling is slow.
9. **No auto-refresh.** Worktrees created/deleted from outside the TUI (CLI, IDE, another terminal) are invisible until the user presses `r`. The user assumes the list is current; it isn't.
10. **Long paths wrap awkwardly.** Full paths flow to the next line and push subsequent rows down, making the list unreadable.

## Design

### 4. `o` keybind copies `cd <path>` to clipboard

In `internal/tui/app.go` `handleKeyPress` `case tea.KeyRunes` add a new `case "o"`:

```go
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
```

`tryCopyClipboard` was added in the previous UX-batch-1 plan (`fmt.Fprint(os.Stderr, osc52.New(path))`). Reuse as-is. Update the list footer help text to include `[o]pen (cd)` — keep alphabetical-ish ordering.

**Out of scope:** spawning a new terminal, printing a `cd` command on exit (eval-style). User explicitly approved the clipboard-copy approach.

### 5. Location visual: `<inside | outside>`

In `internal/tui/create.go` `viewCreateModal()`, change line (around the current `locationLabel := fmt.Sprintf("Location: [%s]", m.create.location)`):

```go
var insidePart, outsidePart string
if m.create.location == "inside" {
    insidePart = primaryStyle.Render("inside")
    outsidePart = mutedStyle.Render("outside")
} else {
    insidePart = mutedStyle.Render("inside")
    outsidePart = primaryStyle.Render("outside")
}
locationLabel := fmt.Sprintf("Location: <%s | %s>", insidePart, outsidePart)
```

(Reuse `primaryStyle` from `styles.go` — or add a `fieldActiveStyle` alias if `primaryStyle` doesn't exist; the existing `titleStyle` uses `primaryColor`. The simplest path: add `activeFieldStyle = lipgloss.NewStyle().Foreground(primaryColor).Bold(true)` to `styles.go` and reuse it.)

The `l` keybind logic stays unchanged — it already toggles `m.create.location`. Only the rendered string changes. No new state.

### 6. Ahead/behind annotation

In `internal/tui/app.go` `View()` list loop, after the branch+here line and before the commit line, append an up/down annotation when `wt.Status != nil` and either count is nonzero:

```go
if wt.Status != nil && (wt.Status.Ahead > 0 || wt.Status.Behind > 0) {
    var ab strings.Builder
    if wt.Status.Ahead > 0 {
        ab.WriteString(fmt.Sprintf("↑%d", wt.Status.Ahead))
        ab.WriteString(" ")
    }
    if wt.Status.Behind > 0 {
        ab.WriteString(fmt.Sprintf("↓%d", wt.Status.Behind))
    }
    annotation := ab.String()
    // choose color: pure ahead = green (successColor), pure behind = amber (warningColor), mixed = primaryColor
    // (actually, just render with mutStyle.Legacy)
}
```

Placement: the annotation goes **on the same row as the branch name**, right after `(here)` if present, before the row's newline. Render in muted style by default; if user feedback later wants color, switch to per-segment (green for `↑`, amber for `↓`) — for now, muted keeps the glyph/scanner concern stable.

No git-layer changes; `Ahead`/`Behind` already populated by `GetWorktreeStatus`.

### 7. Single-stale footer hint + immediate remove

Add to `Model` struct:

```go
staleCount int
stalePaths []string // populated for the single-stale immediate-remove case
```

In `View()` footer:

```go
helpText := "[a]dd [d]elete [c]leanup [r]efresh [o]pen (cd) [/]filter [q]uit"
switch m.staleCount {
case 1:
    helpText = fmt.Sprintf("[c]leanup 1 stale (Enter to remove)  %s", helpText)
case 2, 3, 4, 5, 6, 7, 8, 9:
    helpText = fmt.Sprintf("[c]leanup %d stale  %s", m.staleCount, helpText)
default:
    if m.staleCount > 0 {
        helpText = fmt.Sprintf("[c]leanup %d stale  %s", m.staleCount, helpText)
    }
}
b.WriteString(helpStyle.Render(helpText))
```

In `handleKeyPress` `case "c"`:

```go
case "c":
    if m.staleCount == 1 && len(m.stalePaths) == 1 {
        // Immediate-remove fast path:
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

`staleCount`/`stalePaths` recomputed in `Update` on `worktreesLoadedMsg` (after every refresh, including auto-refresh ticks from #9):

```go
case worktreesLoadedMsg:
    m.worktrees = msg.worktrees
    // ... existing selected clamp logic ...
    branches, _ := m.git.ListBranches()
    branchSet := make(map[string]bool)
    for _, b := range branches {
        branchSet[b] = true
    }
    staleIdxs := classifyStale(m.worktrees, branchSet)
    m.staleCount = len(staleIdxs)
    m.stalePaths = nil
    for _, idx := range staleIdxs {
        m.stalePaths = append(m.stalePaths, m.worktrees[idx].Path)
    }
```

Reuse the `classifyStale` helper from the earlier bug-fix plan (already in `cleanup.go`). No git-layer changes.

### 8. `/` filter mode

Add to `Model` (concatenate with the new fields above):

```go
filterMode  bool
filterText  string
```

Keybind in `handleKeyPress`:

```go
case tea.KeyRunes:
    if m.filterMode {
        // In filter mode, runes append to filterText (NOT switch on runes)
        m.filterText += string(msg.Runes)
        return m, nil
    }
    switch msg.String() {
    // ... existing cases ...
    case "/":
        m.filterMode = true
        m.filterText = ""
        return m, nil
    }
case tea.KeyBackspace:
    if m.filterMode {
        if len(m.filterText) > 0 {
            m.filterText = m.filterText[:len(m.filterText)-1]
        }
        return m, nil
    }
    // ... existing field-branch backspace (only relevant in create modal) ...
case tea.KeyEscape:
    if m.filterMode {
        m.filterMode = false
        m.filterText = ""
        m.selected = 0
        return m, nil
    }
    // ... existing escape handling ...
case tea.KeyEnter:
    if m.filterMode {
        m.filterMode = false // exit filter mode, KEEP filter applied
        return m, nil
    }
    // ... existing Enter handler (path copy + infoMsg) ...
```

For `View()`:

```go
// at the top of the list-view obtain the filtered list
visible := visibleWorktrees(m.worktrees, m.filterText)
indices := visibleIndices(m.worktrees, visible) // indices in m.worktrees matching filter

// the existing for-loop iterates `visible` instead of `m.worktrees`, but
// keeps `m.selected` clamped to `len(visible)-1` (not the total worktrees count)
// since the selected index refers to the visible list
```

Wait — that has an issue: `m.selected` currently refers to `m.worktrees[m.selected]`. Switching to a filtered list breaks every other handler that does `wt := m.worktrees[m.selected]` (delete, cleanup, Enter, `o`). Two alternatives:

**Alternative A (single source of truth):** `m.selected` always refers to `m.worktrees`. Filter is just for display; nav skips indices not matching filter. Render shows only matching rows but `m.selected` is the real index. Nav keys wrap or skip over filtered-out rows. Simplest — no breakage elsewhere.

**Alternative B:** add a `m.filteredWorktrees []model.Worktree` cache for display only; nav works on the filtered cache; other handlers translate filtered index back to `m.worktrees` index via a stored map. More plumbing.

**Use Alternative A:** View iterates `visibleWorktrees()` for **display only**, but selection and all other handlers use `m.worktrees[m.selected]`. Nav keys (j/k/↑/↓) advance by skipping over filtered-out entries. Render highlights only the visible row whose `m.worktrees` index == `m.selected`.

Render the filter input at the bottom of the list view (above the help line when filterMode is active):

```go
if m.filterMode {
    b.WriteString("\n")
    b.WriteString(mutedStyle.Render("/" + m.filterText + "▏")) // cursor block
}
```

When filterMode is OFF but filterText is non-empty (Enter was pressed), show `Filter: <text>` muted at bottom next to help so the user knows filter is applied, and Esc in list mode (without filterMode) clears filter too:

```go
case tea.KeyEscape: // list-mode (not in any modal)
    // If filterText non-empty, Esc clears it
    if !m.filterMode && m.filterText != "" {
        m.filterText = ""
        m.selected = 0
        return m, nil
    }
    // otherwise do nothing (or quit? Existing behavior does nothing in list mode)
```

Pure helpers (unit-tested):

```go
// visibleWorktrees returns the subset of worktrees matching the filter
// (case-insensitive substring on Branch). Empty filter returns all.
func visibleWorktrees(wts []model.Worktree, filter string) []model.Worktree {
    if filter == "" {
        return wts
    }
    var visible []model.Worktree
    needle := strings.ToLower(filter)
    for _, wt := range wts {
        if strings.Contains(strings.ToLower(wt.Branch), needle) {
            visible = append(visible, wt)
        }
    }
    return visible
}
```

`m.selected` semantically: still the index in `m.worktrees`. Nav advances to the next match (skips over filtered-out). Implemented via `advanceSelected(m.selected, +1, m.worktrees, m.filterText)` pure helper.

### 9. Auto-refresh via `tea.Tick`

Add to `internal/tui/app.go`:

```go
type tickMsg time.Time

func autoRefresh() tea.Cmd {
    return tea.Tick(10*time.Second, func(t time.Time) tea.Msg {
        return tickMsg(t)
    })
}
```

`Init()` returns:

```go
func (m Model) Init() tea.Cmd {
    return tea.Batch(m.loadWorktrees, autoRefresh())
}
```

`Update()` handles `tickMsg`:

```go
case tickMsg:
    // Preserve filterText, infoMsg, selected. Just refresh worktrees; re-schedule tick.
    return m, tea.Batch(m.loadWorktrees, autoRefresh())
```

(Note: `loadWorktrees` re-computes `staleCount` via the `worktreesLoadedMsg` handler from #7. `m.selected` is clamped there to the new list length. Filter is applied on display, not on `m.worktrees` — so a refreshed `m.worktrees` won't lose filter state.)

Manual `r` still works, refresh is not deduplicated — if user presses `r` and the next `tickMsg` fires in 9s, both run; cheap and fine.

### 10. Path truncation

In `View()`'s commit line rendering, replace:

```go
b.WriteString(fmt.Sprintf("    %s • %s", commitDisplay, wt.Path))
```

with:

```go
b.WriteString(fmt.Sprintf("    %s • %s", commitDisplay, truncatePath(wt.Path)))
```

Pure helper (unit-tested):

```go
// truncatePath shortens paths longer than 40 chars to "..." + last two segments.
// e.g. "/Users/kjaniec-dev/dev/projects/git-worktree-tui" -> ".../git-worktree-tui"
// (single segment after the head doesn't get truncated since the path is already short)
// Path <= 40 chars: returned as-is.
func truncatePath(path string) string {
    if len(path) <= 40 {
        return path
    }
    parts := strings.Split(path, string(filepath.Separator))
    if len(parts) < 2 {
        return path
    }
    last := parts[len(parts)-1]
    parent := parts[len(parts)-2]
    return "..." + string(filepath.Separator) + parent + string(filepath.Separator) + last
}
```

Full untruncated path is shown via Enter (`Copied: <path>`) or `o` (`Copied: cd <path>`), so users can always see the full path on demand.

## Behavior contracts

- `o` on a selected worktree: attempts OSC 52 copy of `cd <path>`, sets green `m.infoMsg = "Copied: cd <path>"` (or just the text on failure). Clears `m.errMsg`. The `o` keybind only fires in list mode, not in modals.
- Create form location field: renders `Location: <inside | outside>` with active side bold+primary, inactive side muted. `l` keybind unchanged.
- List view: appends `↑N ↓N` after the branch name / `(here)` marker on rows where `Status.Ahead > 0` or `Status.Behind > 0`. Muted-style coloring.
- List footer: shows `[c]leanup N stale (Enter to remove)` for N ≥ 1, prepended to the existing help line. When `staleCount == 1`, pressing `c` immediately removes the stale entry without entering the modal; success → `m.infoMsg = "Removed: <path>"`, failure → `m.errMsg = "Failed to remove ..."`.
- Filter mode toggled by `/`; typing filters the visible worktrees by case-insensitive substring on `Branch`; Esc clears filter+mode, Enter exits mode but keeps filter; backspace deletes; nav keys skip over filtered-out entries. Filter indicator `/` rendered at view bottom when active.
- `Init()` schedules both `m.loadWorktrees` and `autoRefresh()`; `tickMsg` schedules another `loadWorktrees` + `autoRefresh()` batch every 10s. Filter state, infoMsg, selection preserved across refresh.
- List view truncates worktree paths > 40 chars to `.../<parent>/<last>`. Full path accessible via Enter (infoMsg) or `o` (infoMsg).

## Testing

- `visibleWorktrees`: empty filter returns all; non-matching filter returns nil; case-insensitive substring matches; matches on Branch only (not Path).
- `truncatePath`: ≤40 chars returns input; long path returns `.../<parent>/<last>`; single-segment path returns input.
- `staleHintText(count int) string`: 0 → ""; 1 → "[c]leanup 1 stale (Enter to remove)  "; N>1 → "[c]leanup N stale  ".
- `NewModel` extra `cwd` already has tests — re-runs of `TestNewModelWithCWD` should still pass.
- Manual/visual QA (not automated):
  - Run TUI in a terminal. Press `o`. Verify "cd <path>" copied and pasteable in another terminal.
  - In a repo with a stale upstream ahead/behind, verify `↑N ↓N` appears next to the relevant worktree row.
  - Open the create form. Verify `Location: <inside | outside>` renders with active bold.
  - With 1 stale worktree, verify the footer shows the hint and `c` removes immediately without entering the modal.
  - With 2+ stale worktrees, verify the existing modal opens.
  - Press `/`, type a few characters, verify list narrows. Esc clears. Enter keeps filter.
  - Wait 10s without pressing anything; verify the list refreshes silently (worktrees created outside the TUI appear).
  - In a long-path repo, verify the commit line shows `.../parent/last`.

## Out of scope

- New terminal spawning (item 4 has user-approved clipboard-copy approach only).
- fsnotify file watcher (item 9 uses polling per user approval).
- n/N next-match navigation — item 8 uses simple substring filter.
- Color-coding the ahead/behind annotation (kept muted for stability).
- Configurable auto-refresh interval (hardcoded to 10s for now).
- Branch-name legality validation, remote-tracking integration, new modal styling, list-view layout refactoring.
- Git-layer changes (`internal/git/` untouched).