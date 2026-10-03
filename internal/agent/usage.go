package agent

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

type Usage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	CostUSD      float64
	Model        string
}

var (
	tokensUsedRe   = regexp.MustCompile(`(?i)tokens?\s+used[:\s]+([0-9,]+)`)
	totalTokensRe  = regexp.MustCompile(`(?i)total\s+tokens?[:\s]+([0-9,]+)`)
	inputTokensRe  = regexp.MustCompile(`(?i)input\s+tokens?[:\s]+([0-9,]+)`)
	outputTokensRe = regexp.MustCompile(`(?i)output\s+tokens?[:\s]+([0-9,]+)`)
	costRe         = regexp.MustCompile(`(?i)(?:total\s+)?cost[:\s]+\$([0-9,.]+)`)
)

func ParseUsage(output string) Usage {
	var u Usage

	if m := inputTokensRe.FindStringSubmatch(output); len(m) > 1 {
		u.InputTokens = parseCommaInt(m[1])
	}
	if m := outputTokensRe.FindStringSubmatch(output); len(m) > 1 {
		u.OutputTokens = parseCommaInt(m[1])
	}

	if m := totalTokensRe.FindStringSubmatch(output); len(m) > 1 {
		u.TotalTokens = parseCommaInt(m[1])
	} else if m := tokensUsedRe.FindStringSubmatch(output); len(m) > 1 {
		u.TotalTokens = parseCommaInt(m[1])
	}

	if u.TotalTokens == 0 && (u.InputTokens > 0 || u.OutputTokens > 0) {
		u.TotalTokens = u.InputTokens + u.OutputTokens
	}

	if m := costRe.FindStringSubmatch(output); len(m) > 1 {
		u.CostUSD, _ = strconv.ParseFloat(strings.ReplaceAll(m[1], ",", ""), 64)
	}

	return u
}

func parseCommaInt(s string) int64 {
	n, _ := strconv.ParseInt(strings.ReplaceAll(s, ",", ""), 10, 64)
	return n
}

type claudeResult struct {
	Type         string                `json:"type"`
	Result       string                `json:"result"`
	TotalCostUSD float64               `json:"total_cost_usd"`
	ModelUsage   map[string]modelShare `json:"modelUsage"`
	Usage        struct {
		InputTokens              int64 `json:"input_tokens"`
		OutputTokens             int64 `json:"output_tokens"`
		CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
	} `json:"usage"`
}

func ParseClaudeJSON(stdout string) (usage Usage, result string, ok bool) {
	s := strings.TrimSpace(stdout)
	if !strings.HasPrefix(s, "{") {
		return Usage{}, "", false
	}
	var r claudeResult
	if err := json.Unmarshal([]byte(s), &r); err != nil || r.Type != "result" {
		return Usage{}, "", false
	}
	in := r.Usage.InputTokens + r.Usage.CacheCreationInputTokens + r.Usage.CacheReadInputTokens
	return Usage{
		InputTokens:  in,
		OutputTokens: r.Usage.OutputTokens,
		TotalTokens:  in + r.Usage.OutputTokens,
		CostUSD:      r.TotalCostUSD,
		Model:        primaryModel(r.ModelUsage),
	}, r.Result, true
}
