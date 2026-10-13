package authcheck

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/onelastcommit/noctra/internal/github"
)

const repoProbeConcurrency = 4

const repoFix = "for a moved or deleted repository, point its Linear project's Repo: line at the new name and delete the old clone shown by `noctra repos list`; for a new owner, install noctra-agent there"

func Repositories(lookup RepoLookup, repos func(ctx context.Context) []string) Check {
	return Check{Name: "Repositories", Run: func(ctx context.Context) Result {
		list := repos(ctx)
		problems := probeRepos(ctx, lookup, list)
		if ctx.Err() != nil {
			return Result{Skipped: true, Detail: "repository probe did not finish: " + ctx.Err().Error()}
		}
		if len(problems) == 0 {
			return Result{OK: true}
		}
		return Result{
			Detail: fmt.Sprintf("%d of %d cloned repositories need attention: %s", len(problems), len(list), strings.Join(problems, "; ")),
			Fix:    repoFix,
		}
	}}
}

func probeRepos(ctx context.Context, lookup RepoLookup, list []string) []string {
	found := make([]string, len(list))
	sem := make(chan struct{}, repoProbeConcurrency)
	var wg sync.WaitGroup
	for i, ownerRepo := range list {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			fullName, err := lookup(ctx, ownerRepo)
			found[i] = repoProblem(ownerRepo, fullName, err)
		}()
	}
	wg.Wait()
	var problems []string
	for _, p := range found {
		if p != "" {
			problems = append(problems, p)
		}
	}
	return problems
}

func repoProblem(ownerRepo, fullName string, err error) string {
	switch {
	case err == nil && strings.EqualFold(fullName, ownerRepo):
		return ""
	case err == nil:
		return ownerRepo + " moved to " + fullName
	case errors.Is(err, github.ErrAppNotInstalled):
		return ownerRepo + ": noctra-agent is not installed on it (moved, deleted, or never installed)"
	case errors.Is(err, github.ErrRepoNotFound):
		return ownerRepo + ": not found (deleted, or no access)"
	default:
		return ownerRepo + ": " + firstLine(err.Error(), nil)
	}
}
