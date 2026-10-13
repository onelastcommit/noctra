package agent

import (
	"context"
	"encoding/json"
	"regexp"
)

type antigravityBackend struct{}

func (antigravityBackend) Name() string     { return "antigravity" }
func (antigravityBackend) Label() string    { return "Google Antigravity" }
func (antigravityBackend) CLI() string      { return "agy" }
func (antigravityBackend) CoAuthor() string { return "Antigravity <noreply@google.com>" }

func (b antigravityBackend) Run(ctx context.Context, opts RunOptions) (Usage, error) {
	defer stageSkills(opts)()
	out, err := runCLI(ctx, b.CLI(), antigravityArgs(opts), nil, opts, &antigravityParser{})
	return ParseUsage(out), err
}

func antigravityArgs(opts RunOptions) []string {
	var args []string
	if !opts.NoTools {
		args = append(args, "--dangerously-skip-permissions")
	}
	args = append(args, modelArgs("--model", opts)...)
	args = append(args, "--output-format", "stream-json")
	return append(args, "--print", opts.Prompt)
}

type antigravityEvent struct {
	Event      string `json:"event"`
	StepUpdate struct {
		StepIndex int    `json:"step_index"`
		State     string `json:"state"`
		StepType  string `json:"step_type"`
		ToolName  string `json:"tool_name"`
		TextDelta string `json:"text_delta"`
		ToolInfo  struct {
			Parameters json.RawMessage `json:"parameters"`
		} `json:"tool_info"`
	} `json:"step_update"`
	Result *struct {
		Status   string `json:"status"`
		Response string `json:"response"`
	} `json:"result"`
}

type antigravityParser struct {
	toolsSeen  map[int]bool
	text       map[int]string
	lastText   int
	result     string
	haveResult bool
	diag       streamTail
}

func (p *antigravityParser) feed(line []byte) string {
	var ev antigravityEvent
	if json.Unmarshal(line, &ev) != nil {
		p.diag.add(line)
		return ""
	}
	switch ev.Event {
	case "init":
	case "step_update":
		return p.step(ev, line)
	case "result":
		if ev.Result != nil {
			p.result, p.haveResult = ev.Result.Response, true
			if ev.Result.Status != "SUCCESS" {
				p.diag.add(line)
			}
		}
	default:
		p.diag.add(line)
	}
	return ""
}

func (p *antigravityParser) step(ev antigravityEvent, line []byte) string {
	s := ev.StepUpdate
	if s.State != "ACTIVE" && s.State != "DONE" {
		p.diag.add(line)
	}
	switch s.StepType {
	case "tool":
		if p.toolsSeen == nil {
			p.toolsSeen = map[int]bool{}
		}
		if !p.toolsSeen[s.StepIndex] {
			p.toolsSeen[s.StepIndex] = true
			return toolCallLine(s.ToolName, s.ToolInfo.Parameters)
		}
	case "agent_response":
		if p.text == nil {
			p.text = map[int]string{}
		}
		p.text[s.StepIndex] += s.TextDelta
		p.lastText = s.StepIndex
	}
	return ""
}

func (p *antigravityParser) response() string {
	if p.haveResult {
		return p.result
	}
	return p.text[p.lastText]
}

func (p *antigravityParser) diagnostics() string { return p.diag.String() }

var antigravityRateLimitRe = regexp.MustCompile(`(?i)rate.?limit|usage.?limit|quota|resource.?exhausted|exceeded.*limit|too many requests`)

func (antigravityBackend) HasRateLimit(output string) bool {
	return antigravityRateLimitRe.MatchString(withoutToolLines(output))
}
