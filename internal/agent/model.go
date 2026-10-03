package agent

import (
	"regexp"
	"sort"
	"strings"
)

var claudeModelIDRe = regexp.MustCompile(`^claude-([a-z]+)-(\d+)(?:-(\d{1,2}))?(?:-\d{8})?$`)

func ModelDisplayName(id string) string {
	id = strings.TrimSpace(id)
	if i := strings.IndexByte(id, '['); i >= 0 {
		id = id[:i]
	}
	if id == "" {
		return ""
	}
	if rest, ok := strings.CutPrefix(strings.ToLower(id), "gpt-"); ok {
		return "GPT-" + rest
	}
	m := claudeModelIDRe.FindStringSubmatch(strings.ToLower(id))
	if m == nil {
		return id
	}
	name := strings.ToUpper(m[1][:1]) + m[1][1:] + " " + m[2]
	if m[3] != "" {
		name += "." + m[3]
	}
	return name
}

func RunnerLabel(backendLabel, model string) string {
	name := ModelDisplayName(model)
	if name == "" {
		return backendLabel
	}
	return backendLabel + " (" + name + ")"
}

type modelShare struct {
	CostUSD      float64 `json:"costUSD"`
	OutputTokens int64   `json:"outputTokens"`
}

func primaryModel(usage map[string]modelShare) string {
	names := make([]string, 0, len(usage))
	for name := range usage {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		a, b := usage[names[i]], usage[names[j]]
		if a.CostUSD != b.CostUSD {
			return a.CostUSD > b.CostUSD
		}
		if a.OutputTokens != b.OutputTokens {
			return a.OutputTokens > b.OutputTokens
		}
		return names[i] < names[j]
	})
	if len(names) == 0 {
		return ""
	}
	return names[0]
}
