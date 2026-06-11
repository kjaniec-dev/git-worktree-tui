package git

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
)

// GitService handles git operations
type GitService struct {
	RepoRoot string
	Timeout  time.Duration
}

// NewGitService creates a new GitService
func NewGitService(repoRoot string) *GitService {
	return &GitService{
		RepoRoot: repoRoot,
		Timeout:  10 * time.Second,
	}
}

func (g *GitService) runGitCommand(args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), g.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.RepoRoot

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("command timed out after %v", g.Timeout)
	}
	if err != nil {
		return nil, fmt.Errorf("%w\n%s", err, output)
	}

	return output, nil
}

func (g *GitService) runGitCommandInDir(dir string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), g.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return nil, fmt.Errorf("command timed out after %v", g.Timeout)
	}
	if err != nil {
		return nil, fmt.Errorf("%w\n%s", err, output)
	}

	return output, nil
}

// parseWorktreeList parses the output of git worktree list --porcelain
func parseWorktreeList(output string) ([]model.Worktree, error) {
	var worktrees []model.Worktree
	var current *model.Worktree

	lines := strings.Split(output, "\n")
	isFirst := true

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if current != nil {
				worktrees = append(worktrees, *current)
				current = nil
			}
			continue
		}

		if strings.HasPrefix(line, "worktree ") {
			current = &model.Worktree{
				Path:   strings.TrimPrefix(line, "worktree "),
				IsMain: isFirst,
			}
			isFirst = false
		} else if strings.HasPrefix(line, "HEAD ") && current != nil {
			current.Commit = strings.TrimPrefix(line, "HEAD ")
		} else if strings.HasPrefix(line, "branch ") && current != nil {
			branch := strings.TrimPrefix(line, "branch ")
			// Remove refs/heads/ prefix
			branch = strings.TrimPrefix(branch, "refs/heads/")
			current.Branch = branch
		} else if line == "detached" && current != nil {
			current.Detached = true
			current.Branch = "(detached)"
		} else if line == "locked" && current != nil {
			current.IsLocked = true
		} else if line == "bare" && current != nil {
			current.IsBare = true
		}
	}

	// Handle last worktree if no trailing newline
	if current != nil {
		worktrees = append(worktrees, *current)
	}

	return worktrees, nil
}

// ListWorktrees returns all worktrees in the repository
func (g *GitService) ListWorktrees() ([]model.Worktree, error) {
	output, err := g.runGitCommand("worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w", err)
	}

	return parseWorktreeList(string(output))
}

// AddWorktree creates a new worktree
func (g *GitService) AddWorktree(path, branch, base string, createBranch bool) error {
	var args []string
	args = append(args, "worktree", "add")

	if createBranch {
		args = append(args, "-b", branch)
	}

	args = append(args, path)

	if createBranch {
		args = append(args, base)
	} else {
		args = append(args, branch)
	}

	output, err := g.runGitCommand(args...)
	if err != nil {
		return fmt.Errorf("failed to add worktree: %w", err)
	}

	_ = output // suppress unused warning
	return nil
}

// RemoveWorktree removes a worktree
func (g *GitService) RemoveWorktree(path string) error {
	output, err := g.runGitCommand("worktree", "remove", path)
	if err != nil {
		return fmt.Errorf("failed to remove worktree %s: %w", path, err)
	}

	_ = output // suppress unused warning
	return nil
}

// PruneWorktrees removes stale worktree metadata
func (g *GitService) PruneWorktrees() error {
	output, err := g.runGitCommand("worktree", "prune")
	if err != nil {
		return fmt.Errorf("failed to prune worktrees: %w", err)
	}

	_ = output // suppress unused warning
	return nil
}