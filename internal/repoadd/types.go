package repoadd

import (
	"context"

	"github.com/onelastcommit/noctra/internal/linear"
	"github.com/onelastcommit/noctra/internal/repo"
)

type Cloner interface {
	ResolveDirect(ctx context.Context, ref, branch string) (repo.Resolved, error)
}

type Projects interface {
	ListProjects(ctx context.Context) ([]linear.Project, error)
	UpdateProjectContent(ctx context.Context, projectID, content string) error
}

type Request struct {
	Ref     string
	Branch  string
	Project *linear.Project
}

type Result struct {
	OwnerRepo string
	Path      string
	Branch    string
	Project   string
}
