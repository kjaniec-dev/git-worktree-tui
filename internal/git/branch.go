package git

import (
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
	cmd := exec.Command("git", "branch", "--list")
	cmd.Dir = g.RepoRoot

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w\n%s", err, output)
	}

	return parseBranchList(string(output)), nil
}

// BranchExists checks if a branch exists
func (g *GitService) BranchExists(branch string) (bool, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", branch)
	cmd.Dir = g.RepoRoot

	err := cmd.Run()
	return err == nil, nil
}