package watch

import (
	"time"

	"github.com/onelastcommit/noctra/internal/github"
)

type EventType string

const (
	EventComment EventType = "comment"
	EventReview  EventType = "review"
)

type Event struct {
	Type        EventType
	Author      github.Actor
	Body        string
	URL         string
	At          time.Time
	ReviewState string
	Path        string
	Line        int
	CommentID   string
	ThreadID    string
	ReplyTo     *github.ReviewComment
}

type CIFailure struct {
	SHA          string
	FailedChecks []github.Check
}

type PRChanges struct {
	PR            github.PR
	Details       *github.Details
	Events        []Event
	Skipped       []Event
	NewestComment time.Time
	NewestReview  time.Time
	CIFailure     *CIFailure
}
