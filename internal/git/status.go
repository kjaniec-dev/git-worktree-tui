package git

import (
	"fmt"
	"strings"

	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
)

// parseStatus parses the output of git status --porcelain=v2
func parseStatus(output string) model.WorktreeStatus {
	status := model.WorktreeStatus{}
	
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Check for untracked files (start with ?)
		if strings.HasPrefix(line, "?") {
			status.Untracked++
			status.IsDirty = true
			continue
		}
		
		// Any other line indicates a change
		if len(line) > 0 {
			status.IsDirty = true
		}
	}
	
	return status
}

// GetWorktreeStatus returns the status of a worktree
func (g *GitService) GetWorktreeStatus(worktreePath string) (*model.WorktreeStatus, error) {
	output, err := g.runGitCommandInDir(worktreePath, "status", "--porcelain=v2", "--branch", "--ahead-behind")
	if err != nil {
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	status := parseStatus(string(output))

	// Parse ahead/behind from output
	// Format: # branch.ab +N -M
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# branch.ab") {
			parts := strings.Fields(line)
			for _, part := range parts {
				if strings.HasPrefix(part, "+") {
					fmt.Sscanf(part, "+%d", &status.Ahead)
				} else if strings.HasPrefix(part, "-") {
					fmt.Sscanf(part, "-%d", &status.Behind)
				}
			}
		}
	}

	return &status, nil
}