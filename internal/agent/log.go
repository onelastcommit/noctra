package agent

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

func AttemptHeader(logFile string) error {
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintf(f, "--- Attempt %s ---\n", time.Now().Format(time.RFC3339))
	return err
}

func OffsetBefore(logFile string) int64 {
	info, err := os.Stat(logFile)
	if err != nil {
		return 0
	}
	return info.Size()
}

func ReadAfter(logFile string, offset int64) string {
	f, err := os.Open(logFile)
	if err != nil {
		return ""
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return ""
	}
	b, _ := io.ReadAll(f)
	return string(b)
}

var secretRe = regexp.MustCompile(`(?i)(sk-ant-[a-z0-9-]+|sk-[a-z0-9]{20,}|gh[opsur]_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]+|bearer\s+[A-Za-z0-9._~+/-]+=*)`)

func FailureDetail(output string, runErr error) string {
	detail := lastMeaningfulLine(output)
	if detail == "" && runErr != nil {
		detail = runErr.Error()
	}
	return redactLine(detail)
}

func lastMeaningfulLine(output string) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		s := strings.TrimSpace(lines[i])
		if s == "" || strings.HasPrefix(s, "DEBUG:") || strings.HasPrefix(s, "--- Attempt") {
			continue
		}
		return s
	}
	return ""
}

var blockedRe = regexp.MustCompile(`(?im)^BLOCKED:\s*(.*)$`)

func BlockedLine(output string) string {
	m := blockedRe.FindString(output)
	return m
}

var noChangesRe = regexp.MustCompile(`(?im)^NO_CHANGES:\s*(.*)$`)

func NoChangesLine(output string) string {
	matches := noChangesRe.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return ""
	}
	return strings.TrimSpace(matches[len(matches)-1][1])
}

var releaseRe = regexp.MustCompile(`(?im)^RELEASE:\s*(.+)$`)

func ReleaseBump(output string) string {
	matches := releaseRe.FindAllStringSubmatch(output, -1)
	if len(matches) == 0 {
		return ""
	}
	m := matches[len(matches)-1]
	val := strings.ToLower(strings.TrimSpace(m[1]))
	switch val {
	case "patch", "minor", "major", "none":
		return val
	}
	return ""
}

func ReleaseLabel(bump, defaultBump string) string {
	switch bump {
	case "none":
		return ""
	case "patch", "minor", "major":
		return "release:" + bump
	default:
		if defaultBump == "" {
			defaultBump = "patch"
		}
		return "release:" + defaultBump
	}
}

const (
	SummaryStartMarker = "===NOCTRA SUMMARY==="
	SummaryEndMarker   = "===END NOCTRA SUMMARY==="
)

func ExtractSummary(logContents string) string {
	const maxLines = 40

	last := lastAttempt(logContents)

	if s, ok := betweenMarkers(last); ok {
		return s
	}

	var kept []string
	for _, line := range strings.Split(stripUsageFooter(last), "\n") {
		if strings.HasPrefix(line, "DEBUG: ") {
			continue
		}
		kept = append(kept, line)
	}

	for len(kept) > 0 && strings.TrimSpace(kept[0]) == "" {
		kept = kept[1:]
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	if len(kept) > maxLines {
		kept = kept[len(kept)-maxLines:]
	}
	return strings.Join(kept, "\n")
}

func lastAttempt(logContents string) string {
	idx := strings.LastIndex(logContents, "--- Attempt ")
	if idx < 0 {
		return logContents
	}
	nl := strings.IndexByte(logContents[idx:], '\n')
	if nl < 0 {
		return ""
	}
	return logContents[idx+nl+1:]
}

func betweenMarkers(s string) (string, bool) {
	return between(s, SummaryStartMarker, SummaryEndMarker)
}

func between(s, startMarker, endMarker string) (string, bool) {
	start := strings.LastIndex(s, startMarker)
	if start < 0 {
		return "", false
	}
	rest := s[start+len(startMarker):]
	end := strings.Index(rest, endMarker)
	if end < 0 {
		return "", false
	}
	inner := strings.TrimSpace(rest[:end])
	if inner == "" {
		return "", false
	}
	return inner, true
}

var usageFooterRe = regexp.MustCompile(`(?is)\n\s*tokens used\b.*$`)

func stripUsageFooter(s string) string {
	return usageFooterRe.ReplaceAllString(s, "")
}
