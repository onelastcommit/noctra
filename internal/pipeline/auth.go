package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/onelastcommit/noctra/internal/agent"
	"github.com/onelastcommit/noctra/internal/authcheck"
	"github.com/onelastcommit/noctra/internal/config"
	"github.com/onelastcommit/noctra/internal/ghauth"
	"github.com/onelastcommit/noctra/internal/github"
	"github.com/onelastcommit/noctra/internal/notify"
	"github.com/onelastcommit/noctra/internal/sweep"
)

func (p *Pipeline) authChecks() []authcheck.Check {
	var checks []authcheck.Check
	if sess := ghauth.Active(); sess != nil {
		svc := ghauth.NewService(sess.Instance.ServiceURL)
		checks = append(checks, authcheck.GitHubApp(authcheck.SessionMinter(sess), svc.Health, p.knownOwnerRepos))
	} else {
		checks = append(checks, authcheck.GitHubCLI(authcheck.DefaultExec))
	}
	checks = append(checks, authcheck.AgentCLI(p.agent.Name(), p.agent.Label(), authcheck.DefaultExec, os.Getenv))
	if p.linear != nil && p.linearConfigured() {
		checks = append(checks, authcheck.Linear(p.linear.Ping))
	}
	return checks
}

func (p *Pipeline) linearConfigured() bool {
	return p.cfg.LinearAPIKey != "" || p.cfg.LinearOAuthToken != "" || p.cfg.ActorAppConfigured()
}

func (p *Pipeline) knownOwnerRepos(ctx context.Context) []string {
	var repos []string
	for _, remote := range p.resolver.AllRepoRemotes(ctx) {
		if ownerRepo, err := github.ExtractOwnerRepo(remote); err == nil {
			repos = append(repos, ownerRepo)
		}
	}
	return repos
}

func authCheckSchedule(expr string) *sweep.CronSchedule {
	sched, err := sweep.ParseCron(expr)
	if err == nil {
		return sched
	}
	slog.Warn("invalid AUTH_CHECK_SCHEDULE; using the default", "schedule", expr, "default", config.DefaultAuthCheckSchedule, "err", err)
	sched, _ = sweep.ParseCron(config.DefaultAuthCheckSchedule)
	return sched
}

func (p *Pipeline) runAuthCheckLoop(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	sched := authCheckSchedule(p.cfg.AuthCheckSchedule)
	tracker := authcheck.NewTracker()
	checks := p.authChecks()
	for {
		next := sched.Next(time.Now())
		if next.IsZero() {
			slog.Warn("AUTH_CHECK_SCHEDULE never fires; auth check disabled", "schedule", p.cfg.AuthCheckSchedule)
			return
		}
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		results := authcheck.RunAll(ctx, checks)
		if ctx.Err() != nil {
			return
		}
		for _, r := range results {
			if !r.OK && !r.Skipped {
				slog.Warn("auth check failed", "service", r.Name, "detail", r.Detail, "fix", r.Fix)
			}
		}
		if msg := authReportMessage(tracker.Observe(results)); msg != "" {
			p.notifier.Send(ctx, msg)
		}
	}
}

func authReportMessage(rep authcheck.Report) string {
	if rep.Empty() {
		return ""
	}
	var b strings.Builder
	writeFailures := func(title string, results []authcheck.Result) {
		if len(results) == 0 {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(title)
		for _, r := range results {
			fmt.Fprintf(&b, "\n• *%s*: %s", notify.EscapeMarkdown(r.Name), notify.EscapeMarkdown(r.Detail))
			if r.Fix != "" {
				fmt.Fprintf(&b, "\n  Fix: %s", notify.EscapeMarkdown(r.Fix))
			}
		}
	}
	writeFailures("🔑 *Authentication failed* — runs that need it will fail until this is fixed", rep.Failing)
	writeFailures("🔑 *Still not authenticated*", rep.Reminders)
	if len(rep.Recovered) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		names := make([]string, 0, len(rep.Recovered))
		for _, r := range rep.Recovered {
			names = append(names, notify.EscapeMarkdown(r.Name))
		}
		b.WriteString("✅ *Authentication restored* — " + strings.Join(names, ", "))
	}
	return b.String()
}

type agentFailure struct {
	detail string
	auth   bool
}

func describeAgentFailure(b agent.Backend, output string, runErr error) agentFailure {
	if line := agent.AuthFailureLine(output); line != "" {
		return agentFailure{
			detail: fmt.Sprintf("%s is not authenticated — %s.\n%s", b.Label(), agent.LoginHint(b.Name()), line),
			auth:   true,
		}
	}
	return agentFailure{detail: agent.FailureDetail(output, runErr)}
}
