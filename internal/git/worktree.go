package git

import (
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
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = g.RepoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list worktrees: %w\n%s", err, output)
	}

	return parseWorktreeList(string(output))
}