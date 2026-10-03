package pipeline

import (
	"errors"
	"strings"
	"testing"

	"github.com/onelastcommit/noctra/internal/agent"
	"github.com/onelastcommit/noctra/internal/authcheck"
)

func TestDescribeAgentFailure_AuthNamesTheFix(t *testing.T) {
	claude, err := agent.New("claude")
	if err != nil {
		t.Fatal(err)
	}
	output := "DEBUG: pwd = /wt\nDEBUG: branch = noctra/sweep-modernize\nInvalid API key · Please run /login\n"
	got := describeAgentFailure(claude, output, errors.New("exit status 1"))
	if !got.auth {
		t.Fatalf("want an auth failure, got %+v", got)
	}
	for _, want := range []string{"Claude Code is not authenticated", "claude auth login", "Please run /login"} {
		if !strings.Contains(got.detail, want) {
			t.Errorf("detail %q is missing %q", got.detail, want)
		}
	}
	if strings.Contains(got.detail, "exit status") {
		t.Errorf("detail should not fall back to the exit status, got %q", got.detail)
	}
}

func TestDescribeAgentFailure_NonAuthUsesCLIOutput(t *testing.T) {
	codex, err := agent.New("codex")
	if err != nil {
		t.Fatal(err)
	}
	got := describeAgentFailure(codex, "DEBUG: pwd = /wt\nError: model not found: gpt-9\n", errors.New("exit status 1"))
	if got.auth {
		t.Fatalf("not an auth failure, got %+v", got)
	}
	if got.detail != "Error: model not found: gpt-9" {
		t.Errorf("detail = %q, want the CLI's last line", got.detail)
	}
}

func TestDescribeAgentFailure_NoOutputFallsBackToError(t *testing.T) {
	claude, _ := agent.New("claude")
	got := describeAgentFailure(claude, "DEBUG: pwd = /wt\n", errors.New("exit status 1"))
	if got.detail != "exit status 1" {
		t.Errorf("detail = %q, want the error when the CLI printed nothing", got.detail)
	}
}

func TestAuthReportMessage(t *testing.T) {
	if msg := authReportMessage(authcheck.Report{}); msg != "" {
		t.Errorf("empty report should send nothing, got %q", msg)
	}

	msg := authReportMessage(authcheck.Report{
		Failing:   []authcheck.Result{{Name: "Claude Code", Detail: "not logged in", Fix: "run `claude auth login` on the host"}},
		Recovered: []authcheck.Result{{Name: "Linear", OK: true}},
	})
	for _, want := range []string{"Authentication failed", "Claude Code", "not logged in", "claude auth login", "Authentication restored", "Linear"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q is missing %q", msg, want)
		}
	}

	reminder := authReportMessage(authcheck.Report{Reminders: []authcheck.Result{{Name: "GitHub CLI", Detail: "token expired"}}})
	if !strings.Contains(reminder, "Still not authenticated") {
		t.Errorf("reminder should say it is still failing, got %q", reminder)
	}
}
