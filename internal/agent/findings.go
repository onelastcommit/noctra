package agent

import (
	"encoding/json"
	"strings"
)

const (
	FindingsStartMarker = "===NOCTRA FINDINGS==="
	FindingsEndMarker   = "===END NOCTRA FINDINGS==="
)

type FindingReply struct {
	Finding   int    `json:"finding"`
	Addressed bool   `json:"addressed"`
	Respond   *bool  `json:"respond,omitempty"`
	Reply     string `json:"reply"`
}

func (f FindingReply) Silent() bool {
	return f.Respond != nil && !*f.Respond && !f.Addressed
}

func ExtractFindingReplies(logContents string) ([]FindingReply, bool) {
	raw, ok := between(lastAttempt(logContents), FindingsStartMarker, FindingsEndMarker)
	if !ok {
		return nil, false
	}

	var parsed []FindingReply
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, false
	}

	order := make([]int, 0, len(parsed))
	byFinding := make(map[int]FindingReply, len(parsed))
	for _, f := range parsed {
		f.Reply = strings.TrimSpace(f.Reply)
		if f.Finding < 1 || (f.Reply == "" && !f.Silent()) {
			continue
		}
		if _, seen := byFinding[f.Finding]; !seen {
			order = append(order, f.Finding)
		}
		byFinding[f.Finding] = f
	}
	if len(order) == 0 {
		return nil, false
	}

	out := make([]FindingReply, 0, len(order))
	for _, n := range order {
		out = append(out, byFinding[n])
	}
	return out, true
}
