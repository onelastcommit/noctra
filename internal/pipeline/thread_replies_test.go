package pipeline

import (
	"testing"

	"github.com/onelastcommit/noctra/internal/agent"
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
