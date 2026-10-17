package sweep

import (
	"fmt"
	"time"
)

func init() {
	Register(Task{
		Name:         "deps-update",
		Description:  "Update outdated dependencies and verify the build",
		Cooldown:     7 * 24 * time.Hour,
		BranchSuffix: "deps-update",
		CommitPrefix: "chore",
		PRLabel:      "maintenance",
		Prompt: func(repoPath string) string {
			return fmt.Sprintf(`You are performing autonomous maintenance on a codebase.

## Task: Dependency updates

Update outdated dependencies in the project at %s, conservatively.

## Instructions:
1. Detect the package manager(s) in use (e.g. go.mod, package.json, pyproject.toml, Cargo.toml).
2. List outdated dependencies (e.g. go list -u -m all, npm outdated, pip list --outdated).
3. Bump dependencies that have a newer compatible release. Prefer patch and minor bumps. Bump at most 5 dependencies in a single run — pick the most valuable ones and leave the rest for a future run.
4. Apply all chosen bumps first, then run the build and the full test suite ONCE for the whole batch.
5. If the batch fails, bisect at most twice to isolate the culprit. If it is still failing after that, revert to the last combination you verified green (or drop all bumps) and report what you could not land.

## Rules:
- Run the full build/test suite at most 4 times in this run. Shipping 2 verified bumps beats spending the whole run chasing 10.
- Do NOT bump across a major version (e.g. v1 -> v2) unless the change is trivial and verified green.
- Update lockfiles where applicable (go.sum, package-lock.json) via the proper tooling, not by hand.
- One coherent PR of dependency bumps; do not mix in unrelated refactors.
- If everything is already up to date (or no remaining bump passes tests), say BLOCKED: No dependency updates available.
- If you cannot determine the package manager or run the build, say BLOCKED: Cannot determine dependency tooling.

## When done:
Provide a brief summary of what was bumped (name: old -> new). Wrap the summary between these exact marker lines, each alone on its own line with nothing else:

===NOCTRA SUMMARY===
<your summary here>
===END NOCTRA SUMMARY===

RELEASE: none`, repoPath)
		},
	})
}
