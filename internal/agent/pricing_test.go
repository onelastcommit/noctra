package agent

import (
	"math"
	"testing"
)

func TestPricesForModel(t *testing.T) {
	tests := []struct {
		model string
		want  float64
	}{
		{"claude-opus-5-5", 20},
		{"claude-opus-5-5[1m]", 20},
		{"CLAUDE-OPUS-5-5", 20},
		{"claude-opus-5", 25},
		{"claude-opus-4-8", 25},
		{"claude-fable-5-2", 40},
		{"claude-fable-5-1", 50},
		{"claude-sonnet-5-5", 10},
		{"claude-sonnet-5", 10},
		{"claude-sonnet-4-6", 15},
		{"claude-haiku-5-5", 0.50},
		{"claude-haiku-4-5-20251001", 5},
		{"", 15},
		{"some-unknown-model", 15},
	}
	for _, tt := range tests {
		if got := PricesForModel(tt.model).OutputPerMTok; got != tt.want {
			t.Errorf("PricesForModel(%q).OutputPerMTok = %v, want %v", tt.model, got, tt.want)
		}
	}
}

func TestModelPricesEstimate(t *testing.T) {
	p := PricesForModel("claude-opus-5-5")
	got := p.Estimate(1_000_000, 1_000_000, 1_000_000, 1_000_000)
	want := 4.0 + 20.0 + 5.0 + 0.20
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("Estimate = %v, want %v", got, want)
	}
	if zero := p.Estimate(0, 0, 0, 0); zero != 0 {
		t.Errorf("Estimate of nothing = %v, want 0", zero)
	}
}

func TestClaudeUsageAddAndConvert(t *testing.T) {
	var cum claudeUsage
	cum.add(claudeUsage{InputTokens: 10, OutputTokens: 5, CacheCreationInputTokens: 2, CacheReadInputTokens: 100})
	cum.add(claudeUsage{InputTokens: 1, OutputTokens: 2, CacheCreationInputTokens: 3, CacheReadInputTokens: 4})

	if cum.total() != 127 {
		t.Fatalf("total = %d, want 127", cum.total())
	}

	u := cum.toUsage("claude-sonnet-5")
	if u.TotalTokens != 127 {
		t.Errorf("TotalTokens = %d, want 127", u.TotalTokens)
	}
	if u.OutputTokens != 7 {
		t.Errorf("OutputTokens = %d, want 7", u.OutputTokens)
	}
	if u.InputTokens != 120 {
		t.Errorf("InputTokens = %d, want 120", u.InputTokens)
	}
	if u.CostUSD <= 0 {
		t.Errorf("CostUSD = %v, want a positive estimate", u.CostUSD)
	}
}

func TestClaudeUsageToUsageAlwaysCostsSomething(t *testing.T) {
	cum := claudeUsage{CacheReadInputTokens: 5_000_000}
	if got := cum.toUsage("claude-opus-5").CostUSD; got <= 0 {
		t.Fatalf("a 5M-token aborted run estimated at $%v, want a non-zero cost", got)
	}
}
