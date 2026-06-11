package git

import (
	"testing"
)

func TestParseBranchList(t *testing.T) {
	output := `  feature/auth
* main
  feature/ui
`

	branches := parseBranchList(output)
	
	if len(branches) != 3 {
		t.Fatalf("Expected 3 branches, got %d", len(branches))
	}

	expected := []string{"feature/auth", "main", "feature/ui"}
	for i, branch := range branches {
		if branch != expected[i] {
			t.Errorf("Expected branch %s, got %s", expected[i], branch)
		}
	}
}