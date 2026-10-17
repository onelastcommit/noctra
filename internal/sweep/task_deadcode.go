package sweep

import (
	"fmt"
	"time"
)

func init() {
	Register(Task{
		Name:         "dead-code",
		Description:  "Detect and remove unused code, imports, and variables",
		Cooldown:     14 * 24 * time.Hour,
		BranchSuffix: "dead-code",
		CommitPrefix: "refactor",
		PRLabel:      "maintenance",
		Prompt: func(repoPath string) string {
			return fmt.Sprintf(`You are performing autonomous maintenance on a codebase.

## Task: Dead code removal

Scan the codebase at %s for unused code, imports, variables, functions, types, and constants, then remove them.

## Instructions:
1. Read the project structure to understand the language(s) and build system in use.
2. Look for:
   - Unused imports
   - Unused local variables and constants
   - Unexported functions/types/methods that have no callers within the package
   - Commented-out code blocks (not documentation comments — only dead code)
3. Remove dead code carefully. If removal would break the public API (exported symbols), leave it.
4. Make all removals first, then run the build and the test suite ONCE at the end to confirm nothing broke.
5. Run the linter if available.

## Rules:
- Remove at most 20 symbols in a single run. If there is more dead code than that, remove the clearest 20 and note the rest in your summary.
- Run the full build/test suite at most 3 times. If it is still red on the third attempt, revert to the last state you verified green, keep only those removals, and report what you kept.
- Only remove genuinely dead/unused code. Do not refactor, rename, or restructure.
- Do not remove code that is used via reflection, build tags, or generated code patterns.
- Do not remove TODO/FIXME comments — only actual dead code.
- Preserve all tests, even if they test removed code (the tests should fail, telling you the removal was wrong).
- Follow existing project conventions and patterns exactly.
- If there is no dead code to remove, say BLOCKED: No dead code found — nothing to clean up.

## When done:
Provide a brief summary of what was removed. Wrap the summary between these exact marker lines, each alone on its own line with nothing else:

===NOCTRA SUMMARY===
<your summary here>
===END NOCTRA SUMMARY===

RELEASE: none`, repoPath)
		},
	})
}
