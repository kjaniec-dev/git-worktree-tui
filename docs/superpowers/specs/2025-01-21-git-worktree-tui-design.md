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

## Data Model

### Worktree Structure

```go
type Worktree struct {
    Path      string           // Full path to worktree
    Branch    string           // Branch name (e.g., "main", "feature/foo")
    Commit    string           // HEAD commit SHA
    IsMain    bool             // Whether this is the main worktree (origin repo)
    IsLocked  bool             // Whether worktree is locked
    IsBare    bool             // Whether this is a bare worktree
    Status    *WorktreeStatus  // Optional, loaded on demand
}
```

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
| `Enter` | Switch to selected worktree (cd to it) |
| `a` | New worktree (opens modal) |
| `d` | Delete selected worktree (with confirmation) |
| `c` | Cleanup - delete worktrees for non-existent branches |
| `r` | Refresh - reload list and statuses |
| `q` / `Ctrl+C` | Quit |

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

- `●` green - clean worktree
- `●` yellow - dirty (uncommitted changes)
- `→` cyan - currently selected (highlight)
- `🔒` - locked worktree
- `↑N ↓M` - ahead/behind from upstream
- Gray - worktree without upstream

## Data Flow

### Startup Sequence

1. Run `git worktree list --porcelain` in main repo directory
2. Parse output → `[]Worktree`
3. For each worktree: run `git status --porcelain=v2` in background (goroutine)
4. Update model with `WorktreeStatus`
5. Render list

### Actions

- **Add:** Modal → validation → `git worktree add <path> <branch>` → refresh list
- **Delete:** Confirmation → `git worktree remove <path>` → refresh list
- **Cleanup:** `git worktree prune` + iterate list → prompt per non-existent branch → remove
- **Switch:** `cd <worktree path>` (information only, not actual cd from TUI)
- **Refresh:** Reload list + statuses

## Error Handling

### Error Scenarios

| Scenario | Behavior |
|----------|----------|
| Not in a git repo | Error on startup: "Not a git repository", exit 1 |
| Git version < 2.39 | Warning on startup, but allow to run (degraded) |
| Branch already exists on create | Error in modal: "Branch already exists" |
| Worktree has uncommitted changes on delete | Warning in confirmation modal (red text) |
| Git command fails | Show stderr in status bar at bottom (5s timeout) |
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
