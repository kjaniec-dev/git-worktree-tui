# git-worktree-tui

A terminal UI for managing git worktrees.

## Features

- List all worktrees with status (clean/dirty, ahead/behind)
- Create new worktrees inside repo at `.worktrees/<branch-name>`
- Delete worktrees with confirmation
- Cleanup stale worktrees
- Fast, keyboard-driven interface

## Installation

```bash
go install github.com/kjaniec-dev/git-worktree-tui@latest
```

### Build without Go (Docker)

If you don't have Go installed, you can build the binary using Docker:

```bash
docker run --rm -v $(pwd):/app -w /app golang:1.26-alpine \
  sh -c "CGO_ENABLED=0 go build -o git-worktree-tui"
```

This produces a native `git-worktree-tui` binary in your current directory — no Go installation needed. Run it with `./git-worktree-tui`.

## Usage

Run from any git repository:

```bash
git-worktree-tui
```

Worktrees are created inside your repo at `.worktrees/<branch-name>` (e.g., `.worktrees/feature-auth`). This directory is automatically added to `.gitignore`.

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