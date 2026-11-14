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

func jiraTransitions(f *fakeJira, key string) {
	f.on(http.MethodGet, "/rest/api/3/issue/"+key+"/transitions", http.StatusOK, `{"transitions": [
		{"id": "11", "name": "Start", "to": {"name": "In Progress"}},
		{"id": "12", "name": "Reset", "to": {"name": "To Do"}},
		{"id": "13", "name": "Backlog", "to": {"name": "Backlog"}}
	]}`)
	f.on(http.MethodPost, "/rest/api/3/issue/"+key+"/transitions", http.StatusNoContent, ``)
}

func jiraTransitionID(t *testing.T, f *fakeJira, key string) any {
	t.Helper()
	exec, ok := f.find(http.MethodPost, "/rest/api/3/issue/"+key+"/transitions")
	if !ok {
		t.Fatalf("no transition executed; calls = %v", f.calls())
	}
	tr, _ := exec.Body["transition"].(map[string]any)
	return tr["id"]
}

func TestJiraMarkInProgressTransitions(t *testing.T) {
	f, src := newFakeJira(t)
	src.cfg.InProgressStatus = "In Progress"
	jiraTransitions(f, "PROJ-7")

	if err := src.MarkInProgress(context.Background(), Ticket{Identifier: "PROJ-7", State: "To Do"}); err != nil {
		t.Fatalf("MarkInProgress: %v", err)
	}
	if id := jiraTransitionID(t, f, "PROJ-7"); id != "11" {
		t.Fatalf("transition id = %v; want 11", id)
	}
}

func TestJiraMarkInProgressSkipsTicketAlreadyThere(t *testing.T) {
	f, src := newFakeJira(t)
	src.cfg.InProgressStatus = "In Progress"

	if err := src.MarkInProgress(context.Background(), Ticket{Identifier: "PROJ-7", State: "in progress"}); err != nil {
		t.Fatalf("MarkInProgress: %v", err)
	}
	if got := f.calls(); len(got) != 0 {
		t.Fatalf("calls = %v; want none", got)
	}
}

func TestJiraReleaseInProgressReturnsToTriggerStatus(t *testing.T) {
	f, src := newFakeJira(t)
	src.cfg.InProgressStatus = "In Progress"
	jiraTransitions(f, "PROJ-7")

	if err := src.ReleaseInProgress(context.Background(), Ticket{Identifier: "PROJ-7", State: "To Do"}); err != nil {
		t.Fatalf("ReleaseInProgress: %v", err)
	}
	if id := jiraTransitionID(t, f, "PROJ-7"); id != "12" {
		t.Fatalf("transition id = %v; want 12 (To Do)", id)
	}
}

func TestJiraReleaseInProgressRestoresOriginalStatusInLabelMode(t *testing.T) {
	f, src := newFakeJira(t)
	src.cfg.InProgressStatus = "In Progress"
	src.cfg.TriggerLabel = "noctra"
	jiraTransitions(f, "PROJ-7")

	if err := src.ReleaseInProgress(context.Background(), Ticket{Identifier: "PROJ-7", State: "Backlog"}); err != nil {
		t.Fatalf("ReleaseInProgress: %v", err)
	}
	if id := jiraTransitionID(t, f, "PROJ-7"); id != "13" {
		t.Fatalf("transition id = %v; want 13 (Backlog)", id)
	}
}

type linearStateRecorder struct {
	mu     sync.Mutex
	states []string
}

func (r *linearStateRecorder) client(t *testing.T) *linear.Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		var body struct {
			Query     string         `json:"query"`
			Variables map[string]any `json:"variables"`
		}
		_ = json.NewDecoder(req.Body).Decode(&body)
		if strings.Contains(body.Query, "issueUpdate") {
			r.mu.Lock()
			r.states = append(r.states, body.Variables["stateId"].(string))
			r.mu.Unlock()
		}
		_, _ = w.Write([]byte(`{"data":{"issueUpdate":{"success":true}}}`))
	}))
	t.Cleanup(srv.Close)
	return &linear.Client{APIKey: "k", Endpoint: srv.URL, HTTP: srv.Client()}
}

var linearTestStates = linear.StateIDs{
	Trigger:    "s_next",
	InProgress: "s_progress",
	InReview:   "s_review",
	ByName:     map[string]string{"Next": "s_next", "Backlog": "s_backlog", "In Progress": "s_progress"},
}

func TestLinearInProgressRoundTripInStateMode(t *testing.T) {
	rec := &linearStateRecorder{}
	src := NewLinear(rec.client(t), LinearConfig{TriggerMode: "state", InProgressState: "In Progress"})
	src.states = linearTestStates
	ticket := Ticket{ID: "issue_1", State: "Next"}

	if err := src.MarkInProgress(context.Background(), ticket); err != nil {
		t.Fatalf("MarkInProgress: %v", err)
	}
	if err := src.ReleaseInProgress(context.Background(), ticket); err != nil {
		t.Fatalf("ReleaseInProgress: %v", err)
	}
	if got := strings.Join(rec.states, ","); got != "s_progress,s_next" {
		t.Fatalf("states set = %s; want s_progress,s_next", got)
	}
}

func TestLinearReleaseRestoresOriginalStateInLabelMode(t *testing.T) {
	rec := &linearStateRecorder{}
	src := NewLinear(rec.client(t), LinearConfig{TriggerMode: "label", InProgressState: "In Progress"})
	src.states = linearTestStates

	if err := src.ReleaseInProgress(context.Background(), Ticket{ID: "issue_1", State: "Backlog"}); err != nil {
		t.Fatalf("ReleaseInProgress: %v", err)
	}
	if got := strings.Join(rec.states, ","); got != "s_backlog" {
		t.Fatalf("states set = %s; want s_backlog", got)
	}
}

func TestLinearInProgressIsNoOpWithoutTheState(t *testing.T) {
	rec := &linearStateRecorder{}
	src := NewLinear(rec.client(t), LinearConfig{TriggerMode: "state", InProgressState: "Doing"})
	src.states = linear.StateIDs{Trigger: "s_next", InReview: "s_review"}
	ticket := Ticket{ID: "issue_1", State: "Next"}

	if err := src.MarkInProgress(context.Background(), ticket); err != nil {
		t.Fatalf("MarkInProgress: %v", err)
	}
	if err := src.ReleaseInProgress(context.Background(), ticket); err != nil {
		t.Fatalf("ReleaseInProgress: %v", err)
	}
	if len(rec.states) != 0 {
		t.Fatalf("states set = %v; want none", rec.states)
	}
}

func TestLinearTicketCarriesItsState(t *testing.T) {
	ticket := linearTicket(linear.Issue{ID: "issue_1", State: &linear.WorkflowState{Name: "Backlog"}})
	if ticket.State != "Backlog" {
		t.Fatalf("State = %q; want Backlog", ticket.State)
	}
}
