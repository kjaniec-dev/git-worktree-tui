# git-worktree-tui Design Spec

## Overview

TUI application in Go for managing git worktrees. Provides an interactive terminal interface for listing, creating, deleting, and managing git worktrees with status information.

## Technology Stack

- **Language:** Go
- **TUI Framework:** Bubble Tea (Charmbracelet)
- **Components:** Bubbles (table, viewport), Lipgloss (styling)
- **CLI:** Cobra (entry point)
- **Git Integration:** os/exec (direct git commands, no external git libraries)
- **Git Version Requirement:** 2.39+ (full worktree support)

## Architecture

### Project Structure

```
git-worktree-tui/
├── cmd/
│   └── root.go          # Cobra CLI entry point
├── internal/
│   ├── git/
│   │   ├── worktree.go  # Worktree operations (list, add, remove, prune)
│   │   ├── branch.go    # Branch operations
│   │   └── status.go    # Per-worktree status (dirty, ahead/behind)
│   ├── tui/
│   │   ├── app.go       # Main Bubble Tea application
│   │   ├── list.go      # Worktree list view
│   │   ├── create.go    # Create worktree modal
│   │   └── styles.go    # Lipgloss style definitions
│   └── model/
│       └── worktree.go  # Data structures (Worktree, WorktreeStatus)
├── main.go
└── go.mod
```

### Layer Separation

- **git/** - Pure git logic, no TUI dependencies, fully testable
- **tui/** - Bubble Tea model/view/update, presentation only
- **model/** - Shared data structures used by both layers

### Repository Root Detection

- Tool accepts optional `repo-path` CLI argument (defaults to current working directory)
- If `cwd` is not a git repo, traverse upward through parent directories to find `.git`
- If no `.git` found in any parent, exit with error: "Not a git repository"
- If run from a subdirectory of a repo, operate on that repo (not an error)
- Store detected repo root in application state for all git operations

### Git Command Timeout Strategy

- All git commands have 10s timeout (configurable via environment variable `GIT_WORKTREE_TUI_TIMEOUT`)
- On timeout: kill process, show "Command timed out" in status bar, allow retry with `r`
- Background status fetches: 5s timeout per worktree
- Failed/timed-out status: display `?` instead of dirty/clean indicators
- User can refresh (`r`) to retry failed operations

### Concurrency Safety

- Each goroutine for status fetch sends result to a channel: `chan StatusResult`
- `StatusResult` contains either `WorktreeStatus` or `error`
- Main loop collects results with `select` + timeout (5s per worktree max)
- If worktree disappears mid-fetch (error from git), silently ignore result
- No shared mutable state between goroutines - results merged in main thread only

## Data Model

### Worktree Structure

```go
type Worktree struct {
    Path      string           // Full path to worktree (primary identifier)
    Branch    string           // Branch name (e.g., "main", "feature/foo"), or "(detached)" if HEAD detached
    Commit    string           // HEAD commit SHA
    IsMain    bool             // Whether this is the main worktree (origin repo)
    IsLocked  bool             // Whether worktree is locked
    IsBare    bool             // Whether this is a bare worktree (no working directory)
    Detached  bool             // Whether HEAD is detached (no branch checked out)
    Status    *WorktreeStatus  // Optional, loaded on demand
}
```

**Note:** Path is the primary identifier. Moving a worktree manually breaks UI state until refresh.

### WorktreeStatus Structure

```go
type WorktreeStatus struct {
    IsDirty      bool  // Uncommitted changes present
    Ahead        int   // Commits ahead of upstream
    Behind       int   // Commits behind upstream
    HasStash     bool  // Stashed changes present
    Untracked    int   // Number of untracked files
}
```

### Design Decisions

- `Worktree` is an immutable snapshot parsed from `git worktree list --porcelain`
- `WorktreeStatus` loaded optionally (expensive - requires `git status` per worktree)
- `IsMain` distinguishes main worktree (cannot be removed)
- `IsLocked` is important - locked worktrees should not be deleted

## TUI Design

### Main Screen - Worktree List

```
┌─ git-worktree-tui ──────────────────────────────────────┐
│                                                          │
│  ● main                    /Users/.../project           │
│    abc1234 • clean • ↑2                                  │
│                                                          │
│  → feature/auth          /Users/.../project-auth         │
│    def5678 • 3 modified • ↑5 ↓2                         │
│                                                          │
│    feature/ui-redesign    /Users/.../project-ui          │
│    9ab0123 • clean                                       │
│                                                          │
│  🔒 hotfix/critical       /Users/.../project-hotfix      │
│    4cd5678 • locked                                      │
│                                                          │
├──────────────────────────────────────────────────────────┤
│ [a]dd [d]elete [s]witch [c]leanup [r]efresh [q]uit      │
└──────────────────────────────────────────────────────────┘
```

### Keybindings

| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate worktree list |
| `Enter` | Show full path for selected worktree (copy to clipboard if possible) |
| `a` | New worktree (opens modal) |
| `d` | Delete selected worktree (with confirmation) |
| `c` | Cleanup - delete worktrees for non-existent branches |
| `r` | Refresh - reload list and statuses |
| `q` / `Ctrl+C` | Quit |

**Note:** "Switch" action removed - user can see the path and `cd` manually. TUI cannot change parent shell's working directory.

### Create Worktree Modal

```
┌─ Create Worktree ─────────────────────────────────────────┐
│                                                           │
│  Branch name: [feature/my-feature            ]           │
│  Base:        [main ▼]                                   │
│  ☐ Create new branch from base                           │
│                                                           │
│  Path: /Users/.../project-my-feature (auto)              │
│                                                           │
│            [Create]          [Cancel]                     │
└───────────────────────────────────────────────────────────┘
```

**Path Auto-Generation Logic:**
- Base directory = parent of main worktree (git repo root's parent)
- Path pattern = `{repo-root-parent}/{repo-name}-{branch-name-with-slashes-as-dashes}`
- Example: repo at `/Users/dev/myproject`, branch `feature/auth` → `/Users/dev/myproject-feature-auth`
- User can edit path manually in the modal before confirming

### Delete Confirmation Modal

```
┌─ Delete Worktree ─────────────────────────────────────────┐
│                                                           │
│  Delete worktree for "feature/auth"?                     │
│  Path: /Users/.../project-auth                           │
│                                                           │
│  ⚠ This will remove the worktree directory.              │
│  ⚠ Uncommitted changes will be lost.                     │
│                                                           │
│            [Delete]          [Cancel]                     │
└───────────────────────────────────────────────────────────┘
```

### Visual Indicators

- `●` green - clean worktree (git status --porcelain=v2 returns empty)
- `●` yellow - dirty (non-empty output from git status --porcelain=v2)
- `→` cyan - currently selected/highlighted in list
- `🔒` - locked worktree (displayed as prefix)
- `↑N ↓M` - ahead/behind from upstream (only if upstream configured)
- `(bare)` - bare worktree suffix (no working directory, cannot have status)
- `(detached)` - detached HEAD state (no branch checked out)
- Gray text - worktree without upstream or status fetch failed
- `?` - status unknown (fetch failed or timed out)

**Implementation:** Use Lipgloss-styled text, not emoji (better cross-platform compatibility). Symbols like `●` and `🔒` are Unicode characters rendered via terminal font.

## Data Flow

### Startup Sequence

1. Detect repository root (traverse upward from cwd to find .git)
2. Run `git worktree list --porcelain` in main repo directory
3. Parse output → `[]Worktree`
4. For each worktree: spawn goroutine to run `git status --porcelain=v2` with 5s timeout
5. Collect results via channel with timeout handling
6. Update model with `WorktreeStatus` (or mark as `?` if failed/timed out)
7. Render list

### Actions

- **Add:** Modal → validation → `git worktree add <path> <branch>` → refresh list
- **Delete:** Confirmation → `git worktree remove <path>` → refresh list
- **Cleanup:** See detailed flow below
- **Refresh:** Reload list + statuses

### Cleanup Flow (Detailed)

1. User presses `c`
2. Tool runs `git branch --list` to get all local branches
3. Compare branch list against worktree list
4. Show modal: "Worktrees with missing branches:" + list of affected worktrees
5. User can select which to remove (multi-select with space, or "remove all")
6. For each selected worktree:
   - If dirty → show warning, require explicit confirmation
   - If locked → skip, show info "locked, skipping"
   - Otherwise → `git worktree remove <path>`
7. Refresh list after cleanup completes

### Modal Keyboard Navigation

| Key | Action |
|-----|--------|
| `Tab` | Next field |
| `Shift+Tab` | Previous field |
| `Enter` | Confirm (Create/Delete) or submit form |
| `Escape` | Cancel modal |
| `↑/↓` | Navigate dropdown options or list items |
| `Space` | Toggle checkbox or select item in multi-select |

## Error Handling

### Error Scenarios

| Scenario | Behavior |
|----------|----------|
| Not in a git repo | Error on startup: "Not a git repository", exit 1 |
| Git version < 2.39 | Warning on startup, but allow to run (degraded) |
| Branch already exists on create | Error in modal: "Branch already exists" |
| Path already exists on create | Error in modal: "Directory already exists" |
| Permission denied (create/delete) | Error: "Cannot create/remove directory: permission denied" |
| Disk full | Error: "Cannot create worktree: no space left on device" |
| Path exists but is a file (not dir) | Error: "Path exists but is not a directory" |
| Worktree has uncommitted changes on delete | Warning in confirmation modal (red text) |
| Git command fails | Show stderr in status bar at bottom (5s timeout) |
| Git command times out | Show "Command timed out" in status bar, allow retry with `r` |
| Worktree locked on delete | Block action, show info: "Worktree is locked, unlock first" |

### Error Handling Pattern

```go
// In git/ layer - return error with context
func (g *GitService) RemoveWorktree(path string) error {
    cmd := exec.Command("git", "worktree", "remove", path)
    output, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("failed to remove worktree %s: %w\n%s", path, err, output)
    }
    return nil
}

// In tui/ layer - catch error, display in UI
func (m Model) handleDelete() (Model, tea.Cmd) {
    err := m.git.RemoveWorktree(selected.Path)
    if err != nil {
        m.errorMsg = err.Error()
        m.errorTimeout = time.Now().Add(5 * time.Second)
        return m, nil
    }
    // success → refresh
    return m, m.refreshList()
}
```

## Features

### Core Features

1. **List worktrees** - Display all worktrees with path, branch, commit, status
2. **Create worktree** - Interactive modal to create new worktree with branch
3. **Delete worktree** - Remove worktree with confirmation and safety checks
4. **Switch worktree** - Navigate to selected worktree (information display)
5. **Cleanup** - Remove worktrees for deleted branches
6. **Status per worktree** - Show dirty/clean state, ahead/behind counts

### Future Extensions (Out of Scope)

- Branch management (create, delete, rename branches)
- Stash management per worktree
- Diff viewer
- Merge/rebase operations
- Remote operations (push, pull, fetch)

## Testing Strategy

### Unit Tests

- `git/` layer: Test parsing of `git worktree list --porcelain` output
- `git/` layer: Test status parsing from `git status --porcelain=v2`
- `model/` layer: Test data structure validation

### Integration Tests

- Mock git commands to test error handling
- Test worktree creation/deletion flow with mock git

### Manual Testing

- Test in real git repository with multiple worktrees
- Verify all keybindings work correctly
- Test error scenarios (not in repo, locked worktree, etc.)

## Success Criteria

1. Application starts and displays all worktrees in current repo
2. User can create new worktree with branch via modal
3. User can delete worktree with confirmation
4. User can run cleanup to remove stale worktrees
5. Status information (dirty/clean, ahead/behind) displays correctly
6. All error scenarios handled gracefully with user-friendly messages
7. Application exits cleanly on `q` or `Ctrl+C`
