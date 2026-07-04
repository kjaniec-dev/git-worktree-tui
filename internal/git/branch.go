package git

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// parseBranchList parses the output of git branch --list
func parseBranchList(output string) []string {
	var branches []string
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Remove leading* or space
		if strings.HasPrefix(line, "* ") {
			line = strings.TrimPrefix(line, "* ")
		} else if strings.HasPrefix(line, "  ") {
			line = strings.TrimPrefix(line, "  ")
		}
		branches = append(branches, line)
	}

	return branches
}

// ListBranches returns all local branches
func (g *GitService) ListBranches() ([]string, error) {
	output, err := g.runGitCommand("branch", "--list")
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	return parseBranchList(string(output)), nil
}

// MergedBranches returns local branches that are fully merged into base
// (i.e. `git branch --merged <base>`), including base itself.
func (g *GitService) MergedBranches(base string) ([]string, error) {
	output, err := g.runGitCommand("branch", "--merged", base)
	if err != nil {
		return nil, fmt.Errorf("failed to list merged branches: %w", err)
	}

	return parseBranchList(string(output)), nil
}

// buildDeleteBranchArgs constructs `git branch` delete args: -D when force,
// -d (safe delete, refuses on unmerged work) otherwise.
func buildDeleteBranchArgs(branch string, force bool) []string {
	flag := "-d"
	if force {
		flag = "-D"
	}
	return []string{"branch", flag, branch}
}

// DeleteBranch deletes a local branch. With force=false, git refuses to
// delete branches with unmerged commits (safe default); force=true deletes
// regardless.
func (g *GitService) DeleteBranch(branch string, force bool) error {
	output, err := g.runGitCommand(buildDeleteBranchArgs(branch, force)...)
	if err != nil {
		return fmt.Errorf("failed to delete branch %q: %w", branch, err)
	}

	_ = output
	return nil
}

// BranchExists reports whether a *local branch* named <branch> exists.
// Tags and other refs do not count. A nonzero exit from rev-parse means
// the branch does not exist (not an error). Real failures (timeout, git
// missing) are surfaced as errors.
func (g *GitService) BranchExists(branch string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), g.Timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	cmd.Dir = g.RepoRoot

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return false, nil // branch not found
		}
		return false, fmt.Errorf("failed to verify branch %q: %w", branch, err)
	}
	return true, nil
}
