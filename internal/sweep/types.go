package sweep

import (
	"context"
	"time"

	"github.com/onelastcommit/noctra/internal/repo"
)

type RepoResolver interface {
	AllRepoPaths() []string
	ResolveDirect(ctx context.Context, ref, branch string) (repo.Resolved, error)
}

type Job struct {
	Task       Task
	RepoPath   string
	RepoSlug   string
	MainBranch string
}

type repoTarget struct {
	path   string
	branch string
}

type PlanOptions struct {
	Tasks          []string
	Repos          []string
	IgnoreCooldown bool
	Limit          int
}

type Task struct {
	Name         string
	Description  string
	Cooldown     time.Duration
	Prompt       func(repoPath string) string
	BranchSuffix string
	CommitPrefix string
	PRLabel      string
}
