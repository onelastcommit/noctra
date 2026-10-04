package agent

import (
	"context"
	"regexp"
)

type antigravityBackend struct{}

func (antigravityBackend) Name() string     { return "antigravity" }
func (antigravityBackend) Label() string    { return "Google Antigravity" }
func (antigravityBackend) CLI() string      { return "agy" }
func (antigravityBackend) CoAuthor() string { return "Antigravity <noreply@google.com>" }

func (b antigravityBackend) Run(ctx context.Context, opts RunOptions) (Usage, error) {
	defer stageSkills(opts)()
	out, err := runCLI(ctx, b.CLI(), antigravityArgs(opts), nil, opts)
	return ParseUsage(out), err
}

func antigravityArgs(opts RunOptions) []string {
	return []string{
		"--dangerously-skip-permissions",
		"--print", opts.Prompt,
	}
}

var antigravityRateLimitRe = regexp.MustCompile(`(?i)rate.?limit|usage.?limit|quota|resource.?exhausted|exceeded.*limit|too many requests`)

func (antigravityBackend) HasRateLimit(output string) bool {
	return antigravityRateLimitRe.MatchString(output)
}
