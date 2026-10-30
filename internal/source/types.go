package source

import (
	"context"
	"fmt"
	"regexp"
	"strings"
)

type TicketSource interface {
	Name() string
	Prepare(context.Context) error
	Fetch(context.Context) ([]Ticket, error)
	FetchByIdentifier(context.Context, string) (Ticket, error)
	FetchComments(context.Context, Ticket) ([]Comment, error)
	RemovePlanLabel(context.Context, Ticket) error
	BackToTrigger(context.Context, Ticket, string) error
	MarkReady(context.Context, Ticket, ReadyInfo) error
	Comment(context.Context, Ticket, string) error
}

type DoneMarker interface {
	MarkDone(context.Context, Ticket) error
}

type Archiver interface {
	Archive(context.Context, Ticket) error
}

type ReadyInfo struct {
	PRURL        string
	BackendLabel string
	ReviewState  string
}

type Comment struct {
	Body   string
	Author string
}

type Label struct {
	Name string
}

type Ticket struct {
	Source      string
	ID          string
	Identifier  string
	Title       string
	Description string
	URL         string
	ProjectName string
	RepoRef     string
	RepoBranch  string
	Comments    []Comment
	Labels      []Label

	SourceData any
}

var (
	repoDirectiveRe   = regexp.MustCompile(`(?im)^\s*Repo:\s*(.+?)\s*$`)
	branchDirectiveRe = regexp.MustCompile(`(?im)^\s*Branch:\s*(.+?)\s*$`)
)

func ParseRepoDirective(texts ...string) (string, string) {
	for _, src := range texts {
		m := repoDirectiveRe.FindStringSubmatch(src)
		if m == nil {
			continue
		}
		r := strings.TrimSpace(m[1])
		if r == "" {
			continue
		}
		var b string
		if bm := branchDirectiveRe.FindStringSubmatch(src); bm != nil {
			b = strings.TrimSpace(bm[1])
		}
		return r, b
	}
	return "", ""
}

var systemCommentMarkers = []string{
	"**Noctra",
	"**Nightshift",
	"This comment thread is synced",
}

func IsSystemComment(body string) bool {
	firstLine := ""
	for _, line := range strings.Split(body, "\n") {
		if s := strings.TrimSpace(line); s != "" {
			firstLine = s
			break
		}
	}
	if strings.HasPrefix(firstLine, ">") {
		return false
	}
	for _, m := range systemCommentMarkers {
		if strings.Contains(firstLine, m) {
			return true
		}
	}
	return false
}

func (t Ticket) ClarificationComments() []string {
	var out []string
	for _, c := range t.Comments {
		body := strings.TrimSpace(c.Body)
		if body == "" || IsSystemComment(body) {
			continue
		}
		author := strings.TrimSpace(c.Author)
		if author == "" {
			author = "Someone"
		}
		out = append(out, fmt.Sprintf("%s: %s", author, body))
	}
	return out
}

func (t Ticket) HasLabel(name string) bool {
	target := strings.ToLower(strings.TrimSpace(name))
	for _, l := range t.Labels {
		if strings.ToLower(strings.TrimSpace(l.Name)) == target {
			return true
		}
	}
	return false
}

const BackendLabelPrefix = "agent:"

func (t Ticket) BackendLabel() string {
	for _, l := range t.Labels {
		name := strings.ToLower(strings.TrimSpace(l.Name))
		if after, ok := strings.CutPrefix(name, BackendLabelPrefix); ok {
			if v := strings.TrimSpace(after); v != "" {
				return v
			}
		}
	}
	return ""
}

const PlanConfirmCommentPrefix = "📋 **Noctra: Implementation plan**"

func IsApprovalComment(body string) bool {
	s := strings.ToLower(strings.TrimSpace(body))
	switch s {
	case "go", "lgtm", "approved", "approve", "👍", ":thumbsup:", ":+1:":
		return true
	}
	return false
}

type GitHubIssuesConfig struct {
	Repos        []string
	TriggerLabel string
}

type JiraConfig struct {
	BaseURL        string
	UserEmail      string
	APIToken       string
	Project        string
	TriggerStatus  string
	InReviewStatus string
	TriggerLabel   string
}

type LinearConfig struct {
	TeamKey          string
	TriggerMode      string
	TriggerState     string
	TriggerLabel     string
	InReviewState    string
	DoneState        string
	PlanConfirmLabel string
}
