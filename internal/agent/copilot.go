package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type copilotBackend struct{}

func (copilotBackend) Name() string  { return "copilot" }
func (copilotBackend) Label() string { return "GitHub Copilot" }
func (copilotBackend) CLI() string   { return "copilot" }
func (copilotBackend) CoAuthor() string {
	return "Copilot <223556219+Copilot@users.noreply.github.com>"
}

func (b copilotBackend) Run(ctx context.Context, opts RunOptions) (Usage, error) {
	defer stageSkills(opts)()
	out, err := runCLI(ctx, b.CLI(), copilotArgs(opts), copilotEnv(ctx), opts, &copilotParser{})
	return ParseUsage(out), err
}

func copilotEnv(ctx context.Context) []string {
	if os.Getenv("COPILOT_GITHUB_TOKEN") != "" {
		return nil
	}
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	if token == "" {
		out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
		if err != nil {
			return nil
		}
		token = strings.TrimSpace(string(out))
	}
	if token == "" {
		return nil
	}
	if strings.HasPrefix(token, "ghp_") {
		slog.Warn("copilot: gh is authenticated with a classic PAT, which Copilot does not accept; " +
			"not injecting GH_TOKEN. Re-auth gh with the OAuth web flow (`gh auth login`), use a " +
			"fine-grained PAT, or run `copilot /login`.")
		return nil
	}
	return append(os.Environ(), "COPILOT_GITHUB_TOKEN="+token)
}

const copilotNoSuchTool = "noctra-no-tools"

func copilotArgs(opts RunOptions) []string {
	args := []string{"--allow-all-tools"}
	if opts.NoTools {
		args = []string{"--available-tools=" + copilotNoSuchTool, "--disable-builtin-mcps"}
	}
	args = append(args, "--no-ask-user", "--output-format", "json")
	args = append(args, modelArgs("--model", opts)...)
	return append(args, "-p", opts.Prompt)
}

type copilotEvent struct {
	Type     string `json:"type"`
	ExitCode *int   `json:"exitCode"`
	Data     struct {
		ToolName  string          `json:"toolName"`
		Arguments json.RawMessage `json:"arguments"`
		Content   string          `json:"content"`
		Phase     string          `json:"phase"`
	} `json:"data"`
}

type copilotParser struct {
	final, last string
	diag        streamTail
}

func (p *copilotParser) feed(line []byte) string {
	var ev copilotEvent
	if json.Unmarshal(line, &ev) != nil {
		p.diag.add(line)
		return ""
	}
	switch {
	case ev.Type == "tool.execution_start":
		return toolCallLine(ev.Data.ToolName, ev.Data.Arguments)
	case ev.Type == "assistant.message" && ev.Data.Content != "":
		p.last = ev.Data.Content
		if ev.Data.Phase == "final_answer" {
			p.final = ev.Data.Content
		}
	case ev.Type == "result" && ev.ExitCode != nil && *ev.ExitCode != 0,
		strings.Contains(ev.Type, "error"), strings.Contains(ev.Type, "warn"):
		p.diag.add(line)
	}
	return ""
}

func (p *copilotParser) response() string {
	if p.final != "" {
		return p.final
	}
	return p.last
}

func (p *copilotParser) diagnostics() string { return p.diag.String() }

var copilotRateLimitRe = regexp.MustCompile(`(?i)rate.?limit|usage.?limit|quota|exceeded.*limit|too many requests`)

func (copilotBackend) HasRateLimit(output string) bool {
	return copilotRateLimitRe.MatchString(withoutToolLines(output))
}
