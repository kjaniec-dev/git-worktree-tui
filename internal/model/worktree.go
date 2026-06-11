package model

type Worktree struct {
	Path     string
	Branch   string
	Commit   string
	IsMain   bool
	IsLocked bool
	IsBare   bool
	Detached bool
	Status   *WorktreeStatus
}

type WorktreeStatus struct {
	IsDirty   bool
	Ahead     int
	Behind    int
	HasStash  bool
	Untracked int
}