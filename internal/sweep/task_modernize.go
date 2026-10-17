package sweep

import (
	"fmt"
	"time"
)

func init() {
	Register(Task{
		Name:         "modernize",
		Description:  "Replace deprecated APIs and adopt newer language idioms",
		Cooldown:     14 * 24 * time.Hour,
		BranchSuffix: "modernize",
		CommitPrefix: "refactor",
		PRLabel:      "maintenance",
		Prompt: func(repoPath string) string {
			return fmt.Sprintf(`You are performing autonomous maintenance on a codebase.

## Task: Deprecations and newer idioms

Replace deprecated API usage and adopt newer language idioms in the project at %s.

## Instructions:
1. Find usages of deprecated APIs (e.g. staticcheck SA1019, compiler/tooling deprecation notices, gopls "modernize" suggestions, language-version idioms the project can now use).
2. Replace them with the recommended current equivalents.
3. Apply mechanical rewrites only where they are unambiguously equivalent (e.g. loops a stdlib helper now covers).
4. Run the build and full test suite to confirm behaviour is unchanged.

## Rules:
- Behaviour-preserving changes only. Do NOT change features, APIs, or public behaviour.
- Targeted replacements, not broad refactors. If a rewrite is ambiguous or risky, skip it.
- Do not bump the language/runtime version unless the project already requires it.
- If there are no deprecated APIs or safe rewrites, say BLOCKED: No deprecated usage or safe rewrites found.

## When done:
Describe what you replaced or rewrote. Wrap the summary between these exact marker lines, each alone on its own line with nothing else:

===NOCTRA SUMMARY===
<your summary here>
===END NOCTRA SUMMARY===

RELEASE: none`, repoPath)
		},
	})
}
