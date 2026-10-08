package source

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type jiraRequest struct {
	Method string
	Path   string
	Query  string
	Auth   string
	CType  string
	Body   map[string]any
}

type fakeJira struct {
	t        *testing.T
	mu       sync.Mutex
	requests []jiraRequest
	routes   map[string]func(w http.ResponseWriter)
}

func newFakeJira(t *testing.T) (*fakeJira, *JiraSource) {
	t.Helper()
	f := &fakeJira{t: t, routes: map[string]func(w http.ResponseWriter){}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	src := NewJira(JiraConfig{
		BaseURL:        srv.URL + "/",
		UserEmail:      "bot@example.com",
		APIToken:       "secret",
		Project:        "PROJ",
		TriggerStatus:  "To Do",
		InReviewStatus: "In Review",
	})
	if err := src.Prepare(context.Background()); err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	return f, src
}

func (f *fakeJira) on(method, path string, status int, body string) {
	f.routes[method+" "+path] = func(w http.ResponseWriter) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
}

func (f *fakeJira) serve(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	req := jiraRequest{
		Method: r.Method,
		Path:   r.URL.Path,
		Query:  r.URL.RawQuery,
		Auth:   r.Header.Get("Authorization"),
		CType:  r.Header.Get("Content-Type"),
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &req.Body); err != nil {
			f.t.Errorf("%s %s: request body is not JSON: %v", r.Method, r.URL.Path, err)
		}
	}
	f.mu.Lock()
	f.requests = append(f.requests, req)
	f.mu.Unlock()
	route, ok := f.routes[r.Method+" "+r.URL.Path]
	if !ok {
		f.t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	route(w)
}

func (f *fakeJira) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.requests))
	for _, r := range f.requests {
		out = append(out, r.Method+" "+r.Path)
	}
	return out
}

func (f *fakeJira) find(method, path string) (jiraRequest, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.requests {
		if r.Method == method && r.Path == path {
			return r, true
		}
	}
	return jiraRequest{}, false
}

func adfParagraphTexts(t *testing.T, body map[string]any) []any {
	t.Helper()
	doc, _ := body["body"].(map[string]any)
	if doc["type"] != "doc" {
		t.Fatalf("comment body type = %v; want doc", doc["type"])
	}
	paras, _ := doc["content"].([]any)
	if len(paras) != 1 {
		t.Fatalf("comment has %d top-level nodes; want 1 paragraph", len(paras))
	}
	para, _ := paras[0].(map[string]any)
	content, _ := para["content"].([]any)
	return content
}

const jiraIssueJSON = `{
	"id": "10001",
	"key": "PROJ-7",
	"fields": {
		"summary": "Add retries",
		"description": {"type": "doc", "content": [
			{"type": "paragraph", "content": [{"type": "text", "text": "Repo: acme/api"}]},
			{"type": "paragraph", "content": [{"type": "text", "text": "Retry on 503."}]}
		]},
		"status": {"name": "To Do"},
		"project": {"key": "PROJ", "name": "Project"},
		"labels": ["agent:codex"],
		"comment": {"comments": [
			{"body": {"type": "doc", "content": [{"type": "paragraph", "content": [{"type": "text", "text": "Use backoff."}]}]}, "author": {"displayName": "Bob"}}
		]}
	}
}`

func TestJiraFetchSearchesWithJQLAndBasicAuth(t *testing.T) {
	f, src := newFakeJira(t)
	f.on(http.MethodPost, "/rest/api/3/search", http.StatusOK, `{"issues": [`+jiraIssueJSON+`]}`)

	tickets, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	req, ok := f.find(http.MethodPost, "/rest/api/3/search")
	if !ok {
		t.Fatalf("search endpoint was not called; calls = %v", f.calls())
	}
	wantAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("bot@example.com:secret"))
	if req.Auth != wantAuth {
		t.Fatalf("Authorization = %q; want %q", req.Auth, wantAuth)
	}
	if req.CType != "application/json" {
		t.Fatalf("Content-Type = %q; want application/json", req.CType)
	}
	if got, want := req.Body["jql"], `project = "PROJ" AND status = "To Do" ORDER BY created ASC`; got != want {
		t.Fatalf("jql = %v; want %q", got, want)
	}
	fields, _ := req.Body["fields"].([]any)
	if !containsAny(fields, "comment") || !containsAny(fields, "labels") {
		t.Fatalf("fields = %v; want comment and labels requested", fields)
	}

	if len(tickets) != 1 {
		t.Fatalf("got %d tickets; want 1", len(tickets))
	}
	tk := tickets[0]
	if tk.Identifier != "PROJ-7" || tk.Title != "Add retries" {
		t.Fatalf("ticket = %+v", tk)
	}
	if !strings.HasSuffix(tk.URL, "/browse/PROJ-7") || strings.Contains(tk.URL, "//browse") {
		t.Fatalf("URL = %q; want trimmed base URL + /browse/PROJ-7", tk.URL)
	}
	if tk.RepoRef != "acme/api" {
		t.Fatalf("RepoRef = %q; want acme/api", tk.RepoRef)
	}
	if got := tk.BackendLabel(); got != "codex" {
		t.Fatalf("BackendLabel = %q; want codex", got)
	}
	if got := tk.ClarificationComments(); len(got) != 1 || got[0] != "Bob: Use backoff." {
		t.Fatalf("ClarificationComments = %#v", got)
	}
}

func TestJiraFetchEmptyResultIsNotNil(t *testing.T) {
	f, src := newFakeJira(t)
	f.on(http.MethodPost, "/rest/api/3/search", http.StatusOK, `{"issues": []}`)

	tickets, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if tickets == nil || len(tickets) != 0 {
		t.Fatalf("tickets = %#v; want empty non-nil slice", tickets)
	}
}

func TestJiraHTTPErrorsIncludeStatusAndBody(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		status int
		call   func(*JiraSource) error
		want   []string
	}{
		{
			name:   "search",
			method: http.MethodPost,
			path:   "/rest/api/3/search",
			status: http.StatusBadRequest,
			call: func(s *JiraSource) error {
				_, err := s.Fetch(context.Background())
				return err
			},
			want: []string{"jira search", "HTTP 400", "boom"},
		},
		{
			name:   "get issue",
			method: http.MethodGet,
			path:   "/rest/api/3/issue/PROJ-9",
			status: http.StatusNotFound,
			call: func(s *JiraSource) error {
				_, err := s.FetchByIdentifier(context.Background(), "PROJ-9")
				return err
			},
			want: []string{"jira get issue PROJ-9", "HTTP 404", "boom"},
		},
		{
			name:   "get comments",
			method: http.MethodGet,
			path:   "/rest/api/3/issue/PROJ-9/comment",
			status: http.StatusForbidden,
			call: func(s *JiraSource) error {
				_, err := s.FetchComments(context.Background(), Ticket{Identifier: "PROJ-9"})
				return err
			},
			want: []string{"jira get comments PROJ-9", "HTTP 403", "boom"},
		},
		{
			name:   "add comment expects 201",
			method: http.MethodPost,
			path:   "/rest/api/3/issue/PROJ-9/comment",
			status: http.StatusOK,
			call: func(s *JiraSource) error {
				return s.Comment(context.Background(), Ticket{Identifier: "PROJ-9"}, "hi")
			},
			want: []string{"jira add comment PROJ-9", "HTTP 200", "boom"},
		},
		{
			name:   "list transitions",
			method: http.MethodGet,
			path:   "/rest/api/3/issue/PROJ-9/transitions",
			status: http.StatusUnauthorized,
			call: func(s *JiraSource) error {
				return s.transitionTo(context.Background(), "PROJ-9", "In Review")
			},
			want: []string{"jira list transitions PROJ-9", "HTTP 401", "boom"},
		},
		{
			name:   "remove label expects 204",
			method: http.MethodPut,
			path:   "/rest/api/3/issue/PROJ-9",
			status: http.StatusOK,
			call: func(s *JiraSource) error {
				return s.removeLabel(context.Background(), "PROJ-9", "noctra")
			},
			want: []string{"jira remove label PROJ-9", "HTTP 200", "boom"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, src := newFakeJira(t)
			f.on(tt.method, tt.path, tt.status, "boom")
			err := tt.call(src)
			if err == nil {
				t.Fatal("expected an error")
			}
			for _, w := range tt.want {
				if !strings.Contains(err.Error(), w) {
					t.Fatalf("error %q does not contain %q", err, w)
				}
			}
		})
	}
}

func TestJiraDecodeErrorsAreWrapped(t *testing.T) {
	f, src := newFakeJira(t)
	f.on(http.MethodGet, "/rest/api/3/issue/PROJ-1", http.StatusOK, `{not json`)
	_, err := src.FetchByIdentifier(context.Background(), "PROJ-1")
	if err == nil || !strings.Contains(err.Error(), "jira get issue PROJ-1: decode") {
		t.Fatalf("err = %v; want wrapped decode error", err)
	}
}

func TestJiraFetchByIdentifierRequestsFields(t *testing.T) {
	f, src := newFakeJira(t)
	f.on(http.MethodGet, "/rest/api/3/issue/PROJ-7", http.StatusOK, jiraIssueJSON)

	tk, err := src.FetchByIdentifier(context.Background(), "PROJ-7")
	if err != nil {
		t.Fatalf("FetchByIdentifier: %v", err)
	}
	if tk.Identifier != "PROJ-7" || tk.Description != "Repo: acme/api\nRetry on 503." {
		t.Fatalf("ticket = %+v", tk)
	}
	req, _ := f.find(http.MethodGet, "/rest/api/3/issue/PROJ-7")
	if !strings.Contains(req.Query, "fields=summary,description,status,project,labels,comment") {
		t.Fatalf("query = %q; want the ticket fields requested", req.Query)
	}
	if req.CType != "" {
		t.Fatalf("GET sent Content-Type %q; want none", req.CType)
	}
}

func TestJiraFetchCommentsFlattensADF(t *testing.T) {
	f, src := newFakeJira(t)
	f.on(http.MethodGet, "/rest/api/3/issue/PROJ-7/comment", http.StatusOK, `{"comments": [
		{"body": {"type": "doc", "content": [{"type": "paragraph", "content": [
			{"type": "text", "text": "line one"}, {"type": "hardBreak"}, {"type": "text", "text": "line two"}
		]}]}, "author": {"displayName": "Carol"}},
		{"body": null, "author": {"displayName": "Dave"}}
	]}`)

	comments, err := src.FetchComments(context.Background(), Ticket{Identifier: "PROJ-7"})
	if err != nil {
		t.Fatalf("FetchComments: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("got %d comments; want 2", len(comments))
	}
	if comments[0].Author != "Carol" || comments[0].Body != "line one\nline two" {
		t.Fatalf("comments[0] = %+v", comments[0])
	}
	if comments[1].Author != "Dave" || comments[1].Body != "" {
		t.Fatalf("comments[1] = %+v; want empty body for a null ADF document", comments[1])
	}
}

func TestJiraCommentEncodesNewlinesAsHardBreaks(t *testing.T) {
	f, src := newFakeJira(t)
	f.on(http.MethodPost, "/rest/api/3/issue/PROJ-7/comment", http.StatusCreated, `{}`)

	if err := src.Comment(context.Background(), Ticket{Identifier: "PROJ-7"}, "first\n\nthird"); err != nil {
		t.Fatalf("Comment: %v", err)
	}
	req, ok := f.find(http.MethodPost, "/rest/api/3/issue/PROJ-7/comment")
	if !ok {
		t.Fatalf("comment endpoint not called; calls = %v", f.calls())
	}
	content := adfParagraphTexts(t, req.Body)
	var kinds []string
	for _, n := range content {
		node := n.(map[string]any)
		if node["type"] == "text" {
			kinds = append(kinds, "text:"+node["text"].(string))
		} else {
			kinds = append(kinds, node["type"].(string))
		}
	}
	want := []string{"text:first", "hardBreak", "hardBreak", "text:third"}
	if strings.Join(kinds, "|") != strings.Join(want, "|") {
		t.Fatalf("ADF nodes = %v; want %v (no empty text nodes)", kinds, want)
	}
}

func TestJiraBackToTriggerOnlyComments(t *testing.T) {
	f, src := newFakeJira(t)
	f.on(http.MethodPost, "/rest/api/3/issue/PROJ-7/comment", http.StatusCreated, `{}`)

	if err := src.BackToTrigger(context.Background(), Ticket{Identifier: "PROJ-7"}, "needs info"); err != nil {
		t.Fatalf("BackToTrigger: %v", err)
	}
	if got := f.calls(); len(got) != 1 || got[0] != "POST /rest/api/3/issue/PROJ-7/comment" {
		t.Fatalf("calls = %v; want a single comment", got)
	}
}

func TestJiraMarkReadyTransitionsMatchingStatusCaseInsensitively(t *testing.T) {
	f, src := newFakeJira(t)
	f.on(http.MethodGet, "/rest/api/3/issue/PROJ-7/transitions", http.StatusOK, `{"transitions": [
		{"id": "11", "name": "Start", "to": {"name": "In Progress"}},
		{"id": "21", "name": "Review", "to": {"name": "  in review "}}
	]}`)
	f.on(http.MethodPost, "/rest/api/3/issue/PROJ-7/transitions", http.StatusNoContent, ``)
	f.on(http.MethodPost, "/rest/api/3/issue/PROJ-7/comment", http.StatusCreated, `{}`)

	err := src.MarkReady(context.Background(), Ticket{Identifier: "PROJ-7"}, ReadyInfo{
		PRURL:        "https://github.com/acme/api/pull/5",
		BackendLabel: "claude",
		ReviewState:  "In Review",
	})
	if err != nil {
		t.Fatalf("MarkReady: %v", err)
	}

	want := []string{
		"GET /rest/api/3/issue/PROJ-7/transitions",
		"POST /rest/api/3/issue/PROJ-7/transitions",
		"POST /rest/api/3/issue/PROJ-7/comment",
	}
	if got := f.calls(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("calls = %v; want %v (no label removal in status mode)", got, want)
	}

	exec, _ := f.find(http.MethodPost, "/rest/api/3/issue/PROJ-7/transitions")
	tr, _ := exec.Body["transition"].(map[string]any)
	if tr["id"] != "21" {
		t.Fatalf("transition id = %v; want 21", tr["id"])
	}

	comment, _ := f.find(http.MethodPost, "/rest/api/3/issue/PROJ-7/comment")
	var text strings.Builder
	for _, n := range adfParagraphTexts(t, comment.Body) {
		if s, ok := n.(map[string]any)["text"].(string); ok {
			text.WriteString(s)
		}
	}
	for _, w := range []string{"https://github.com/acme/api/pull/5", "claude", "In Review"} {
		if !strings.Contains(text.String(), w) {
			t.Fatalf("comment %q missing %q", text.String(), w)
		}
	}
}

func TestJiraMarkReadyRemovesTriggerLabelInLabelMode(t *testing.T) {
	f, src := newFakeJira(t)
	src.cfg.TriggerLabel = "noctra"
	f.on(http.MethodGet, "/rest/api/3/issue/PROJ-7/transitions", http.StatusOK, `{"transitions": [
		{"id": "21", "to": {"name": "In Review"}}
	]}`)
	f.on(http.MethodPost, "/rest/api/3/issue/PROJ-7/transitions", http.StatusNoContent, ``)
	f.on(http.MethodPut, "/rest/api/3/issue/PROJ-7", http.StatusNoContent, ``)
	f.on(http.MethodPost, "/rest/api/3/issue/PROJ-7/comment", http.StatusCreated, `{}`)

	if err := src.MarkReady(context.Background(), Ticket{Identifier: "PROJ-7"}, ReadyInfo{}); err != nil {
		t.Fatalf("MarkReady: %v", err)
	}
	put, ok := f.find(http.MethodPut, "/rest/api/3/issue/PROJ-7")
	if !ok {
		t.Fatalf("label was not removed; calls = %v", f.calls())
	}
	update, _ := put.Body["update"].(map[string]any)
	labels, _ := update["labels"].([]any)
	if len(labels) != 1 || labels[0].(map[string]any)["remove"] != "noctra" {
		t.Fatalf("label update = %v; want a single remove of noctra", update)
	}
}

func TestJiraMarkReadyMissingTransitionStillCommentsAndReportsFirstError(t *testing.T) {
	f, src := newFakeJira(t)
	src.cfg.TriggerLabel = "noctra"
	f.on(http.MethodGet, "/rest/api/3/issue/PROJ-7/transitions", http.StatusOK, `{"transitions": [
		{"id": "11", "to": {"name": "In Progress"}},
		{"id": "31", "to": {"name": "Done"}}
	]}`)
	f.on(http.MethodPut, "/rest/api/3/issue/PROJ-7", http.StatusBadRequest, `label gone`)
	f.on(http.MethodPost, "/rest/api/3/issue/PROJ-7/comment", http.StatusCreated, `{}`)

	err := src.MarkReady(context.Background(), Ticket{Identifier: "PROJ-7"}, ReadyInfo{})
	if err == nil {
		t.Fatal("expected an error when no transition reaches the in-review status")
	}
	for _, w := range []string{`no transition to status "In Review"`, `"In Progress", "Done"`} {
		if !strings.Contains(err.Error(), w) {
			t.Fatalf("error %q does not contain %q", err, w)
		}
	}
	if strings.Contains(err.Error(), "label gone") {
		t.Fatalf("error %q reports the later label failure; want the first error", err)
	}
	if _, ok := f.find(http.MethodPost, "/rest/api/3/issue/PROJ-7/transitions"); ok {
		t.Fatal("executed a transition despite no match")
	}
	if _, ok := f.find(http.MethodPost, "/rest/api/3/issue/PROJ-7/comment"); !ok {
		t.Fatalf("comment was skipped after transition failure; calls = %v", f.calls())
	}
}

func TestJiraMarkReadyReportsCommentFailureWhenTransitionSucceeds(t *testing.T) {
	f, src := newFakeJira(t)
	f.on(http.MethodGet, "/rest/api/3/issue/PROJ-7/transitions", http.StatusOK, `{"transitions": [
		{"id": "21", "to": {"name": "In Review"}}
	]}`)
	f.on(http.MethodPost, "/rest/api/3/issue/PROJ-7/transitions", http.StatusNoContent, ``)
	f.on(http.MethodPost, "/rest/api/3/issue/PROJ-7/comment", http.StatusInternalServerError, `down`)

	err := src.MarkReady(context.Background(), Ticket{Identifier: "PROJ-7"}, ReadyInfo{})
	if err == nil || !strings.Contains(err.Error(), "jira add comment PROJ-7: HTTP 500") {
		t.Fatalf("err = %v; want the comment failure", err)
	}
}

func TestJiraTransitionExecuteFailure(t *testing.T) {
	f, src := newFakeJira(t)
	f.on(http.MethodGet, "/rest/api/3/issue/PROJ-7/transitions", http.StatusOK, `{"transitions": [
		{"id": "21", "to": {"name": "In Review"}}
	]}`)
	f.on(http.MethodPost, "/rest/api/3/issue/PROJ-7/transitions", http.StatusConflict, `workflow says no`)

	err := src.transitionTo(context.Background(), "PROJ-7", "In Review")
	if err == nil || !strings.Contains(err.Error(), "jira execute transition PROJ-7: HTTP 409: workflow says no") {
		t.Fatalf("err = %v; want execute-transition failure", err)
	}
}

func TestJiraRequestFailsWhenServerUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	src := NewJira(JiraConfig{BaseURL: url})

	if _, err := src.Fetch(context.Background()); err == nil || !strings.HasPrefix(err.Error(), "jira search:") {
		t.Fatalf("err = %v; want a wrapped jira search transport error", err)
	}
}

func TestJiraRemovePlanLabelIsNoop(t *testing.T) {
	f, src := newFakeJira(t)
	if err := src.RemovePlanLabel(context.Background(), Ticket{Identifier: "PROJ-7"}); err != nil {
		t.Fatalf("RemovePlanLabel: %v", err)
	}
	if got := f.calls(); len(got) != 0 {
		t.Fatalf("calls = %v; want none", got)
	}
}

func containsAny(items []any, want string) bool {
	for _, it := range items {
		if it == want {
			return true
		}
	}
	return false
}
