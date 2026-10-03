package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestModelDisplayName(t *testing.T) {
	cases := map[string]string{
		"claude-opus-5-5":            "Opus 5.5",
		"claude-opus-5-5[1m]":        "Opus 5.5",
		"claude-fable-5-1":           "Fable 5.1",
		"claude-sonnet-5-5":          "Sonnet 5.5",
		"claude-haiku-4-5-20251001":  "Haiku 4.5",
		"claude-opus-4-20250514":     "Opus 4",
		"Claude-Opus-5-5":            "Opus 5.5",
		"claude-3-5-sonnet-20241022": "claude-3-5-sonnet-20241022",
		"gpt-5.1-codex":              "gpt-5.1-codex",
		"":                           "",
		"  ":                         "",
	}
	for id, want := range cases {
		if got := ModelDisplayName(id); got != want {
			t.Errorf("ModelDisplayName(%q) = %q, want %q", id, got, want)
		}
	}
}

func TestRunnerLabel(t *testing.T) {
	if got := RunnerLabel("Claude Code", "claude-opus-5-5[1m]"); got != "Claude Code (Opus 5.5)" {
		t.Errorf("got %q", got)
	}
	if got := RunnerLabel("Codex", ""); got != "Codex" {
		t.Errorf("unknown model must leave the label alone, got %q", got)
	}
}

func TestPrimaryModel_PicksTheModelThatDidTheWork(t *testing.T) {
	got := primaryModel(map[string]modelShare{
		"claude-haiku-4-5-20251001": {CostUSD: 0.01, OutputTokens: 900},
		"claude-opus-5-5[1m]":       {CostUSD: 0.40, OutputTokens: 300},
	})
	if got != "claude-opus-5-5[1m]" {
		t.Errorf("got %q", got)
	}
	got = primaryModel(map[string]modelShare{
		"claude-haiku-4-5-20251001": {OutputTokens: 10},
		"claude-opus-5-5":           {OutputTokens: 900},
	})
	if got != "claude-opus-5-5" {
		t.Errorf("without costs, output tokens decide: got %q", got)
	}
	if primaryModel(nil) != "" {
		t.Error("empty usage should give no model")
	}
}

func TestParseClaudeJSON_ReadsPrimaryModel(t *testing.T) {
	out := `{"type":"result","result":"done","total_cost_usd":0.2,"usage":{"input_tokens":10,"output_tokens":5},` +
		`"modelUsage":{"claude-opus-5-5[1m]":{"outputTokens":5,"costUSD":0.18},"claude-haiku-4-5-20251001":{"outputTokens":40,"costUSD":0.02}}}`
	usage, _, ok := ParseClaudeJSON(out)
	if !ok || usage.Model != "claude-opus-5-5[1m]" {
		t.Fatalf("ok=%v model=%q", ok, usage.Model)
	}
}

func fakeClaude(t *testing.T, events string) RunOptions {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "events.jsonl"), []byte(events), 0o644); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncat " + filepath.Join(dir, "events.jsonl") + "\n"
	if err := os.WriteFile(filepath.Join(dir, "claude"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return RunOptions{Workdir: dir, Prompt: "x", LogFile: filepath.Join(dir, "run.log")}
}

func TestRunCapped_ModelFromResultEvent(t *testing.T) {
	opts := fakeClaude(t, `{"type":"assistant","message":{"model":"claude-haiku-4-5-20251001","usage":{"output_tokens":50}}}
{"type":"assistant","message":{"model":"claude-opus-5-5","usage":{"output_tokens":20}}}
{"type":"result","result":"ok","total_cost_usd":0.3,"usage":{"input_tokens":5,"output_tokens":70},"modelUsage":{"claude-opus-5-5":{"outputTokens":20,"costUSD":0.29},"claude-haiku-4-5-20251001":{"outputTokens":50,"costUSD":0.01}}}
`)
	opts.MaxTokens = 1_000_000
	usage, err := claudeBackend{}.Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if usage.Model != "claude-opus-5-5" {
		t.Fatalf("model = %q", usage.Model)
	}
}

func TestRunCapped_ModelSurvivesAbort(t *testing.T) {
	opts := fakeClaude(t, `{"type":"assistant","message":{"model":"claude-opus-5-5","usage":{"input_tokens":600,"output_tokens":600}}}
`)
	opts.MaxTokens = 1000
	usage, err := claudeBackend{}.Run(context.Background(), opts)
	if !errors.Is(err, ErrTokenCapExceeded) {
		t.Fatalf("want ErrTokenCapExceeded, got %v", err)
	}
	if usage.Model != "claude-opus-5-5" {
		t.Fatalf("aborted run lost its model: %q", usage.Model)
	}
}
