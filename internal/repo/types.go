package repo

type NonTransientError struct {
	Err error
}

func (e *NonTransientError) Error() string { return e.Err.Error() }

func (e *NonTransientError) Unwrap() error { return e.Err }

type Resolved struct {
	Path       string
	MainBranch string
}

type Worktree struct {
	Path   string
	Branch string
}
