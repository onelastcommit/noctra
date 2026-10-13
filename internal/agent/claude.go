package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
)

type claudeBackend struct{}

func (claudeBackend) Name() string     { return "claude" }
func (claudeBackend) Label() string    { return "Claude Code" }
func (claudeBackend) CLI() string      { return "claude" }
func (claudeBackend) CoAuthor() string { return "Claude <noreply@anthropic.com>" }

func (b claudeBackend) Run(ctx context.Context, opts RunOptions) (Usage, error) {
	var env []string
	if opts.UseAgentTeams {
		env = append(os.Environ(), "CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1")
	}

	return b.runStream(ctx, opts, env)
}

func claudeStreamArgs(opts RunOptions) []string {
	args := []string{
		"--dangerously-skip-permissions",
		"--print",
		"--output-format", "stream-json",
		"--verbose",
	}
	args = append(args, claudePluginArgs(opts)...)
	return append(args, "-p", opts.Prompt)
}

func claudePluginArgs(opts RunOptions) []string {
	args := make([]string, 0, 2*len(opts.PluginDirs))
	for _, dir := range opts.PluginDirs {
		args = append(args, "--plugin-dir", dir)
	}
	return args
}

type claudeStreamEvent struct {
	Type         string                `json:"type"`
	Subtype      string                `json:"subtype"`
	Result       string                `json:"result"`
	TotalCostUSD float64               `json:"total_cost_usd"`
	Model        string                `json:"model"`
	ModelUsage   map[string]modelShare `json:"modelUsage"`
	Usage        claudeUsage           `json:"usage"`
	Outcome      string                `json:"outcome"`
	RateLimit    struct {
		Status string `json:"status"`
	} `json:"rate_limit_info"`
	Message struct {
		Model   string      `json:"model"`
		Usage   claudeUsage `json:"usage"`
		Content []struct {
			Type  string          `json:"type"`
			Name  string          `json:"name"`
			Input json.RawMessage `json:"input"`
		} `json:"content"`
	} `json:"message"`
}

type claudeUsage struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

func (u claudeUsage) total() int64 {
	return u.InputTokens + u.OutputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
}

func (u *claudeUsage) add(other claudeUsage) {
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.CacheCreationInputTokens += other.CacheCreationInputTokens
	u.CacheReadInputTokens += other.CacheReadInputTokens
}

func (u claudeUsage) toUsage(model string) Usage {
	billedInput := u.InputTokens + u.CacheCreationInputTokens + u.CacheReadInputTokens
	return Usage{
		InputTokens:  billedInput,
		OutputTokens: u.OutputTokens,
		TotalTokens:  u.total(),
		CostUSD: PricesForModel(model).Estimate(
			u.InputTokens, u.OutputTokens, u.CacheCreationInputTokens, u.CacheReadInputTokens),
	}
}

type streamTail struct {
	lines [][]byte
	size  int
}

const streamTailMax = 16 << 10

func (t *streamTail) add(line []byte) {
	l := slices.Clone(line)
	t.lines = append(t.lines, l)
	t.size += len(l)
	for t.size > streamTailMax && len(t.lines) > 1 {
		t.size -= len(t.lines[0])
		t.lines = t.lines[1:]
	}
}

func (t *streamTail) String() string {
	return string(bytes.Join(t.lines, []byte("\n")))
}

var claudeProgressSubtypes = map[string]bool{
	"init":                     true,
	"thinking_tokens":          true,
	"task_started":             true,
	"task_progress":            true,
	"task_updated":             true,
	"task_notification":        true,
	"background_tasks_changed": true,
	"hook_started":             true,
	"hook_progress":            true,
}

func tailWorthy(ev claudeStreamEvent) bool {
	switch ev.Type {
	case "assistant", "user", "tool_progress":
		return false
	case "rate_limit_event":
		return ev.RateLimit.Status != "allowed"
	case "system":
		if ev.Subtype == "hook_response" {
			return ev.Outcome != "success"
		}
		return !claudeProgressSubtypes[ev.Subtype]
	case "result":
		return ev.Result == ""
	}
	return true
}

func (b claudeBackend) runStream(ctx context.Context, opts RunOptions, env []string) (Usage, error) {
	runCtx, cancel := context.WithCancel(ctx)
	if opts.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
	}
	defer cancel()

	cmd := exec.CommandContext(runCtx, b.CLI(), claudeStreamArgs(opts)...)
	cmd.Dir = opts.Workdir
	if env := childEnv(env, opts); env != nil {
		cmd.Env = env
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return Usage{}, fmt.Errorf("stdout pipe: %w", err)
	}
	logF, err := openRunLog(ctx, opts)
	if err != nil {
		return Usage{}, err
	}
	defer logF.Close()
	if err := cmd.Start(); err != nil {
		return Usage{}, fmt.Errorf("start claude: %w", err)
	}

	var (
		final     Usage
		result    string
		cumUsage  claudeUsage
		cumTokens int64
		model     string
		seen      = map[string]modelShare{}
		aborted   bool
		tail      streamTail
	)
	reader := bufio.NewReader(pipe)
	for {
		line, rerr := reader.ReadBytes('\n')
		if trimmed := bytes.TrimSpace(line); len(trimmed) > 0 {
			var ev claudeStreamEvent
			if json.Unmarshal(trimmed, &ev) != nil {
				tail.add(trimmed)
			} else {
				switch ev.Type {
				case "assistant":
					for _, c := range ev.Message.Content {
						if c.Type == "tool_use" {
							fmt.Fprintln(logF, toolCallLine(c.Name, c.Input))
						}
					}
					if ev.Message.Model != "" {
						model = ev.Message.Model
						share := seen[model]
						share.OutputTokens += ev.Message.Usage.OutputTokens
						seen[model] = share
					}
					cumUsage.add(ev.Message.Usage)
					cumTokens = cumUsage.total()
					if !aborted && opts.MaxTokens > 0 && cumTokens >= opts.MaxTokens {
						aborted = true
						cancel()
					}
				case "result":
					in := ev.Usage.InputTokens + ev.Usage.CacheCreationInputTokens + ev.Usage.CacheReadInputTokens
					final = Usage{
						InputTokens:  in,
						OutputTokens: ev.Usage.OutputTokens,
						TotalTokens:  in + ev.Usage.OutputTokens,
						CostUSD:      ev.TotalCostUSD,
						Model:        primaryModel(ev.ModelUsage),
					}
					result = ev.Result
				}
				if tailWorthy(ev) {
					tail.add(trimmed)
				}
			}
		}
		if rerr != nil {
			break
		}
	}
	waitErr := cmd.Wait()

	var diagnostics string
	if tail.size > 0 {
		diagnostics += "[noctra] non-result stream events:\n" + tail.String() + "\n\n"
	}
	if aborted {
		diagnostics += fmt.Sprintf("[noctra] run aborted: per-run token ceiling %d reached (cumulative ~%d)\n\n", opts.MaxTokens, cumTokens)
	}
	writeRunTrailer(logF, diagnostics+stderr.String(), result)

	if final.TotalTokens == 0 && cumTokens > 0 {
		final = cumUsage.toUsage(model)
	} else if final.CostUSD == 0 && cumTokens > 0 {
		final.CostUSD = cumUsage.toUsage(model).CostUSD
	}
	if final.Model == "" {
		final.Model = primaryModel(seen)
	}

	if aborted {
		return final, fmt.Errorf("%w: cumulative ~%d tokens (ceiling %d)", ErrTokenCapExceeded, cumTokens, opts.MaxTokens)
	}
	if waitErr != nil {
		if runCtx.Err() == context.DeadlineExceeded {
			return final, fmt.Errorf("%w: %w", ErrTimedOut, waitErr)
		}
		return final, waitErr
	}
	return final, nil
}

var claudeRateLimitRe = regexp.MustCompile(`(?i)rate.limit|usage.limit|exceeded.*limit|too many requests`)

func (claudeBackend) HasRateLimit(output string) bool {
	return claudeRateLimitRe.MatchString(withoutToolLines(output))
}
