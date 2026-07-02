package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
	"github.com/kjaniec-dev/git-worktree-tui/internal/model"
)

func TestNewModel(t *testing.T) {
	gitService := git.NewGitService("/tmp/test")
	m := NewModel(gitService, "/tmp/test")

	if m.git == nil {
		t.Error("Expected git service to be set")
	}
	if m.selected != 0 {
		t.Errorf("Expected selected to be 0, got %d", m.selected)
	}
	if m.mode != modeList {
		t.Errorf("Expected mode to be modeList, got %v", m.mode)
	}
}

func TestNewModelWithCWD(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/custom/cwd")
	if m.cwd != "/custom/cwd" {
		t.Errorf("m.cwd = %q, want /custom/cwd", m.cwd)
	}
}

func TestHereMarkerOnWorktreeRoot(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/wt/feat")
	m.worktrees = []model.Worktree{
		{Path: "/wt/main", Branch: "main"},
		{Path: "/wt/feat", Branch: "feat"},
	}
	m.mode = modeList
	view := m.View()
	if !strings.Contains(view, "feat (here)") {
		t.Errorf("expected 'feat (here)' in view, got:\n%s", view)
	}
	if strings.Contains(view, "main (here)") {
		t.Errorf("did not expect (here) on 'main' row, got:\n%s", view)
	}
	if c := strings.Count(view, "(here)"); c != 1 {
		t.Errorf("expected exactly 1 (here) marker, got %d:\n%s", c, view)
	}
}

func TestHereMarkerInSubdirectory(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/wt/feat/subdir")
	m.worktrees = []model.Worktree{
		{Path: "/wt/main", Branch: "main"},
		{Path: "/wt/feat", Branch: "feat"},
	}
	m.mode = modeList
	view := m.View()
	if !strings.Contains(view, "feat (here)") {
		t.Errorf("expected (here) on feat even when CWD is a subdirectory of /wt/feat:\n%s", view)
	}
	if c := strings.Count(view, "(here)"); c != 1 {
		t.Errorf("expected exactly 1 (here) marker, got %d:\n%s", c, view)
	}
}

func TestInitialBase(t *testing.T) {
	tests := []struct {
		name     string
		branches []string
		wantBase string
		wantIdx  int
	}{
		{"empty -> main default", []string{}, "main", 0},
		{"main at 0", []string{"main", "develop"}, "main", 0},
		{"main not at 0 -> index of main", []string{"develop", "main", "feat"}, "main", 1},
		{"master when no main", []string{"develop", "master"}, "master", 1},
		{"main preferred over master", []string{"master", "main"}, "main", 1},
		{"no main/master -> first branch", []string{"develop", "feat"}, "develop", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base, idx := initialBase(tt.branches)
			if base != tt.wantBase {
				t.Errorf("base = %q, want %q", base, tt.wantBase)
			}
			if idx != tt.wantIdx {
				t.Errorf("idx = %d, want %d", idx, tt.wantIdx)
			}
		})
	}
}

func TestEmptyListNavigationNoOp(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = nil
	m.selected = 0
	m.mode = modeList

	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'j'}},
		{Type: tea.KeyRunes, Runes: []rune{'k'}},
		{Type: tea.KeyDown},
		{Type: tea.KeyUp},
	} {
		out, _ := m.handleKeyPress(msg)
		mm := out.(Model)
		if mm.selected != 0 {
			t.Errorf("after %v on empty list, selected = %d, want 0", msg, mm.selected)
		}
	}
}

func TestStatusGlyphs(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.worktrees = []model.Worktree{
		{Path: "/p/locked", Branch: "b", IsLocked: true},
		{Path: "/p/main", Branch: "main", IsMain: true},
		{Path: "/p/dirty", Branch: "d", Status: &model.WorktreeStatus{IsDirty: true}},
		{Path: "/p/clean", Branch: "c", Status: &model.WorktreeStatus{IsDirty: false}},
		{Path: "/p/unknown", Branch: "u"}, // Status == nil -> not loaded
	}
	m.mode = modeList
	view := m.View()

	cases := []struct{ state, glyph string }{
		{"locked", "🔒"},
		{"main", "★"},
		{"dirty", "●"},
		{"clean", "○"},
		{"unknown", "?"},
	}
	for _, c := range cases {
		if !strings.Contains(view, c.glyph) {
			t.Errorf("expected glyph %q for %s worktree in view, got:\n%s", c.glyph, c.state, view)
		}
	}
	// Sanity: dirty and clean must produce DIFFERENT view strings
	if strings.Contains(view, "● c") || strings.Contains(view, "○ d") {
		t.Errorf("dirty/clean glyphs collided:\n%s", view)
	}
}
