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