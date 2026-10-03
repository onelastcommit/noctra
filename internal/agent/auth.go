package agent

import (
	"regexp"
	"strings"
)

const authScanLines = 40

var authFailureRe = regexp.MustCompile(`(?i)not logged in|please run /login|invalid api key|invalid x-api-key|authentication_error|oauth token (has )?expired|token (has )?expired|could not be refreshed|bad credentials|http 401|401 unauthori[sz]ed|no authentication information|not authenticated|authentication required|please (re-?)?log ?in|login required|session (has )?expired`)

func AuthFailureLine(output string) string {
	lines := strings.Split(output, "\n")
	scanned := 0
	for i := len(lines) - 1; i >= 0 && scanned < authScanLines; i-- {
		s := strings.TrimSpace(lines[i])
		if s == "" {
			continue
		}
		scanned++
		if authFailureRe.MatchString(s) {
			return redactLine(s)
		}
	}
	return ""
}

func redactLine(s string) string {
	s = strings.TrimSpace(secretRe.ReplaceAllString(s, "[redacted]"))
	const max = 300
	if r := []rune(s); len(r) > max {
		s = string(r[:max]) + "…"
	}
	return s
}

func LoginHint(backendName string) string {
	switch backendName {
	case "codex":
		return "run `codex login` on the host (or set OPENAI_API_KEY)"
	case "copilot":
		return "run `gh auth login` (OAuth web flow, not a classic PAT) or set COPILOT_GITHUB_TOKEN"
	case "antigravity":
		return "run `agy` once on the host to log in again"
	default:
		return "run `claude auth login` on the host (or set ANTHROPIC_API_KEY)"
	}
}
