package authcheck

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/onelastcommit/noctra/internal/ghauth"
)

type Result struct {
	Name    string
	OK      bool
	Skipped bool
	Detail  string
	Fix     string
}

type Check struct {
	Name string
	Run  func(ctx context.Context) Result
}

type Exec func(ctx context.Context, name string, args ...string) (string, error)

func DefaultExec(ctx context.Context, name string, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}

const probeTimeout = 30 * time.Second

func RunAll(ctx context.Context, checks []Check) []Result {
	results := make([]Result, 0, len(checks))
	for _, c := range checks {
		probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
		r := c.Run(probeCtx)
		cancel()
		r.Name = c.Name
		results = append(results, r)
	}
	return results
}

func GitHubCLI(run Exec) Check {
	return Check{Name: "GitHub CLI", Run: func(ctx context.Context) Result {
		out, err := run(ctx, "gh", "auth", "status")
		if err != nil {
			return Result{Detail: firstLine(out, err), Fix: "run `gh auth login` on the host"}
		}
		return Result{OK: true}
	}}
}

type TokenMinter func(ctx context.Context, ownerRepo string) error

func GitHubApp(mint TokenMinter, health func(ctx context.Context) error, repos func(ctx context.Context) []string) Check {
	return Check{Name: "GitHub App", Run: func(ctx context.Context) Result {
		fix := "run `noctra github login --force` on the host"
		if list := repos(ctx); len(list) > 0 {
			if err := mint(ctx, list[0]); err != nil {
				return Result{Detail: "could not mint a token for " + list[0] + ": " + err.Error(), Fix: fix}
			}
			return Result{OK: true}
		}
		if err := health(ctx); err != nil {
			return Result{Detail: "token service unreachable: " + err.Error(), Fix: "check network access to NOCTRA_AUTH_URL"}
		}
		return Result{OK: true}
	}}
}

func SessionMinter(sess *ghauth.Session) TokenMinter {
	return func(ctx context.Context, ownerRepo string) error {
		_, err := sess.FreshToken(ctx, ownerRepo, ghauth.ScopeRead)
		return err
	}
}

func Linear(ping func(ctx context.Context) (string, error)) Check {
	return Check{Name: "Linear", Run: func(ctx context.Context) Result {
		if _, err := ping(ctx); err != nil {
			return Result{Detail: err.Error(), Fix: "renew the Linear API key or OAuth credentials in .env"}
		}
		return Result{OK: true}
	}}
}

func AgentCLI(backend, label string, run Exec, getenv func(string) string) Check {
	return Check{Name: label, Run: func(ctx context.Context) Result {
		switch backend {
		case "claude":
			return claudeStatus(ctx, run, getenv)
		case "codex":
			out, err := run(ctx, "codex", "login", "status")
			if err != nil {
				return Result{Detail: firstLine(out, err), Fix: "run `codex login` on the host (or set OPENAI_API_KEY)"}
			}
			return Result{OK: true}
		case "copilot":
			return copilotStatus(ctx, run, getenv)
		default:
			return Result{Skipped: true, Detail: "no non-interactive auth status command; failures surface when a run starts"}
		}
	}}
}

func claudeStatus(ctx context.Context, run Exec, getenv func(string) string) Result {
	if getenv("ANTHROPIC_API_KEY") != "" {
		return Result{OK: true}
	}
	fix := "run `claude auth login` on the host (or set ANTHROPIC_API_KEY)"
	out, err := run(ctx, "claude", "auth", "status", "--json")
	var status struct {
		LoggedIn bool `json:"loggedIn"`
	}
	if jsonErr := json.Unmarshal([]byte(jsonObject(out)), &status); jsonErr == nil {
		if !status.LoggedIn {
			return Result{Detail: "not logged in", Fix: fix}
		}
		return Result{OK: true}
	}
	if err != nil {
		return Result{Detail: firstLine(out, err), Fix: fix}
	}
	return Result{Detail: "unrecognised `claude auth status` output: " + firstLine(out, nil), Fix: fix}
}

func copilotStatus(ctx context.Context, run Exec, getenv func(string) string) Result {
	for _, key := range []string{"COPILOT_GITHUB_TOKEN", "GH_TOKEN", "GITHUB_TOKEN"} {
		if getenv(key) != "" {
			return Result{OK: true}
		}
	}
	fix := "run `gh auth login` (OAuth web flow) or set COPILOT_GITHUB_TOKEN"
	out, err := run(ctx, "gh", "auth", "token")
	if err != nil {
		return Result{Detail: "no token: " + firstLine(out, err), Fix: fix}
	}
	if strings.HasPrefix(strings.TrimSpace(out), "ghp_") {
		return Result{Detail: "gh is authenticated with a classic PAT, which Copilot rejects", Fix: fix}
	}
	return Result{OK: true}
}

func jsonObject(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end < start {
		return ""
	}
	return s[start : end+1]
}

var failureLineRe = regexp.MustCompile(`(?i)fail|error|not logged|invalid|expired|denied|unauthori[sz]ed|revoked`)

func firstLine(out string, err error) string {
	var first string
	for _, l := range strings.Split(out, "\n") {
		s := strings.TrimSpace(l)
		if s == "" {
			continue
		}
		if failureLineRe.MatchString(s) {
			return strings.TrimLeft(s, "X✗ ")
		}
		if first == "" {
			first = s
		}
	}
	if first != "" {
		return first
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Sprintf("exited with status %d and no output", exitErr.ExitCode())
		}
		return err.Error()
	}
	return "no output"
}
