package pipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/onelastcommit/noctra/internal/agent"
	"github.com/onelastcommit/noctra/internal/config"
)

type promptCapture struct {
	agent.Backend
	prompt string
}

func (c *promptCapture) Run(_ context.Context, opts agent.RunOptions) (agent.Usage, error) {
	c.prompt = opts.Prompt
	return agent.Usage{}, nil
}

func TestRunAgent_AppendsLanguageRule(t *testing.T) {
	for variant, want := range map[string]string{"british": "British English", "american": "American English"} {
		claude, _ := agent.New("claude")
		capture := &promptCapture{Backend: claude}
		p := &Pipeline{cfg: &config.Config{EnglishVariant: variant}}

		if _, err := p.runAgent(context.Background(), capture, agent.RunOptions{Prompt: "Fix the bug."}); err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(capture.prompt, "Fix the bug.") || !strings.Contains(capture.prompt, want) {
			t.Errorf("%s: prompt = %q, want the task followed by a %s rule", variant, capture.prompt, want)
		}
	}
}

func TestEnglishLabel(t *testing.T) {
	if got := englishLabel("american"); got != "American English" {
		t.Errorf("englishLabel(american) = %q", got)
	}
	if got := englishLabel(""); got != "British English" {
		t.Errorf("englishLabel(\"\") = %q, want the British default", got)
	}
}
