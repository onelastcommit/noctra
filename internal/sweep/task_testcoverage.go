package sweep

import (
	"fmt"
	"time"
)

func init() {
	Register(Task{
		Name:         "test-coverage",
		Description:  "Add tests for the lowest-coverage, highest-risk code",
		Cooldown:     14 * 24 * time.Hour,
		BranchSuffix: "test-coverage",
		CommitPrefix: "test",
		PRLabel:      "maintenance",
		Prompt: func(repoPath string) string {
			return fmt.Sprintf(`You are performing autonomous maintenance on a codebase.

## Task: Test coverage

Improve test coverage for the project at %s by adding meaningful tests where they are missing.

## Instructions:
1. Measure current coverage (e.g. go test ./... -cover, or the project's coverage tool).
2. Identify the packages/files with the lowest coverage AND non-trivial logic (skip generated code, trivial getters, main wiring).
3. Pick ONE such area and add tests that exercise real behaviour and edge cases — not assertions that merely re-state the implementation.
4. Run the new tests and confirm they pass and actually raise coverage. Run only the package under test, not the whole suite, while iterating.

## Rules:
- Run the full test suite at most twice in this run; use targeted per-package runs while iterating.
- If a test you add will not go green after 3 attempts, delete that test, keep whatever is already passing, and report what you kept.
- Add tests only. Do NOT change production code except where a test reveals a genuine bug (and if so, keep the fix minimal and call it out).
- Follow the project's existing test style, helpers, and table-driven patterns.
- Prefer one well-covered area over many shallow tests sprinkled around.
- If coverage is already high everywhere or there is no meaningful untested logic, say BLOCKED: No meaningful coverage gaps found.
- If you cannot run the test suite, say BLOCKED: Cannot run the test suite.

## When done:
Describe which area you covered and the coverage delta. Wrap the summary between these exact marker lines, each alone on its own line with nothing else:

===NOCTRA SUMMARY===
<your summary here>
===END NOCTRA SUMMARY===

RELEASE: none`, repoPath)
		},
	})
}
