package authcheck

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeExec map[string]struct {
	out string
	err error
}

func (f fakeExec) run(_ context.Context, name string, args ...string) (string, error) {
	r, ok := f[name+" "+strings.Join(args, " ")]
	if !ok {
		return "", errors.New("unexpected command")
	}
	return r.out, r.err
}

func noEnv(string) string { return "" }

func runOne(c Check) Result {
	return RunAll(context.Background(), []Check{c})[0]
}

func TestAgentCLI_Claude(t *testing.T) {
	cases := []struct {
		name   string
		out    string
		err    error
		wantOK bool
	}{
		{"logged in", `{"loggedIn": true, "authMethod": "claude.ai"}`, nil, true},
		{"logged out", `{"loggedIn": false}`, errors.New("exit status 1"), false},
		{"noise around json", "warning: update available\n{\"loggedIn\": true}\n", nil, true},
		{"command missing", "", errors.New("executable file not found"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ex := fakeExec{"claude auth status --json": {c.out, c.err}}
			r := runOne(AgentCLI("claude", "Claude Code", ex.run, noEnv))
			if r.OK != c.wantOK {
				t.Fatalf("OK = %v, want %v (%+v)", r.OK, c.wantOK, r)
			}
			if !r.OK && !strings.Contains(r.Fix, "claude auth login") {
				t.Errorf("Fix should name the login command, got %q", r.Fix)
			}
			if r.Name != "Claude Code" {
				t.Errorf("Name = %q", r.Name)
			}
		})
	}
}

func TestAgentCLI_ClaudeAPIKeySkipsProbe(t *testing.T) {
	env := func(k string) string {
		if k == "ANTHROPIC_API_KEY" {
			return "sk-ant-x"
		}
		return ""
	}
	if r := runOne(AgentCLI("claude", "Claude Code", fakeExec{}.run, env)); !r.OK {
		t.Fatalf("API key should count as authenticated, got %+v", r)
	}
}

func TestAgentCLI_Codex(t *testing.T) {
	ok := fakeExec{"codex login status": {"Logged in using ChatGPT", nil}}
	if r := runOne(AgentCLI("codex", "Codex", ok.run, noEnv)); !r.OK {
		t.Fatalf("want OK, got %+v", r)
	}
	bad := fakeExec{"codex login status": {"Not logged in", errors.New("exit status 1")}}
	r := runOne(AgentCLI("codex", "Codex", bad.run, noEnv))
	if r.OK || r.Detail != "Not logged in" {
		t.Fatalf("want failure with CLI output as detail, got %+v", r)
	}
}

func TestAgentCLI_Copilot(t *testing.T) {
	classic := fakeExec{"gh auth token": {"ghp_abc\n", nil}}
	if r := runOne(AgentCLI("copilot", "Copilot", classic.run, noEnv)); r.OK {
		t.Fatalf("classic PAT should fail, got %+v", r)
	}
	oauth := fakeExec{"gh auth token": {"gho_abc\n", nil}}
	if r := runOne(AgentCLI("copilot", "Copilot", oauth.run, noEnv)); !r.OK {
		t.Fatalf("OAuth token should pass, got %+v", r)
	}
}

func TestAgentCLI_AntigravityIsSkipped(t *testing.T) {
	if r := runOne(AgentCLI("antigravity", "Antigravity", fakeExec{}.run, noEnv)); !r.Skipped {
		t.Fatalf("want Skipped, got %+v", r)
	}
}

func TestGitHubCLI(t *testing.T) {
	bad := fakeExec{"gh auth status": {"github.com\n  X Failed to log in to github.com using token (GH_TOKEN)\n", errors.New("exit status 1")}}
	r := runOne(GitHubCLI(bad.run))
	if r.OK || !strings.Contains(r.Detail, "Failed to log in") {
		t.Fatalf("want failure carrying gh's message, got %+v", r)
	}
}

func TestGitHubApp(t *testing.T) {
	repos := func(context.Context) []string { return []string{"o/r"} }
	none := func(context.Context) []string { return nil }
	healthy := func(context.Context) error { return nil }
	minted := func(context.Context, string) error { return nil }
	revoked := func(context.Context, string) error { return errors.New("instance not linked") }

	if r := runOne(GitHubApp(minted, healthy, repos)); !r.OK {
		t.Fatalf("want OK, got %+v", r)
	}
	if r := runOne(GitHubApp(revoked, healthy, repos)); r.OK || !strings.Contains(r.Fix, "github login") {
		t.Fatalf("revoked link should fail with a login fix, got %+v", r)
	}
	if r := runOne(GitHubApp(revoked, healthy, none)); !r.OK {
		t.Fatalf("with no repos only service health is checked, got %+v", r)
	}
}

func TestLinear(t *testing.T) {
	r := runOne(Linear(func(context.Context) (string, error) { return "", errors.New("401 authentication required") }))
	if r.OK || !strings.Contains(r.Detail, "401") {
		t.Fatalf("want failure, got %+v", r)
	}
}

func TestTracker(t *testing.T) {
	tr := NewTracker()
	bad := []Result{{Name: "Claude Code", Detail: "not logged in"}, {Name: "Linear", OK: true}}

	if rep := tr.Observe(bad); len(rep.Failing) != 1 || rep.Failing[0].Name != "Claude Code" {
		t.Fatalf("first failure should alert, got %+v", rep)
	}
	if rep := tr.Observe(bad); len(rep.Reminders) != 1 || len(rep.Failing) != 0 {
		t.Fatalf("each later check while broken should remind, got %+v", rep)
	}
	good := []Result{{Name: "Claude Code", OK: true}, {Name: "Linear", OK: true}}
	if rep := tr.Observe(good); len(rep.Recovered) != 1 {
		t.Fatalf("recovery should be announced, got %+v", rep)
	}
	if rep := tr.Observe(good); !rep.Empty() {
		t.Fatalf("steady healthy state should be quiet, got %+v", rep)
	}
}

func TestTracker_IgnoresSkipped(t *testing.T) {
	tr := NewTracker()
	if rep := tr.Observe([]Result{{Name: "Antigravity", Skipped: true}}); !rep.Empty() {
		t.Fatalf("skipped checks should never alert, got %+v", rep)
	}
}
