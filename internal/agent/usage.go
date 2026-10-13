package agent

import (
	"regexp"
	"strconv"
	"strings"
)

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
