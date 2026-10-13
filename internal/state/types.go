package state

import "time"

type PRState struct {
	TicketID string `json:"ticket_id,omitempty"`

	AgentBackend string `json:"agent_backend,omitempty"`

	LastCommentAt time.Time `json:"last_comment_at,omitempty"`

	LastReviewAt time.Time `json:"last_review_at,omitempty"`

	LastCISHA string `json:"last_ci_sha,omitempty"`

	Iterations int `json:"iterations,omitempty"`

	LastIteratedAt time.Time `json:"last_iterated_at,omitempty"`

	LastPushedSHA string `json:"last_pushed_sha,omitempty"`

	MergedProcessed bool `json:"merged_processed,omitempty"`

	LastReasoning string `json:"last_reasoning,omitempty"`

	LastCIRunURL string `json:"last_ci_run_url,omitempty"`
}

type SweepState struct {
	LastRunAt time.Time `json:"last_run_at,omitempty"`
}

type PlanState struct {
	Source    string    `json:"source,omitempty"`
	IssueID   string    `json:"issue_id"`
	Plan      string    `json:"plan"`
	PlannedAt time.Time `json:"planned_at"`
}

type RunHistory struct {
	Identifier   string
	TicketID     string
	PRURL        string
	Repo         string
	AgentBackend string
	RunType      string
	StartedAt    time.Time
	FinishedAt   time.Time
	Status       string
	Iterations   int
}

type UsageEvent struct {
	OccurredAt   time.Time
	Source       string
	TicketID     string
	PRURL        string
	AgentBackend string
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	CostUSD      float64
}

type OAuthState struct {
	AccessToken  string    `json:"access_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	RefreshToken string    `json:"refresh_token,omitempty"`
}

type fileFormat struct {
	PRs    map[string]*PRState    `json:"prs"`
	Sweeps map[string]*SweepState `json:"sweeps,omitempty"`
	OAuth  *OAuthState            `json:"oauth,omitempty"`
}
