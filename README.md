# git-worktree-tui

A terminal UI for managing git worktrees.

## Features

- List all worktrees with status (clean/dirty, ahead/behind), with automatic scrolling for long lists
- Create new worktrees inside repo at `.worktrees/<branch-name>`
- Delete worktrees with confirmation, optionally deleting the backing branch too
- Lock/unlock worktrees to protect them from accidental removal
- Cleanup worktrees whose branch was deleted *or* fully merged into the base branch
- In-app help screen (`?`) with the full keybinding list and status-glyph legend
- Async git operations with a progress spinner — the UI never freezes on slow operations
- Shell integration (`g`) to `cd` straight into a worktree when the TUI exits
- Fast, keyboard-driven interface

## Installation

```bash
go install github.com/kjaniec-dev/git-worktree-tui@latest
```

### Build without Go (Docker)

If you don't have Go installed, you can build the binary using Docker:

```bash
docker run --rm -v $(pwd):/app -w /app golang:1.26-alpine \
  sh -c "CGO_ENABLED=0 GOOS=$(uname -s | tr '[:upper:]' '[:lower:]') GOARCH=$(uname -m | sed 's/x86_64/amd64/') go build -o git-worktree-tui"
```

This cross-compiles a native binary for your OS and architecture — no Go installation needed. Run it with `./git-worktree-tui`.

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
- `d` - Delete selected worktree (`b` in the confirmation to also delete the branch)
- `l` - Lock/unlock selected worktree
- `c` - Cleanup worktrees with a deleted or merged branch
- `g` - Print path & quit, for `cd`-on-exit shell integration (see below)
- `o` - Copy `cd <path>` to clipboard
- `/` - Filter the list by branch name
- `r` - Refresh list (also happens automatically every 10s)
- `?` - Toggle the in-app help screen
- `q` - Quit

Press `?` at any time to see the full keybinding list and the glyph legend (★ main, 🔒 locked, ● dirty, ○ clean) without leaving the app.

### Shell integration

Pressing `g` quits the TUI and prints the selected worktree's path to stdout — but a child process can't change your shell's working directory, so wrap it in a shell function to actually `cd` there.

Add to your `~/.zshrc` or `~/.bashrc`:

```bash
gwt() {
  local dir
  dir=$(git-worktree-tui) && [ -n "$dir" ] && cd "$dir"
}
```

Then run `gwt` instead of `git-worktree-tui` directly: navigate to a worktree and press `g`, and your shell will `cd` into it once the TUI closes. Quitting with `q` prints nothing, so the function is a no-op in that case.

## Requirements

- Git 2.39+ (for full worktree support)

## License

MIT