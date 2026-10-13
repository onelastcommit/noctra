package dashboard

import (
	"context"

	"github.com/onelastcommit/noctra/internal/state"
	"github.com/onelastcommit/noctra/internal/sweep"
)

type SnapshotFunc func() any

type Controls interface {
	KillRun(identifier string) error
	RequeueTicket(ctx context.Context, identifier, extraContext, source string) error
	PauseDispatch() bool
	ResumeDispatch() bool
	ClearSkipped(identifier string) error
	TriggerSweep(opts sweep.PlanOptions) error
}

type Providers struct {
	Store           *state.Store
	SweepTasks      []sweep.Task
	MaxPRIterations int
	RepoPaths       func() []string
	LogDir          string
	Hub             *Hub
}

type historyEntry struct {
	Identifier string  `json:"identifier"`
	TicketID   string  `json:"ticket_id,omitempty"`
	PRURL      string  `json:"pr_url,omitempty"`
	Repo       string  `json:"repo"`
	Agent      string  `json:"agent_backend,omitempty"`
	RunType    string  `json:"run_type"`
	Status     string  `json:"status"`
	DurationS  float64 `json:"duration_s"`
	StartedAt  string  `json:"started_at"`
	FinishedAt string  `json:"finished_at,omitempty"`
	LinearURL  string  `json:"linear_url,omitempty"`
}

type prEntry struct {
	PRURL         string `json:"pr_url"`
	TicketID      string `json:"ticket_id,omitempty"`
	Iterations    int    `json:"iterations"`
	MaxIterations int    `json:"max_iterations"`
	Capped        bool   `json:"capped"`
	LastCISHA     string `json:"last_ci_sha,omitempty"`
	LastCIRunURL  string `json:"last_ci_run_url,omitempty"`
	LastReasoning string `json:"last_reasoning,omitempty"`
	LinearURL     string `json:"linear_url,omitempty"`
}

type sweepEntry struct {
	Repo          string  `json:"repo"`
	Task          string  `json:"task"`
	Description   string  `json:"description"`
	CooldownH     float64 `json:"cooldown_h"`
	LastRunAt     string  `json:"last_run_at,omitempty"`
	CooldownLeftH float64 `json:"cooldown_left_h"`
	Eligible      bool    `json:"eligible"`
}

type costBucket struct {
	Date        string  `json:"date"`
	CostUSD     float64 `json:"cost_usd"`
	TotalTokens int64   `json:"total_tokens"`
}

type costResponse struct {
	Buckets []costBucket `json:"buckets"`
}

type spendEntry struct {
	Agent       string  `json:"agent"`
	TotalTokens int64   `json:"total_tokens"`
	CostUSD     float64 `json:"cost_usd"`
}
