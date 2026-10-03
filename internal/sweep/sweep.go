package sweep

import (
	"strings"
	"time"
)

type Task struct {
	Name         string
	Description  string
	Cooldown     time.Duration
	Prompt       func(repoPath string) string
	BranchSuffix string
	CommitPrefix string
	PRLabel      string
}

var catalog []Task

func Register(t Task) {
	catalog = append(catalog, t)
}

func Catalog() []Task {
	out := make([]Task, len(catalog))
	copy(out, catalog)
	return out
}

func FilterTasks(enabled []string) []Task {
	if len(enabled) == 0 {
		return Catalog()
	}
	set := make(map[string]bool, len(enabled))
	for _, n := range enabled {
		set[n] = true
	}
	var out []Task
	for _, t := range catalog {
		if set[t.Name] {
			out = append(out, t)
		}
	}
	return out
}

const sweepBranchPrefix = "noctra/sweep-"

func SweepBranchName(taskSuffix string) string {
	return sweepBranchPrefix + strings.ToLower(taskSuffix)
}

func TaskSuffixFromBranch(branch string) (string, bool) {
	rest, ok := strings.CutPrefix(strings.ToLower(branch), sweepBranchPrefix)
	if !ok {
		return "", false
	}
	for _, t := range catalog {
		if t.BranchSuffix == rest {
			return rest, true
		}
	}
	return "", false
}

func SweepIdentifier(repoSlug, taskSuffix string) string {
	return strings.ToUpper("SWEEP-" + sanitizeRepoSlug(repoSlug) + "-" + taskSuffix)
}

func sanitizeRepoSlug(repoSlug string) string {
	return strings.ReplaceAll(repoSlug, "/", "-")
}

func ParseSweepIdentifier(identifier string) (repoSlug, taskSuffix string, ok bool) {
	rest, found := strings.CutPrefix(strings.ToLower(identifier), "sweep-")
	if !found {
		return "", "", false
	}
	best := ""
	for _, t := range catalog {
		if s := t.BranchSuffix; strings.HasSuffix(rest, "-"+s) && len(s) > len(best) {
			best = s
		}
	}
	if best == "" {
		return "", "", false
	}
	return strings.TrimSuffix(rest, "-"+best), best, true
}
