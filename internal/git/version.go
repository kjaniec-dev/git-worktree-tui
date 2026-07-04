// internal/git/version.go
package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
)

type GitVersion struct {
	Major int
	Minor int
	Patch int
}

func (g *GitService) GetVersion() (*GitVersion, error) {
	cmd := exec.Command("git", "--version")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git version: %w", err)
	}

	// Parse "git version 2.39.0"
	re := regexp.MustCompile(`git version (\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(string(output))
	if len(matches) != 4 {
		return nil, fmt.Errorf("failed to parse git version: %s", output)
	}

	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	patch, _ := strconv.Atoi(matches[3])

	return &GitVersion{Major: major, Minor: minor, Patch: patch}, nil
}

func (v *GitVersion) IsAtLeast(major, minor int) bool {
	if v.Major > major {
		return true
	}
	if v.Major == major && v.Minor >= minor {
		return true
	}
	return false
}
