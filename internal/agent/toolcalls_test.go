package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestToolCallLine_PicksTheMostTellingField(t *testing.T) {
	cases := []struct {
		name, tool, input, want string
	}{
		{"claude bash", "Bash", `{"command":"pnpm install","description":"Install deps"}`, "[tool] Bash: pnpm install"},
		{"claude read", "Read", `{"file_path":"/w/x.txt"}`, "[tool] Read: /w/x.txt"},
		{"claude write skips content", "Write", `{"content":"secret body","file_path":"/w/a.go"}`, "[tool] Write: /w/a.go"},
		{"antigravity camel case", "run_command", `{"CommandLine":"ls -la"}`, "[tool] run_command: ls -la"},
		{"string input", "shell", `"echo hi"`, "[tool] shell: echo hi"},
		{"no known field", "TodoWrite", `{"todos":[]}`, "[tool] TodoWrite"},
		{"no input", "Think", ``, "[tool] Think"},
		{"multiline collapsed", "Bash", `{"command":"cd x &&\n  make test"}`, "[tool] Bash: cd x && make test"},
	}
	for _, tc := range cases {
		if got := toolCallLine(tc.tool, json.RawMessage(tc.input)); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestToolCallLine_TruncatesLongCommands(t *testing.T) {
	input, _ := json.Marshal(map[string]string{"command": strings.Repeat("a", 500)})
	got := toolCallLine("Bash", input)
	if r := []rune(strings.TrimPrefix(got, "[tool] Bash: ")); len(r) != toolDetailMax+1 || r[len(r)-1] != '…' {
		t.Errorf("want %d runes plus an ellipsis, got %d: %q", toolDetailMax, len(r), got)
	}
}

func TestFailureScanners_IgnoreToolLines(t *testing.T) {
	output := "[tool] Bash: grep -rn \"rate limit\" src\n[tool] Grep: not logged in\nError: exit status 1\n"
	for _, b := range []Backend{claudeBackend{}, codexBackend{}, copilotBackend{}, antigravityBackend{}} {
		if b.HasRateLimit(output) {
			t.Errorf("%s: a logged command mentioning rate limits must not count as a rate limit", b.Name())
		}
	}
	if line := AuthFailureLine(output); line != "" {
		t.Errorf("a logged command must not count as an auth failure, got %q", line)
	}
	if !(claudeBackend{}).HasRateLimit(output + "API Error: rate limit exceeded\n") {
		t.Error("a real rate-limit line must still be detected")
	}
}

func TestClaudeRun_LogsToolCallsAboveTheResponse(t *testing.T) {
	opts := fakeClaude(t, `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"model":"claude-opus-5-5","content":[{"type":"text","text":"Checking."},{"type":"tool_use","name":"Bash","input":{"command":"go test ./...","description":"Run tests"}}],"usage":{"output_tokens":5}}}
{"type":"tool_progress","elapsed_time_seconds":30,"heartbeat":true}
{"type":"assistant","message":{"model":"claude-opus-5-5","content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/w/main.go","old_string":"a","new_string":"b"}}],"usage":{"output_tokens":5}}}
{"type":"result","result":"Implemented the feature.","total_cost_usd":0.1,"usage":{"input_tokens":5,"output_tokens":10}}
`)
	if _, err := (claudeBackend{}).Run(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(opts.LogFile)
	log := string(raw)
	for _, want := range []string{"[tool] Bash: go test ./...", "[tool] Edit: /w/main.go"} {
		if !strings.Contains(log, want) {
			t.Errorf("log missing %q:\n%s", want, log)
		}
	}
	if strings.Contains(log, "tool_progress") {
		t.Errorf("progress heartbeats must not be logged:\n%s", log)
	}
	if strings.Index(log, "[tool] Edit") > strings.Index(log, ResponseHeader) {
		t.Errorf("tool lines must come before the response block:\n%s", log)
	}
	if got := ExtractSummary(log); got != "Implemented the feature." {
		t.Errorf("summary = %q", got)
	}
}

func feedAll(p streamParser, sample string) []string {
	var tools []string
	for _, l := range strings.Split(strings.TrimSpace(sample), "\n") {
		if tl := p.feed([]byte(l)); tl != "" {
			tools = append(tools, tl)
		}
	}
	return tools
}

func TestCopilotParser_ToolCallsAndFinalAnswer(t *testing.T) {
	sample := `{"type":"session.mcp_servers_loaded","data":{"servers":[]}}
{"type":"assistant.tool_call_delta","data":{"toolName":"bash","inputDelta":"{\""}}
{"type":"assistant.message","data":{"content":"","toolRequests":[{"name":"bash"}]}}
{"type":"tool.execution_start","data":{"toolName":"bash","arguments":{"command":"ls -la && sed -n '1,200p' x.txt","description":"List files and read x.txt","mode":"sync"}}}
{"type":"session.background_tasks_changed","data":{}}
{"type":"assistant.message","data":{"content":"I'll wrap up.","phase":"commentary"}}
{"type":"assistant.message","data":{"content":"hello","toolRequests":[],"phase":"final_answer"}}
{"type":"session.error","data":{"message":"quota exceeded"}}
{"type":"result","exitCode":0,"usage":{"premiumRequests":1}}`
	p := &copilotParser{}
	tools := feedAll(p, sample)
	if len(tools) != 1 || tools[0] != "[tool] bash: ls -la && sed -n '1,200p' x.txt" {
		t.Errorf("tools = %q", tools)
	}
	if p.response() != "hello" {
		t.Errorf("response = %q, want the final_answer message", p.response())
	}
	if d := p.diagnostics(); !strings.Contains(d, "quota exceeded") || strings.Contains(d, "background_tasks") {
		t.Errorf("diagnostics should keep only error events, got %q", d)
	}
}

func TestCopilotParser_FallsBackToLastMessage(t *testing.T) {
	p := &copilotParser{}
	feedAll(p, `{"type":"assistant.message","data":{"content":"first"}}
{"type":"assistant.message","data":{"content":"second"}}
{"type":"result","exitCode":1}`)
	if p.response() != "second" {
		t.Errorf("response = %q", p.response())
	}
	if !strings.Contains(p.diagnostics(), `"exitCode":1`) {
		t.Errorf("a failing result must be kept, got %q", p.diagnostics())
	}
}

func TestAntigravityParser_ToolCallsAndResult(t *testing.T) {
	sample := `{"event":"init","conversation_id":"c","init":{"tools":["run_command"]}}
{"event":"step_update","step_update":{"step_index":0,"state":"DONE","step_type":"user_input"}}
{"event":"step_update","step_update":{"step_index":2,"state":"ACTIVE","step_type":"tool","tool_name":"run_command","tool_info":{"name":"run_command","parameters":{"CommandLine":"ls -la"}}}}
{"event":"step_update","step_update":{"step_index":2,"state":"DONE","step_type":"tool","tool_name":"run_command","tool_info":{"name":"run_command","parameters":{"CommandLine":"ls -la"},"output":"total 8"}}}
{"event":"step_update","step_update":{"step_index":3,"state":"ACTIVE","step_type":"agent_response","text_delta":"hel"}}
{"event":"step_update","step_update":{"step_index":3,"state":"DONE","step_type":"agent_response","text_delta":"lo\n"}}
{"event":"result","result":{"status":"SUCCESS","response":"hello\n"}}`
	p := &antigravityParser{}
	tools := feedAll(p, sample)
	if len(tools) != 1 || tools[0] != "[tool] run_command: ls -la" {
		t.Errorf("each tool step must be logged once, got %q", tools)
	}
	if p.response() != "hello\n" {
		t.Errorf("response = %q", p.response())
	}
	if p.diagnostics() != "" {
		t.Errorf("a clean run has no diagnostics, got %q", p.diagnostics())
	}
}

func TestAntigravityParser_NoResultUsesStreamedTextAndKeepsFailures(t *testing.T) {
	p := &antigravityParser{}
	feedAll(p, `{"event":"step_update","step_update":{"step_index":1,"state":"ACTIVE","step_type":"agent_response","text_delta":"partial "}}
{"event":"step_update","step_update":{"step_index":1,"state":"ACTIVE","step_type":"agent_response","text_delta":"answer"}}
{"event":"step_update","step_update":{"step_index":2,"state":"ERROR","step_type":"tool","tool_name":"run_command"}}
{"event":"error","message":"RESOURCE_EXHAUSTED"}
not json at all`)
	if p.response() != "partial answer" {
		t.Errorf("response = %q", p.response())
	}
	d := p.diagnostics()
	for _, want := range []string{`"state":"ERROR"`, "RESOURCE_EXHAUSTED", "not json at all"} {
		if !strings.Contains(d, want) {
			t.Errorf("diagnostics missing %q: %q", want, d)
		}
	}
}

func TestRunCLI_WithParserLogsToolLinesAndParsedResponse(t *testing.T) {
	dir := t.TempDir()
	events := filepath.Join(dir, "events.jsonl")
	if err := os.WriteFile(events, []byte(`{"type":"tool.execution_start","data":{"toolName":"bash","arguments":{"command":"make test"}}}
{"type":"assistant.message","data":{"content":"Done.","phase":"final_answer"}}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	opts := RunOptions{LogFile: filepath.Join(dir, "agent.log"), Workdir: dir}
	if _, err := runCLI(context.Background(), "sh", []string{"-c", "cat " + events + "; echo stats >&2"}, nil, opts, &copilotParser{}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(opts.LogFile)
	log := string(raw)
	if !strings.Contains(log, "[tool] bash: make test") || !strings.Contains(log, "stats") {
		t.Errorf("log should hold tool lines and stderr:\n%s", log)
	}
	if strings.Contains(log, "tool.execution_start") {
		t.Errorf("raw events must not be logged:\n%s", log)
	}
	if got := ExtractSummary(log); got != "Done." {
		t.Errorf("summary = %q", got)
	}
}
