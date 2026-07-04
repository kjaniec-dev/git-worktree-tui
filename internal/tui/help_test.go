package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
)

func TestQuestionMarkOpensHelp(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.mode = modeList

	out, _ := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	mm := out.(Model)
	if mm.mode != modeHelp {
		t.Errorf("mode = %v, want modeHelp", mm.mode)
	}
}

func TestHelpModalContainsKeybindingsAndGlyphs(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.mode = modeHelp

	view := m.View()
	for _, want := range []string{"Enter", "Create new worktree", "★", "Main worktree", "🔒", "Locked worktree"} {
		if !strings.Contains(view, want) {
			t.Errorf("expected help view to contain %q, got:\n%s", want, view)
		}
	}
}

func TestHelpEscapeReturnsToList(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.mode = modeHelp

	out, _ := m.handleHelpKeyPress(tea.KeyMsg{Type: tea.KeyEscape})
	mm := out.(Model)
	if mm.mode != modeList {
		t.Errorf("mode = %v, want modeList after Esc", mm.mode)
	}
}

func TestHelpQuestionMarkClosesHelp(t *testing.T) {
	m := NewModel(git.NewGitService("/tmp/test"), "/tmp/test")
	m.mode = modeHelp

	out, _ := m.handleHelpKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	mm := out.(Model)
	if mm.mode != modeList {
		t.Errorf("mode = %v, want modeList after ?", mm.mode)
	}
}
