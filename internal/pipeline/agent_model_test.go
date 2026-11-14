package pipeline

import (
	"context"
	"testing"

	"github.com/onelastcommit/noctra/internal/agent"
	"github.com/onelastcommit/noctra/internal/config"
)

type modelCapture struct {
	agent.Backend
	model string
}

func (c *modelCapture) Run(_ context.Context, opts agent.RunOptions) (agent.Usage, error) {
	c.model = opts.Model
	return agent.Usage{}, nil
}

func TestRunAgent_UsesConfiguredModelForBackend(t *testing.T) {
	cfg := &config.Config{AgentModels: map[string]string{"claude": "claude-sonnet-5-5", "codex": "gpt-5.5"}}
	p := &Pipeline{cfg: cfg}

	for backend, want := range map[string]string{"claude": "claude-sonnet-5-5", "codex": "gpt-5.5", "copilot": ""} {
		b, err := agent.New(backend)
		if err != nil {
			t.Fatal(err)
		}
		capture := &modelCapture{Backend: b}
		if _, err := p.runAgent(context.Background(), capture, agent.RunOptions{Prompt: "Fix the bug."}); err != nil {
			t.Fatal(err)
		}
		if capture.model != want {
			t.Errorf("%s: model = %q, want %q", backend, capture.model, want)
		}
	}
}

func TestRunAgent_ExplicitModelWins(t *testing.T) {
	claude, _ := agent.New("claude")
	capture := &modelCapture{Backend: claude}
	p := &Pipeline{cfg: &config.Config{AgentModels: map[string]string{"claude": "claude-opus-5-5"}}}

	if _, err := p.runAgent(context.Background(), capture, agent.RunOptions{Prompt: "Triage.", Model: "haiku"}); err != nil {
		t.Fatal(err)
	}
	if capture.model != "haiku" {
		t.Errorf("model = %q, want the triage model to override the configured one", capture.model)
	}
}

func TestModelSummary(t *testing.T) {
	if got := modelSummary(nil); got != "" {
		t.Errorf("modelSummary(nil) = %q, want empty", got)
	}
	got := modelSummary(map[string]string{"codex": "gpt-5.5", "claude": "claude-sonnet-5-5"})
	if want := "claude=claude-sonnet-5-5, codex=gpt-5.5"; got != want {
		t.Errorf("modelSummary = %q, want %q", got, want)
	}
}
