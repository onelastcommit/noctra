package agent

import (
	"context"
	"regexp"
)

type codexBackend struct{}

func (codexBackend) Name() string     { return "codex" }
func (codexBackend) Label() string    { return "OpenAI Codex" }
func (codexBackend) CLI() string      { return "codex" }
func (codexBackend) CoAuthor() string { return "Codex <noreply@openai.com>" }

func (b codexBackend) Run(ctx context.Context, opts RunOptions) (Usage, error) {
	defer stageSkills(opts)()
	out, err := runCLI(ctx, b.CLI(), codexArgs(opts), nil, opts, nil)
	usage := ParseUsage(out)
	usage.Model = codexModel(out)
	return usage, err
}

var codexModelRe = regexp.MustCompile(`(?m)^model:[ \t]*(\S+)[ \t]*$`)

func codexModel(output string) string {
	if m := codexModelRe.FindStringSubmatch(output); m != nil {
		return m[1]
	}
	return ""
}

func codexArgs(opts RunOptions) []string {
	args := []string{"exec", "--dangerously-bypass-approvals-and-sandbox"}
	if opts.NoTools {
		args = []string{"exec", "--sandbox", "read-only", "--skip-git-repo-check"}
	}
	args = append(args, modelArgs("--model", opts)...)
	return append(args, opts.Prompt)
}

var codexRateLimitRe = regexp.MustCompile(`(?i)rate.?limit|usage.?limit|quota|exceeded.*limit|too many requests`)

func (codexBackend) HasRateLimit(output string) bool {
	return codexRateLimitRe.MatchString(withoutToolLines(output))
}
