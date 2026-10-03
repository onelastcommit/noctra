package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/onelastcommit/noctra/internal/agent"
	"github.com/onelastcommit/noctra/internal/budget"
	"github.com/onelastcommit/noctra/internal/notify"
	"github.com/onelastcommit/noctra/internal/state"
)

func TestFinishRun_StampsStatusAndKeepsBase(t *testing.T) {
	p := &Pipeline{store: testStore(t)}
	started := time.Now().Add(-time.Minute).Truncate(time.Second)
	run := state.RunHistory{
		Identifier: "ENG-1", TicketID: "ENG-1", Repo: "app",
		AgentBackend: "claude", RunType: "ticket", StartedAt: started,
	}

	p.finishRun(run, "failed")
	run.PRURL = "https://github.com/o/app/pull/1"
	p.finishRun(run, "pr_opened")

	rows, err := p.store.ListRunHistory(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	byStatus := map[string]state.RunHistory{}
	for _, r := range rows {
		byStatus[r.Status] = r
	}
	failed, opened := byStatus["failed"], byStatus["pr_opened"]
	if failed.Identifier != "ENG-1" || failed.Repo != "app" || failed.RunType != "ticket" || failed.PRURL != "" {
		t.Errorf("failed row lost base fields: %+v", failed)
	}
	if opened.PRURL == "" {
		t.Errorf("pr_opened row should carry the PR URL: %+v", opened)
	}
	if failed.FinishedAt.Before(started) {
		t.Errorf("FinishedAt should be stamped at finish, got %v (started %v)", failed.FinishedAt, started)
	}
}

func TestChargeUsage_RecordsBudgetAndUsageEvent(t *testing.T) {
	p := &Pipeline{store: testStore(t), budget: budget.New(budget.Config{})}
	claude, _ := agent.New("claude")

	p.chargeUsage(agent.Usage{TotalTokens: 1200, CostUSD: 0.5}, "sweep", "SWEEP-app-lint", "", claude)

	if got := p.budget.Stats().DailyTokens; got != 1200 {
		t.Errorf("budget tokens = %d, want 1200", got)
	}
	events, err := p.store.ListUsageEvents(time.Now().Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Source != "sweep" || events[0].TotalTokens != 1200 {
		t.Errorf("usage events = %+v, want one sweep event of 1200 tokens", events)
	}
}

func TestPauseIfBudgetExceeded(t *testing.T) {
	p := &Pipeline{
		budget:   budget.New(budget.Config{MaxDailyTokens: 1000}),
		notifier: notify.NewMulti(nil, nil),
	}
	if p.pauseIfBudgetExceeded(context.Background()) {
		t.Fatal("under the cap should not pause")
	}
	p.budget.Record(1500, 0)
	if !p.pauseIfBudgetExceeded(context.Background()) {
		t.Fatal("over the cap should pause")
	}
}

func TestAgentFailureIcon(t *testing.T) {
	if got := (agentFailure{auth: true}).icon(); got != "🔑" {
		t.Errorf("auth icon = %q", got)
	}
	if got := (agentFailure{}).icon(); got != "❌" {
		t.Errorf("failure icon = %q", got)
	}
}
