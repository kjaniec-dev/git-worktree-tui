// cmd/root.go
package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/bubbletea"
	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
	"github.com/kjaniec-dev/git-worktree-tui/internal/tui"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "git-worktree-tui",
	Short: "TUI for managing git worktrees",
	RunE:  run,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// Detect repository root
	repoRoot, err := findRepoRoot()
	if err != nil {
		return fmt.Errorf("not a git repository: %w", err)
	}

	// Create git service
	gitService := git.NewGitService(repoRoot)

	// Check git version
	version, err := gitService.GetVersion()
	if err != nil {
		return fmt.Errorf("failed to get git version: %w", err)
	}

	if !version.IsAtLeast(2, 39) {
		fmt.Fprintf(os.Stderr, "Warning: git %d.%d.%d detected, 2.39+ recommended\n",
			version.Major, version.Minor, version.Patch)
	}

	// Create and run TUI
	model := tui.NewModel(gitService)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running program: %w", err)
	}

	return nil
}

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		gitDir := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("not a git repository")
		}
		dir = parent
	}
}