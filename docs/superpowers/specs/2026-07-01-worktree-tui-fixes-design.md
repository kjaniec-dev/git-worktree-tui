# git-worktree-tui Fixes — Design Spec

- Date: 2026-07-01
- Status: Approved (pending spec review)
- Scope: Bug fixes across `internal/tui/create.go`, `internal/tui/delete.go`, `internal/tui/cleanup.go`, `internal/tui/app.go`, `internal/git/worktree.go`, `internal/git/branch.go`. No new flows. No unrelated refactoring.

## Problem

The worktree TUI has bugs in create, delete, and cleanup flows that make it fail (crashes/confusing errors), behave wrongly (wrong base branch / wrong stale detection), and is unsafe for users who don't know what a worktree is.

### Bugs

1. **Create — base selector is broken.** `createModel.baseIndex` is initialized to the zero value `0`, while `baseBranch` is initialized to `main`/`master` (or `branches[0]`) in `NewModel`. When `main` is not at index 0, the displayed base and the selector index are out of sync, so ↑/↓ jumps to the wrong entries and the user cannot reliably land on the desired base branch. (`internal/tui/app.go:34-58`, `internal/tui/create.go:116-131`)
2. **Create — confusing dual control on Base field.** The Base field is both free-text (typing/backspace appends/removes to `baseBranch`) and a selector (↑/↓ changes `baseIndex` and overwrites `baseBranch` from the list). The two modes fight each other. (`internal/tui/create.go:124-137, 144-148`)
3. **Create — silent check-out when "create new branch" is chosen but branch exists.** In `AddWorktree`, `createBranch && branchExists` falls through to the `else` branch and runs `git worktree add <path> <branch>`, silently checking out the existing branch instead of creating a new one. (`internal/git/worktree.go:136-141`)
4. **Create — confusing git error when "create new branch" is unchecked but branch does not exist.** `createBranch == false && !branchExists` runs `git worktree add <path> <branch>`, which fails with a raw git "invalid reference" error. (`internal/git/worktree.go:139-141`)
5. **Create — `BranchExists` matches any ref, not only local branches.** `git rev-parse --verify <branch>` succeeds for tags and commit hashes too, so the create-new-vs-checkout-existing decision can be wrong. (`internal/git/branch.go:41-44`)
6. **Create — no friendly path-collision message.** If the auto-generated path already exists, the user sees a raw git error ("already exists") instead of a targeted message. (`internal/git/worktree.go:125-150`, `internal/tui/create.go:103-115`)
7. **Delete — dirty worktrees cannot be deleted.** `RemoveWorktree` runs `git worktree remove <path>` with no `--force`. Git refuses worktrees with untracked/modified files, so the delete modal warns "changes will be lost" and then fails. (`internal/git/worktree.go:152-161`, `internal/tui/delete.go:58`)
8. **Delete — stale `errMsg` lingers.** Entering delete mode does not clear `m.errMsg`. (`internal/tui/app.go:232-241`)
9. **Cleanup — detached-HEAD worktrees falsely flagged as stale.** `findStaleWorktrees` compares `wt.Branch` to `ListBranches()` (local branches only). Detached worktrees have `Branch == "(detached)"`, which is never in the set, so every detached worktree is offered for deletion. (`internal/tui/cleanup.go:129-152`, `internal/tui/app.go:91-99` porcelain parsing)
10. **Cleanup — dirty worktrees silently no-op.** The Enter handler skips dirty worktrees (lines 72-74) with no message, so selecting one and pressing Enter appears to do nothing. (`internal/tui/cleanup.go:51-127`)
11. **Cleanup — first error aborts the batch.** The removal loop returns on the first failure, leaving the list half-removed and not refreshed. (`internal/tui/cleanup.go:56-84`)
12. **List — `m.selected` can go out of bounds on an empty list.** After refresh, `m.selected >= len(...)` sets `m.selected = len-1` which is `-1` when empty; the subsequent `m.selected < 0` guard resets to 0, but list navigation keys assume `len > 0`. (`internal/tui/app.go:84-89, 214-271`)

## Design

### A. Dirty worktree deletion — force on explicit confirm

`RemoveWorktree(path string, force bool)` gains a `force` parameter. The TUI decides: a non-dirty worktree is removed normally; a dirty worktree is removed with `--force` **only** when the user confirms on a modal that explicitly says changes will be lost (option 2 per design approval).

The delete modal already shows the dirty warning. The confirmation key (`y`) in that modal is the safety gate. When the worktree is dirty, the modal shows `[y] force-remove [n]o`; `y` calls `RemoveWorktree(path, true)`. When clean, the modal shows `[y]es [n]o`; `y` calls `RemoveWorktree(path, false)`.

`--force` is never passed automatically without the modal having displayed the "changes will be lost" warning. The command at the git layer:

```
git worktree remove [--force] <path>
```

### B. Create form fixes

1. **Base selector sync.** In `NewModel`, after choosing `baseBranch`, set `createModel.baseIndex` to `indexOf(baseBranch, branches)` (0 when not found). This makes the displayed base and the ↑/↓ index agree.
2. **Base field becomes selector-only.** Remove free-text/backspace handling for `fieldBase` in `handleCreateKeyPress`; ↑/↓ move `baseIndex` and set `baseBranch = branches[baseIndex]`. Branch-name field remains free-text. Prep for (B1) makes this consistent.
3. **Explicit create-vs-checkout matrix in `AddWorktree`.** Replace the current `if createBranch && !branchExists` logic with four explicit cases, returning friendly errors *before* invoking git:
   - `createBranch && branchExists` → return error: `branch '<name>' already exists; uncheck 'create new branch' to check it out`.
   - `createBranch && !branchExists` → `git worktree add -b <branch> <path> <base>`.
   - `!createBranch && branchExists` → `git worktree add <path> <branch>`.
   - `!createBranch && !branchExists` → return error: `branch '<name>' does not exist; check 'create new branch' or select an existing branch`.
4. **Path-collision pre-check.** Before invoking `git worktree add`, if the auto-generated path already exists (and is non-empty), return error: `path '<path>' already exists`. (`os.Stat`/`os.ReadDir`.)
5. **`BranchExists` → local-branch only.** Use `git rev-parse --verify --quiet refs/heads/<branch>` and treat non-zero exit as "does not exist as a local branch". Returns `(bool, error)` with the error surfaced (no discarded `_`).

### C. Delete flow

- Pass `force` from (A). Clean worktrees normal; dirty worktrees `--force` only after explicit confirm in the modal.
- Clear `m.errMsg` when transitioning into delete mode (in `handleKeyPress` `case "d"`), so the modal shows a clean state.

### D. Cleanup flow

1. **Exclude detached worktrees from the stale set.** In `findStaleWorktrees`, `continue` when `wt.Detached` is true.
2. **Exclude dirty worktrees up front.** Move the dirty guard from the Enter handler into `findStaleWorktrees`: a worktree with `Status != nil && Status.IsDirty` is not stale-removable. Removed worktrees are those that are non-main, non-locked, non-dirty, and whose branch is not a local branch. The Enter handler no longer silently no-ops on dirty because such entries never appear.
3. **Collect errors, don't abort the batch.** The Enter handler iterates all selected, attempts removal for each, accumulates any errors into a single message, then refreshes the list (`m.loadWorktrees`) regardless of partial failure. The accumulated message becomes `m.errMsg` (e.g. `failed to remove <path1>; failed to remove <path2>`). On full success `m.errMsg` is empty.

### E. Safety / UX (small)

- Keep git-layer error messages prefixed with context (mostly present); ensure create paths return friendly strings per (B3, B4) rather than raw git output.
- Guard list navigation in `handleKeyPress` against an empty worktree list: `j`/`k`/`↑`/`↓` no-op when `len(m.worktrees) == 0`. The existing `worktreesLoadedMsg` bound clamp already resets `m.selected` to ≥ 0; add an explicit `len == 0` early-return in navigation to avoid reasoning about `-1` arithmetic.

## Behavior contracts

- `AddWorktree(path, branch, base string, createBranch bool) error` — returns a friendly, non-nil error for the two invalid combinations (create+exists, checkout+missing) and for path collisions **before** running git. Never silently changes semantics.
- `RemoveWorktree(path string, force bool) error` — runs `git worktree remove [--force] <path>`. `force` is decided by the caller based on dirty status + user confirmation.
- `BranchExists(branch string) (bool, error)` — `true` iff `<branch>` resolves as `refs/heads/<branch>` (a local branch); errors surfaced.
- `findStaleWorktrees()` — excludes main, locked, detached, and dirty worktrees; populates `staleWorktrees`/`selected` only from genuinely stale entries (branch not in local branch set).

## Testing

- `internal/git` unit tests:
  - `BranchExists`: local branch → true; tag of same name → false; missing → false.
  - `AddWorktree`: create+exists → friendly error, no git invocation (assert via not executing); checkout+missing → friendly error; path collision → friendly error (using a temp repo).
  - `RemoveWorktree(force)`: non-dirty normal; dry-run the `--force` arg construction (do not require a real dirty tree in unit tests; assert command args).
- `internal/tui` tests:
  - `NewModel` sets `baseIndex` to the index of `baseBranch` in `branches`.
  - Create keypress: Base field ↑/↓ changes `baseIndex` and `baseBranch` consistently; Base field ignores typed runes (no append).
  - Create keypress: pressing Enter with createBranch + existing branch shows the friendly error and does not change mode.
  - Delete keypress: dirty worktree → `y` calls `RemoveWorktree(path, true)` (verify via a fake git service interface—if refactor needed, use a small interface so the TUI is testable without shelling out).
  - Cleanup: `findStaleWorktrees` excludes detached and dirty; Enter handler removes all selected and refreshes even if one removal errors.
- Run the full existing test suite; ensure `go vet ./...` and `go build ./...` pass.

## Out of scope

- No new "modify/rename/move worktree" flow (clarified with user: their "change branch" concern was the create-form base selector, not a separate modify operation).
- No new validation of branch-name legality beyond existing behavior; only the matrix/collision checks listed here.
- No remote-tracking branch support in create or cleanup.
- No styling/layout changes.