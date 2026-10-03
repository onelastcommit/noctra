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
	out, err := runCLI(ctx, b.CLI(), codexArgs(opts), nil, opts)
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
	return []string{
		"exec",
		"--dangerously-bypass-approvals-and-sandbox",
		opts.Prompt,
	}
}

var codexRateLimitRe = regexp.MustCompile(`(?i)rate.?limit|usage.?limit|quota|exceeded.*limit|too many requests`)

func (codexBackend) HasRateLimit(output string) bool {
	return codexRateLimitRe.MatchString(output)
}
