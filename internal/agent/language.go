package agent

import "fmt"

type englishSpelling struct {
	name     string
	examples string
}

var englishVariants = map[string]englishSpelling{
	"british":  {"British English", `"colour", "behaviour", "organise", "licence" (noun), "analyse", "centre"`},
	"american": {"American English", `"color", "behavior", "organize", "license", "analyze", "center"`},
}

func LanguageSection(variant string) string {
	spelling, ok := englishVariants[variant]
	if !ok {
		spelling = englishVariants["british"]
	}
	return fmt.Sprintf(`

## Language

Write all prose in %s: code comments, docs, commit messages, PR titles and descriptions, review replies and user-facing copy (e.g. %s). Do not respell anything fixed by a language, specification or API, such as keywords, CSS properties, HTML attributes, library identifiers and existing names in the codebase. When naming something new, follow the spelling the codebase already uses.
`, spelling.name, spelling.examples)
}
