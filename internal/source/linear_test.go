package source

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/onelastcommit/noctra/internal/linear"
)

type linearCall struct {
	Op   string
	Vars map[string]any
}

type fakeLinear struct {
	mu        sync.Mutex
	log       []linearCall
	responses map[string]string
	failing   map[string]bool
}

func newFakeLinear(t *testing.T) (*fakeLinear, *linear.Client) {
	t.Helper()
	f := &fakeLinear{
		responses: map[string]string{
			"teamStates": `{"teams":{"nodes":[{"key":"ENG","states":{"nodes":[
				{"id":"s_next","name":"Next"},
				{"id":"s_progress","name":"In Progress"},
				{"id":"s_review","name":"In Review"},
				{"id":"s_done","name":"Done"}]}}]}}`,
			"issueLabels":   `{"issueLabels":{"nodes":[{"id":"l_noctra","name":"noctra"},{"id":"l_plan","name":"plan"}]}}`,
			"issueLabelIDs": `{"issue":{"labels":{"nodes":[{"id":"l_noctra"},{"id":"l_plan"},{"id":"l_bug"}]}}}`,
			"setState":      `{"issueUpdate":{"success":true}}`,
			"setLabels":     `{"issueUpdate":{"success":true}}`,
			"comment":       `{"commentCreate":{"success":true}}`,
			"archive":       `{"issueArchive":{"success":true}}`,
			"issueComments": `{"issue":{"comments":{"nodes":[{"body":"looks good","user":{"name":"Ada"}},{"body":"bot note"}]}}}`,
			"issueByID":     `{"issue":{"id":"issue_1","identifier":"ENG-1","title":"Fix it","state":{"name":"Next"}}}`,
			"triggerIssues": `{"teams":{"nodes":[{"issues":{"nodes":[{"id":"issue_1","identifier":"ENG-1","title":"By state"}]}}]}}`,
			"labeledIssues": `{"teams":{"nodes":[{"issues":{"nodes":[{"id":"issue_2","identifier":"ENG-2","title":"By label"}]}}]}}`,
		},
		failing: map[string]bool{},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		op := linearOp(body.Query, body.Variables)
		f.mu.Lock()
		f.log = append(f.log, linearCall{Op: op, Vars: body.Variables})
		fail := f.failing[op]
		data := f.responses[op]
		f.mu.Unlock()
		if fail {
			_, _ = w.Write([]byte(`{"errors":[{"message":"boom"}]}`))
			return
		}
		if data == "" {
			data = "{}"
		}
		_, _ = w.Write([]byte(`{"data":` + data + `}`))
	}))
	t.Cleanup(srv.Close)
	return f, &linear.Client{APIKey: "k", Endpoint: srv.URL, HTTP: srv.Client()}
}

func linearOp(query string, vars map[string]any) string {
	switch {
	case strings.Contains(query, "issueArchive"):
		return "archive"
	case strings.Contains(query, "commentCreate"):
		return "comment"
	case strings.Contains(query, "issueUpdate"):
		if _, ok := vars["labelIds"]; ok {
			return "setLabels"
		}
		return "setState"
	case strings.Contains(query, "issueLabels"):
		return "issueLabels"
	case strings.Contains(query, "filter: { labels"):
		return "labeledIssues"
	case strings.Contains(query, "filter: { state"):
		return "triggerIssues"
	case strings.Contains(query, "states { nodes"):
		return "teamStates"
	case strings.Contains(query, "issue(id: $id) { labels"):
		return "issueLabelIDs"
	case strings.Contains(query, "issue(id: $id) { comments"):
		return "issueComments"
	case strings.Contains(query, "issue(id: $id) { id"):
		return "issueByID"
	}
	return "unknown"
}

func (f *fakeLinear) fail(op string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failing[op] = true
}

func (f *fakeLinear) respond(op, data string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses[op] = data
}

func (f *fakeLinear) calls() []linearCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]linearCall(nil), f.log...)
}

func (f *fakeLinear) ops() string {
	var names []string
	for _, c := range f.calls() {
		names = append(names, c.Op)
	}
	return strings.Join(names, ",")
}

func (f *fakeLinear) last(t *testing.T, op string) map[string]any {
	t.Helper()
	calls := f.calls()
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].Op == op {
			return calls[i].Vars
		}
	}
	t.Fatalf("no %s call; calls = %s", op, f.ops())
	return nil
}

func labelIDs(vars map[string]any) string {
	raw, _ := vars["labelIds"].([]any)
	ids := make([]string, 0, len(raw))
	for _, v := range raw {
		ids = append(ids, v.(string))
	}
	return strings.Join(ids, ",")
}

func TestLinearPrepareStateMode(t *testing.T) {
	_, client := newFakeLinear(t)
	src := NewLinear(client, LinearConfig{
		TeamKey: "ENG", TriggerMode: "state", TriggerState: "Next",
		InProgressState: "In Progress", InReviewState: "In Review", DoneState: "Done",
		PlanConfirmLabel: "plan",
	})

	if err := src.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	got := src.StateIDs()
	if got.Trigger != "s_next" || got.InProgress != "s_progress" || got.InReview != "s_review" || got.Done != "s_done" {
		t.Fatalf("states = %+v", got)
	}
	if src.TriggerLabelID() != "" {
		t.Fatalf("TriggerLabelID = %q; want empty in state mode", src.TriggerLabelID())
	}
	if src.PlanConfirmLabelID() != "l_plan" {
		t.Fatalf("PlanConfirmLabelID = %q; want l_plan", src.PlanConfirmLabelID())
	}
}

func TestLinearPrepareLabelMode(t *testing.T) {
	_, client := newFakeLinear(t)
	src := NewLinear(client, LinearConfig{
		TeamKey: "ENG", TriggerMode: "label", TriggerState: "Missing",
		InReviewState: "In Review", TriggerLabel: "noctra",
	})

	if err := src.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if src.StateIDs().Trigger != "" {
		t.Fatalf("Trigger = %q; want unresolved in label mode", src.StateIDs().Trigger)
	}
	if src.TriggerLabelID() != "l_noctra" {
		t.Fatalf("TriggerLabelID = %q; want l_noctra", src.TriggerLabelID())
	}
}

func TestLinearPrepareErrors(t *testing.T) {
	tests := []struct {
		name string
		cfg  LinearConfig
		want string
	}{
		{
			name: "unknown trigger state",
			cfg:  LinearConfig{TeamKey: "ENG", TriggerState: "Nope", InReviewState: "In Review"},
			want: "resolve linear states",
		},
		{
			name: "unknown trigger label",
			cfg:  LinearConfig{TeamKey: "ENG", TriggerMode: "label", InReviewState: "In Review", TriggerLabel: "absent"},
			want: "resolve trigger label",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, client := newFakeLinear(t)
			err := NewLinear(client, tt.cfg).Prepare(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("err = %v; want containing %q", err, tt.want)
			}
		})
	}
}

func TestLinearPrepareToleratesMissingPlanLabel(t *testing.T) {
	_, client := newFakeLinear(t)
	src := NewLinear(client, LinearConfig{
		TeamKey: "ENG", TriggerState: "Next", InReviewState: "In Review", PlanConfirmLabel: "absent",
	})

	if err := src.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if src.PlanConfirmLabelID() != "" {
		t.Fatalf("PlanConfirmLabelID = %q; want empty", src.PlanConfirmLabelID())
	}
}

func TestLinearFetch(t *testing.T) {
	tests := []struct {
		mode    string
		wantOp  string
		wantVar string
		wantKey string
		wantID  string
	}{
		{mode: "state", wantOp: "triggerIssues", wantVar: "state", wantKey: "Next", wantID: "ENG-1"},
		{mode: "label", wantOp: "labeledIssues", wantVar: "label", wantKey: "noctra", wantID: "ENG-2"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			f, client := newFakeLinear(t)
			src := NewLinear(client, LinearConfig{TriggerMode: tt.mode, TriggerState: "Next", TriggerLabel: "noctra"})

			tickets, err := src.Fetch(context.Background())
			if err != nil {
				t.Fatalf("Fetch: %v", err)
			}
			if len(tickets) != 1 || tickets[0].Identifier != tt.wantID || tickets[0].Source != "linear" {
				t.Fatalf("tickets = %+v", tickets)
			}
			if got := f.last(t, tt.wantOp)[tt.wantVar]; got != tt.wantKey {
				t.Fatalf("%s = %v; want %s", tt.wantVar, got, tt.wantKey)
			}
		})
	}
}

func TestLinearFetchError(t *testing.T) {
	f, client := newFakeLinear(t)
	f.fail("triggerIssues")

	if _, err := NewLinear(client, LinearConfig{TriggerState: "Next"}).Fetch(context.Background()); err == nil {
		t.Fatal("Fetch: want error")
	}
}

func TestLinearFetchByIdentifierAttachesComments(t *testing.T) {
	_, client := newFakeLinear(t)

	ticket, err := NewLinear(client, LinearConfig{}).FetchByIdentifier(context.Background(), "ENG-1")
	if err != nil {
		t.Fatalf("FetchByIdentifier: %v", err)
	}
	if ticket.ID != "issue_1" || ticket.State != "Next" {
		t.Fatalf("ticket = %+v", ticket)
	}
	if len(ticket.Comments) != 2 || ticket.Comments[0].Author != "Ada" || ticket.Comments[1].Author != "" {
		t.Fatalf("comments = %+v", ticket.Comments)
	}
}

func TestLinearFetchByIdentifierToleratesCommentFailure(t *testing.T) {
	f, client := newFakeLinear(t)
	f.fail("issueComments")

	ticket, err := NewLinear(client, LinearConfig{}).FetchByIdentifier(context.Background(), "ENG-1")
	if err != nil {
		t.Fatalf("FetchByIdentifier: %v", err)
	}
	if ticket.Identifier != "ENG-1" || len(ticket.Comments) != 0 {
		t.Fatalf("ticket = %+v", ticket)
	}
}

func TestLinearFetchByIdentifierMissingIssue(t *testing.T) {
	f, client := newFakeLinear(t)
	f.respond("issueByID", `{"issue":null}`)

	if _, err := NewLinear(client, LinearConfig{}).FetchByIdentifier(context.Background(), "ENG-9"); err == nil {
		t.Fatal("FetchByIdentifier: want error")
	}
}

func TestLinearFetchComments(t *testing.T) {
	f, client := newFakeLinear(t)
	src := NewLinear(client, LinearConfig{})

	comments, err := src.FetchComments(context.Background(), Ticket{ID: "issue_1"})
	if err != nil {
		t.Fatalf("FetchComments: %v", err)
	}
	if len(comments) != 2 || comments[0] != (Comment{Body: "looks good", Author: "Ada"}) {
		t.Fatalf("comments = %+v", comments)
	}

	f.fail("issueComments")
	if _, err := src.FetchComments(context.Background(), Ticket{ID: "issue_1"}); err == nil {
		t.Fatal("FetchComments: want error")
	}
}

func TestLinearRemovePlanLabel(t *testing.T) {
	tests := []struct {
		name    string
		labelID string
		labels  []Label
		wantOps string
	}{
		{name: "label unresolved", labelID: "", labels: []Label{{Name: "plan"}}, wantOps: ""},
		{name: "ticket not labelled", labelID: "l_plan", labels: []Label{{Name: "bug"}}, wantOps: ""},
		{name: "labelled", labelID: "l_plan", labels: []Label{{Name: "Plan"}}, wantOps: "issueLabelIDs,setLabels"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, client := newFakeLinear(t)
			src := NewLinear(client, LinearConfig{PlanConfirmLabel: "plan"})
			src.planConfirmLabelID = tt.labelID

			if err := src.RemovePlanLabel(context.Background(), Ticket{ID: "issue_1", Labels: tt.labels}); err != nil {
				t.Fatalf("RemovePlanLabel: %v", err)
			}
			if got := f.ops(); got != tt.wantOps {
				t.Fatalf("ops = %q; want %q", got, tt.wantOps)
			}
			if tt.wantOps != "" {
				if got := labelIDs(f.last(t, "setLabels")); got != "l_noctra,l_bug" {
					t.Fatalf("labelIds = %s; want l_noctra,l_bug", got)
				}
			}
		})
	}
}

func TestLinearBackToTrigger(t *testing.T) {
	f, client := newFakeLinear(t)
	src := NewLinear(client, LinearConfig{})
	src.states = linear.StateIDs{Trigger: "s_next"}

	if err := src.BackToTrigger(context.Background(), Ticket{ID: "issue_1"}, "needs info"); err != nil {
		t.Fatalf("BackToTrigger: %v", err)
	}
	if got := f.ops(); got != "setState,comment" {
		t.Fatalf("ops = %q; want setState,comment", got)
	}
	if got := f.last(t, "setState")["stateId"]; got != "s_next" {
		t.Fatalf("stateId = %v; want s_next", got)
	}
	if got := f.last(t, "comment")["body"]; got != "needs info" {
		t.Fatalf("body = %v; want needs info", got)
	}
}

func TestLinearBackToTriggerWithoutTriggerState(t *testing.T) {
	f, client := newFakeLinear(t)
	src := NewLinear(client, LinearConfig{TriggerMode: "label"})

	if err := src.BackToTrigger(context.Background(), Ticket{ID: "issue_1"}, "needs info"); err != nil {
		t.Fatalf("BackToTrigger: %v", err)
	}
	if got := f.ops(); got != "comment" {
		t.Fatalf("ops = %q; want comment", got)
	}
}

func TestLinearBackToTriggerCommentsDespiteStateError(t *testing.T) {
	f, client := newFakeLinear(t)
	f.fail("setState")
	src := NewLinear(client, LinearConfig{})
	src.states = linear.StateIDs{Trigger: "s_next"}

	if err := src.BackToTrigger(context.Background(), Ticket{ID: "issue_1"}, "needs info"); err == nil {
		t.Fatal("BackToTrigger: want error")
	}
	if got := f.ops(); got != "setState,comment" {
		t.Fatalf("ops = %q; want setState,comment", got)
	}
}

func TestLinearMarkReady(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		wantOps string
	}{
		{name: "state mode", mode: "state", wantOps: "setState,comment"},
		{name: "label mode", mode: "label", wantOps: "issueLabelIDs,setLabels,setState,comment"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, client := newFakeLinear(t)
			src := NewLinear(client, LinearConfig{TriggerMode: tt.mode})
			src.states = linear.StateIDs{InReview: "s_review"}
			src.triggerLabelID = "l_noctra"
			info := ReadyInfo{PRURL: "https://github.com/o/r/pull/7", BackendLabel: "claude", ReviewState: "In Review"}

			if err := src.MarkReady(context.Background(), Ticket{ID: "issue_1"}, info); err != nil {
				t.Fatalf("MarkReady: %v", err)
			}
			if got := f.ops(); got != tt.wantOps {
				t.Fatalf("ops = %q; want %q", got, tt.wantOps)
			}
			if got := f.last(t, "setState")["stateId"]; got != "s_review" {
				t.Fatalf("stateId = %v; want s_review", got)
			}
			body, _ := f.last(t, "comment")["body"].(string)
			for _, want := range []string{info.PRURL, "via claude", "**In Review**"} {
				if !strings.Contains(body, want) {
					t.Fatalf("comment %q missing %q", body, want)
				}
			}
			if tt.mode == "label" {
				if got := labelIDs(f.last(t, "setLabels")); got != "l_plan,l_bug" {
					t.Fatalf("labelIds = %s; want l_plan,l_bug", got)
				}
			}
		})
	}
}

func TestLinearMarkReadyReturnsFirstError(t *testing.T) {
	f, client := newFakeLinear(t)
	f.fail("issueLabelIDs")
	f.fail("comment")
	src := NewLinear(client, LinearConfig{TriggerMode: "label"})
	src.states = linear.StateIDs{InReview: "s_review"}
	src.triggerLabelID = "l_noctra"

	err := src.MarkReady(context.Background(), Ticket{ID: "issue_1"}, ReadyInfo{})
	if err == nil || !strings.Contains(err.Error(), "fetch labels") {
		t.Fatalf("err = %v; want the label error", err)
	}
	if got := f.ops(); got != "issueLabelIDs,setState,comment" {
		t.Fatalf("ops = %q; want every step attempted", got)
	}
}

func TestLinearMarkDone(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		done    string
		wantOps string
		wantErr bool
	}{
		{name: "state mode", mode: "state", done: "s_done", wantOps: "setState"},
		{name: "state mode without done state", mode: "state", wantErr: true},
		{name: "label mode", mode: "label", done: "s_done", wantOps: "issueLabelIDs,setLabels,setState"},
		{name: "label mode without done state", mode: "label", wantOps: "issueLabelIDs,setLabels"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, client := newFakeLinear(t)
			src := NewLinear(client, LinearConfig{TriggerMode: tt.mode})
			src.states = linear.StateIDs{Done: tt.done}
			src.triggerLabelID = "l_noctra"

			err := src.MarkDone(context.Background(), Ticket{ID: "issue_1"})
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v; wantErr %v", err, tt.wantErr)
			}
			if got := f.ops(); got != tt.wantOps {
				t.Fatalf("ops = %q; want %q", got, tt.wantOps)
			}
		})
	}
}

func TestLinearArchive(t *testing.T) {
	tests := []struct {
		mode    string
		wantOps string
	}{
		{mode: "state", wantOps: "archive"},
		{mode: "label", wantOps: "issueLabelIDs,setLabels,archive"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			f, client := newFakeLinear(t)
			src := NewLinear(client, LinearConfig{TriggerMode: tt.mode})
			src.triggerLabelID = "l_noctra"

			if err := src.Archive(context.Background(), Ticket{ID: "issue_1"}); err != nil {
				t.Fatalf("Archive: %v", err)
			}
			if got := f.ops(); got != tt.wantOps {
				t.Fatalf("ops = %q; want %q", got, tt.wantOps)
			}
			if got := f.last(t, "archive")["id"]; got != "issue_1" {
				t.Fatalf("id = %v; want issue_1", got)
			}
		})
	}
}

func TestLinearArchiveFailure(t *testing.T) {
	f, client := newFakeLinear(t)
	f.respond("archive", `{"issueArchive":{"success":false}}`)

	if err := NewLinear(client, LinearConfig{}).Archive(context.Background(), Ticket{ID: "issue_1"}); err == nil {
		t.Fatal("Archive: want error")
	}
}
