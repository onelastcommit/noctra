package github

import "time"

type Actor struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}

func (a Actor) IsBot() bool { return a.Type == "Bot" }

type PR struct {
	URL         string `json:"url"`
	Number      int    `json:"number"`
	Title       string `json:"title"`
	HeadRefName string `json:"headRefName"`
	Body        string `json:"body"`

	RepoURL string `json:"-"`
}

type Comment struct {
	ID        string    `json:"id"`
	Author    Actor     `json:"author"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"createdAt"`
	URL       string    `json:"url"`
}

type Review struct {
	ID          string    `json:"id"`
	Author      Actor     `json:"author"`
	Body        string    `json:"body"`
	State       string    `json:"state"`
	SubmittedAt time.Time `json:"submittedAt"`
}

type ReviewComment struct {
	ID        int64     `json:"id"`
	Author    Actor     `json:"user"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
	URL       string    `json:"html_url"`
	Path      string    `json:"path"`
	Line      int       `json:"line"`
	InReplyTo int64     `json:"in_reply_to_id"`
}

func (rc ReviewComment) ThreadRootID() int64 {
	if rc.InReplyTo != 0 {
		return rc.InReplyTo
	}
	return rc.ID
}

type Check struct {
	Typename     string `json:"__typename"`
	Name         string `json:"name"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	DetailsURL   string `json:"detailsUrl"`
	WorkflowName string `json:"workflowName"`
	Context      string `json:"context"`
	State        string `json:"state"`
	TargetURL    string `json:"targetUrl"`
}

func (c Check) CheckName() string {
	if c.Name != "" {
		return c.Name
	}
	return c.Context
}

func (c Check) URL() string {
	if c.DetailsURL != "" {
		return c.DetailsURL
	}
	return c.TargetURL
}

func (c Check) IsComplete() bool {
	if c.Status != "" {
		return c.Status == "COMPLETED"
	}
	return c.State != "" && c.State != "PENDING" && c.State != "EXPECTED"
}

func (c Check) IsFailure() bool {
	switch c.Conclusion {
	case "FAILURE", "TIMED_OUT", "CANCELLED", "ACTION_REQUIRED", "STARTUP_FAILURE":
		return true
	}
	switch c.State {
	case "FAILURE", "ERROR":
		return true
	}
	return false
}

type Details struct {
	URL               string          `json:"url"`
	Number            int             `json:"number"`
	State             string          `json:"state"`
	HeadRefOid        string          `json:"headRefOid"`
	Comments          []Comment       `json:"comments"`
	Reviews           []Review        `json:"reviews"`
	StatusCheckRollup []Check         `json:"statusCheckRollup"`
	ReviewComments    []ReviewComment `json:"-"`
}

func (d Details) IsOpen() bool { return d.State == "OPEN" }
