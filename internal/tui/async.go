package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kjaniec-dev/git-worktree-tui/internal/git"
)

// This file holds the async tea.Cmd wrappers + result-message handlers for
// git operations that can take noticeable time (worktree add/remove/lock and
// bulk cleanup). Each Cmd runs the git call off the Update loop and reports
// back via a typed *ResultMsg so the busy spinner can be shown meanwhile.

// createResultMsg reports the outcome of an async AddWorktree call.
type createResultMsg struct {
	err error
}

func addWorktreeCmd(g *git.GitService, path, branch, base string, createBranch bool) tea.Cmd {
	return func() tea.Msg {
		err := g.AddWorktree(path, branch, base, createBranch)
		return createResultMsg{err: err}
	}
}

func (m Model) handleCreateResult(msg createResultMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	if msg.err != nil {
		m.create.errMsg = msg.err.Error()
		return m, nil
	}
	m.mode = modeList
	return m, m.loadWorktrees
}

// deleteResultMsg reports the outcome of an async RemoveWorktree call
// (optionally followed by a branch delete).
type deleteResultMsg struct {
	branch       string
	deleteBranch bool
	removeErr    error
	branchDelErr error
}

func removeWorktreeCmd(g *git.GitService, path, branch string, force, alsoDeleteBranch bool) tea.Cmd {
	return func() tea.Msg {
		removeErr := g.RemoveWorktree(path, force)
		if removeErr != nil {
			return deleteResultMsg{branch: branch, removeErr: removeErr}
		}
		var branchDelErr error
		if alsoDeleteBranch && branch != "" {
			// Always a safe (-d) delete here, regardless of `force`: force
			// means "the worktree had uncommitted file changes", which says
			// nothing about whether the branch has commits that live
			// nowhere else. Reusing it to force-delete (-D) the branch
			// would silently discard real, possibly-unmerged commits.
			branchDelErr = g.DeleteBranch(branch, false)
		}
		return deleteResultMsg{branch: branch, deleteBranch: alsoDeleteBranch, branchDelErr: branchDelErr}
	}
}

func (m Model) handleDeleteResult(msg deleteResultMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	if msg.removeErr != nil {
		m.errMsg = msg.removeErr.Error()
		return m, nil
	}
	m.mode = modeList
	if msg.deleteBranch {
		if msg.branchDelErr != nil {
			m.errMsg = fmt.Sprintf("Worktree removed, but failed to delete branch %q: %v", msg.branch, msg.branchDelErr)
		} else {
			m.infoMsg = fmt.Sprintf("Removed worktree and branch %q", msg.branch)
		}
	}
	return m, m.loadWorktrees
}

// cleanupResultMsg reports accumulated failures from a bulk cleanup removal,
// plus how many worktrees were attempted (for the success message).
type cleanupResultMsg struct {
	attempted int
	errs      []string
}

// cleanupWorktreesCmd removes every worktree in targets (paths + branches),
// best-effort deleting each backing branch too, and accumulates any
// per-worktree errors instead of stopping at the first failure.
func cleanupWorktreesCmd(g *git.GitService, paths, branches []string) tea.Cmd {
	return func() tea.Msg {
		var errs []string
		for i, path := range paths {
			if err := g.RemoveWorktree(path, false); err != nil {
				errs = append(errs, fmt.Sprintf("failed to remove %s: %v", path, err))
				continue
			}
			if i < len(branches) && branches[i] != "" {
				_ = g.DeleteBranch(branches[i], false)
			}
		}
		return cleanupResultMsg{attempted: len(paths), errs: errs}
	}
}

func (m Model) handleCleanupResult(msg cleanupResultMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	m.mode = modeList
	if len(msg.errs) > 0 {
		m.errMsg = joinErrs(msg.errs)
		return m, m.loadWorktrees
	}
	m.errMsg = ""
	removed := msg.attempted - len(msg.errs)
	if removed == 1 {
		m.infoMsg = "Removed 1 worktree"
	} else if removed > 1 {
		m.infoMsg = fmt.Sprintf("Removed %d worktrees", removed)
	}
	return m, m.loadWorktrees
}

// lockResultMsg reports the outcome of an async lock/unlock call.
type lockResultMsg struct {
	branch  string
	wasLock bool // true if this call was an unlock (worktree was locked before)
	err     error
}

func lockWorktreeCmd(g *git.GitService, path, branch string, currentlyLocked bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if currentlyLocked {
			err = g.UnlockWorktree(path)
		} else {
			err = g.LockWorktree(path)
		}
		return lockResultMsg{branch: branch, wasLock: currentlyLocked, err: err}
	}
}

func (m Model) handleLockResult(msg lockResultMsg) (tea.Model, tea.Cmd) {
	m.busy = false
	if msg.err != nil {
		m.errMsg = msg.err.Error()
		return m, nil
	}
	if msg.wasLock {
		m.infoMsg = fmt.Sprintf("Unlocked: %s", msg.branch)
	} else {
		m.infoMsg = fmt.Sprintf("Locked: %s", msg.branch)
	}
	return m, m.loadWorktrees
}

func joinErrs(errs []string) string {
	out := errs[0]
	for _, e := range errs[1:] {
		out += "; " + e
	}
	return out
}
