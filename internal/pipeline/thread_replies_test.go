package pipeline

import (
	"testing"

	"github.com/onelastcommit/noctra/internal/agent"
	"github.com/onelastcommit/noctra/internal/config"
	"github.com/onelastcommit/noctra/internal/watch"
)

func TestThreadRepliesFor_RoutesReplyCommentToItsThreadRoot(t *testing.T) {
	ch := watch.PRChanges{Events: []watch.Event{{
		Type:      watch.EventComment,
		Path:      "src/pages/docs.astro",
		CommentID: "4177195032",
		ThreadID:  "4177153334",
	}}}
	out := agent.FindingsStartMarker + "\n" +
		`[{"finding": 1, "addressed": false, "reply": "Already covered in the intro."}]` +
		"\n" + agent.FindingsEndMarker

	got := threadRepliesFor(ch, out, "")

	if len(got) != 1 {
		t.Fatalf("expected one thread reply, got %v", got)
	}
	r, ok := got[4177153334]
	if !ok {
		t.Fatalf("reply should be keyed by the thread root, got %v", got)
	}
	if r.Body != "Already covered in the intro." || r.Resolve {
		t.Errorf("reply: %+v", r)
	}
}

func codexSequence() watch.PRChanges {
	return watch.PRChanges{Events: []watch.Event{
		{Type: watch.EventComment, Body: "Codex Review Summary: in progress"},
		{Type: watch.EventReview, Body: "Here are some automated review suggestions"},
		{Type: watch.EventComment, Path: "metrics-repository.ts", CommentID: "7", ThreadID: "7", Body: "P1: restrict to the window"},
	}}
}

func findingsOutput(json string) string {
	return agent.FindingsStartMarker + "\n" + json + "\n" + agent.FindingsEndMarker
}

func TestRespondableEvents_DropsWhatTheAgentChoseNotToAnswer(t *testing.T) {
	out := findingsOutput(`[{"finding":1,"addressed":false,"respond":false,"reply":""},
{"finding":2,"addressed":false,"respond":false,"reply":""},
{"finding":3,"addressed":true,"reply":"Counted only meetings inside the window."}]`)
	ch := codexSequence()

	got := respondableEvents(ch, out)
	if len(got.Events) != 1 || got.Events[0].Path != "metrics-repository.ts" {
		t.Fatalf("only the inline finding should remain, got %+v", got.Events)
	}
	if hasConversationComment(got) {
		t.Error("a status comment the agent chose not to answer must not trigger a conversation reply")
	}
	replies := threadRepliesFor(ch, out, "abc1234")
	if r, ok := replies[7]; !ok || !r.Resolve || len(replies) != 1 {
		t.Errorf("the finding should get its own resolved thread reply, got %+v", replies)
	}
	if nothingToRespondTo(ch, out) {
		t.Error("a run with a real finding is not a quiet run")
	}
}

func TestNothingToRespondTo(t *testing.T) {
	statusOnly := watch.PRChanges{Events: []watch.Event{{Type: watch.EventComment, Body: "Codex Review Summary: in progress"}}}
	silent := findingsOutput(`[{"finding":1,"addressed":false,"respond":false,"reply":""}]`)
	answered := findingsOutput(`[{"finding":1,"addressed":false,"reply":"Nothing to change; the review is still running."}]`)

	if !nothingToRespondTo(statusOnly, silent) {
		t.Error("everything judged silent and no CI → quiet run")
	}
	if nothingToRespondTo(statusOnly, answered) {
		t.Error("the agent chose to answer → not quiet")
	}
	if nothingToRespondTo(statusOnly, "no findings block at all") {
		t.Error("without a findings block Noctra cannot know, so it must still reply")
	}
	withCI := statusOnly
	withCI.CIFailure = &watch.CIFailure{SHA: "abc"}
	if nothingToRespondTo(withCI, silent) {
		t.Error("a CI failure is always worth reporting")
	}
}

func TestShouldTriage(t *testing.T) {
	on := &Pipeline{cfg: &config.Config{PRTriage: true}}
	off := &Pipeline{cfg: &config.Config{PRTriage: false}}
	feedback := watch.PRChanges{Events: []watch.Event{{Type: watch.EventComment, Body: "status"}}}
	withCI := feedback
	withCI.CIFailure = &watch.CIFailure{SHA: "abc"}

	claude, _ := agent.New("claude")
	agy, _ := agent.New("antigravity")

	if !on.shouldTriage(feedback, claude) {
		t.Error("review activity alone should be triaged")
	}
	if on.shouldTriage(withCI, claude) {
		t.Error("a CI failure always engages, so it must not be triaged")
	}
	if on.shouldTriage(watch.PRChanges{}, claude) {
		t.Error("nothing to triage")
	}
	if off.shouldTriage(feedback, claude) {
		t.Error("PR_TRIAGE=false turns it off")
	}
	if on.shouldTriage(feedback, agy) {
		t.Error("Antigravity can't run without tools, so it must engage directly")
	}
}
