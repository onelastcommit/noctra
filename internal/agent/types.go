package agent

import (
	"context"
	"time"
)

type RunOptions struct {
	Workdir       string
	Prompt        string
	LogFile       string
	Timeout       time.Duration
	UseAgentTeams bool
	MaxTokens     int64
	Env           []string
	PluginDirs    []string
	Model         string
	NoTools       bool
}

type Backend interface {
	Name() string
	Label() string
	CLI() string
	CoAuthor() string
	Run(ctx context.Context, opts RunOptions) (Usage, error)
	HasRateLimit(output string) bool
}

type FindingReply struct {
	Finding   int    `json:"finding"`
	Addressed bool   `json:"addressed"`
	Respond   *bool  `json:"respond,omitempty"`
	Reply     string `json:"reply"`
}

func (f FindingReply) Silent() bool {
	return f.Respond != nil && !*f.Respond && !f.Addressed
}

type FeedbackItem struct {
	Kind          string
	Author        string
	Human         bool
	Body          string
	State         string
	URL           string
	Path          string
	Line          int
	ReplyToAuthor string
	ReplyToBody   string
}

type CIItem struct {
	Name string
	URL  string
	Logs string
}

type FixPromptInput struct {
	Identifier     string
	Title          string
	Description    string
	PRNumber       int
	PRURL          string
	Feedback       []FeedbackItem
	CI             []CIItem
	RepoLessons    string
	PriorReasoning string
}

type BuildPromptInput struct {
	Identifier       string
	Title            string
	Description      string
	Comments         []string
	UseTeams         bool
	AutoReleaseLabel bool
	RepoLessons      string
}

type Usage struct {
	InputTokens  int64
	OutputTokens int64
	TotalTokens  int64
	CostUSD      float64
	Model        string
}

type TriageInput struct {
	PRNumber int
	PRTitle  string
	Feedback []FeedbackItem
}

type TriageVerdict struct {
	Act    bool   `json:"act"`
	Reason string `json:"reason"`
}
