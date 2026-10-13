package linear

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

type Project struct {
	ID          string `json:"id,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`
	SlugID      string `json:"slugId,omitempty"`
	URL         string `json:"url,omitempty"`
}

var (
	repoDirectiveRe   = regexp.MustCompile(`(?im)^\s*Repo:\s*(.+?)\s*$`)
	branchDirectiveRe = regexp.MustCompile(`(?im)^\s*Branch:\s*(.+?)\s*$`)
)

func (p *Project) RepoDirective() (string, string) {
	if p == nil {
		return "", ""
	}
	for _, src := range []string{p.Content, p.Description} {
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

func UpsertRepoDirective(content, ref, branch string) string {
	ref = strings.TrimSpace(ref)
	branch = strings.TrimSpace(branch)

	block := []string{"Repo: " + ref}
	if branch != "" {
		block = append(block, "Branch: "+branch)
	}

	var kept []string
	replaced := false
	for _, line := range strings.Split(content, "\n") {
		switch {
		case repoDirectiveRe.MatchString(line):
			if replaced {
				continue
			}
			replaced = true
			kept = append(kept, block...)
		case branchDirectiveRe.MatchString(line):
		default:
			kept = append(kept, line)
		}
	}

	if !replaced {
		if strings.TrimSpace(content) != "" {
			block = append(block, "")
		}
		kept = append(block, kept...)
	}
	return strings.Join(kept, "\n")
}

func MatchProjects(projects []Project, query string) []Project {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}

	if strings.Contains(strings.ToLower(query), "linear.app/") {
		for _, p := range projects {
			if p.SlugID != "" && strings.Contains(query, p.SlugID) {
				return []Project{p}
			}
			if p.URL != "" && strings.HasPrefix(query, p.URL) {
				return []Project{p}
			}
		}
		return nil
	}

	for _, p := range projects {
		if strings.EqualFold(strings.TrimSpace(p.Name), query) {
			return []Project{p}
		}
	}

	var partial []Project
	lower := strings.ToLower(query)
	for _, p := range projects {
		if strings.Contains(strings.ToLower(p.Name), lower) {
			partial = append(partial, p)
		}
	}
	return partial
}

type WorkflowState struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Team struct {
	Key string `json:"key"`
}

type User struct {
	Name string `json:"name"`
}

type Label struct {
	Name string `json:"name"`
}

type LabelConnection struct {
	Nodes []Label `json:"nodes"`
}

type Comment struct {
	Body string `json:"body"`
	User *User  `json:"user,omitempty"`
}

type CommentConnection struct {
	Nodes []Comment `json:"nodes"`
}

type Issue struct {
	ID          string            `json:"id"`
	Identifier  string            `json:"identifier"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	URL         string            `json:"url"`
	Project     *Project          `json:"project,omitempty"`
	Team        *Team             `json:"team,omitempty"`
	State       *WorkflowState    `json:"state,omitempty"`
	Assignee    *User             `json:"assignee,omitempty"`
	Comments    CommentConnection `json:"comments,omitempty"`
	Labels      LabelConnection   `json:"labels,omitempty"`
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

func (i Issue) ClarificationComments() []string {
	var out []string
	for _, c := range i.Comments.Nodes {
		body := strings.TrimSpace(c.Body)
		if body == "" || IsSystemComment(body) {
			continue
		}
		author := "Someone"
		if c.User != nil && c.User.Name != "" {
			author = c.User.Name
		}
		out = append(out, fmt.Sprintf("%s: %s", author, body))
	}
	return out
}

func (i Issue) ProjectName() string {
	if i.Project == nil {
		return ""
	}
	return i.Project.Name
}

func (i Issue) StateName() string {
	if i.State == nil {
		return ""
	}
	return i.State.Name
}

func (i Issue) AssigneeName() string {
	if i.Assignee == nil {
		return ""
	}
	return i.Assignee.Name
}

const PlanConfirmCommentPrefix = "📋 **Noctra: Implementation plan**"

func (i Issue) HasLabel(name string) bool {
	target := strings.ToLower(strings.TrimSpace(name))
	for _, l := range i.Labels.Nodes {
		if strings.ToLower(strings.TrimSpace(l.Name)) == target {
			return true
		}
	}
	return false
}

func IsApprovalComment(body string) bool {
	s := strings.ToLower(strings.TrimSpace(body))
	switch s {
	case "go", "lgtm", "approved", "approve", "👍", ":thumbsup:", ":+1:":
		return true
	}
	return false
}

const BackendLabelPrefix = "agent:"

func (i Issue) BackendLabel() string {
	for _, l := range i.Labels.Nodes {
		name := strings.ToLower(strings.TrimSpace(l.Name))
		if strings.HasPrefix(name, BackendLabelPrefix) {
			if v := strings.TrimSpace(strings.TrimPrefix(name, BackendLabelPrefix)); v != "" {
				return v
			}
		}
	}
	return ""
}

type TokenStore interface {
	LoadOAuth() (access string, expiresAt time.Time, refresh string)
	SaveOAuth(access string, expiresAt time.Time, refresh string) error
}

type TokenManagerConfig struct {
	ClientID     string
	ClientSecret string
	RefreshToken string
	Scope        string
	Endpoint     string
	HTTP         *http.Client
	Store        TokenStore
}

type StateIDs struct {
	Trigger  string
	InReview string
	Done     string
}

type WorkflowStateID struct {
	ID   string
	Name string
}

type StateCount struct {
	State    string
	Type     string
	Position float64
	Count    int
}
