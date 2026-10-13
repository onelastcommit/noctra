package authcheck

import "context"

type Result struct {
	Name    string
	OK      bool
	Skipped bool
	Detail  string
	Fix     string
}

type Check struct {
	Name string
	Run  func(ctx context.Context) Result
}

type Exec func(ctx context.Context, name string, args ...string) (string, error)

type TokenMinter func(ctx context.Context, ownerRepo string) error

type Report struct {
	Failing   []Result
	Reminders []Result
	Recovered []Result
}

func (r Report) Empty() bool {
	return len(r.Failing) == 0 && len(r.Reminders) == 0 && len(r.Recovered) == 0
}
