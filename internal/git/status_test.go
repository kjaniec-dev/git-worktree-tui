package git

import (
	"testing"
)

func TestParseStatusClean(t *testing.T) {
	output := `` // Empty output = clean
	
	status := parseStatus(output)
	
	if status.IsDirty {
		t.Error("Expected clean worktree")
	}
	if status.Untracked != 0 {
		t.Errorf("Expected 0 untracked, got %d", status.Untracked)
	}
}

func TestParseStatusDirty(t *testing.T) {
	output := `1 .M N... 100644 100644 100644 abc123 def456 file.txt
? untracked.txt
`
	
	status := parseStatus(output)
	
	if !status.IsDirty {
		t.Error("Expected dirty worktree")
	}
	if status.Untracked != 1 {
		t.Errorf("Expected 1 untracked, got %d", status.Untracked)
	}
}

func TestParseAheadBehind(t *testing.T) {
	// Test ahead/behind parsing (simplified - actual implementation uses git rev-list)
	// This is a placeholder for the actual parsing logic
}