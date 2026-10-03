package agent

import (
	"strings"
	"testing"
)

func TestAuthFailureLine(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   string
	}{
		{"claude not logged in", "DEBUG: pwd = /x\nInvalid API key · Please run /login\n", "Invalid API key · Please run /login"},
		{"claude oauth expired", "API Error: 401 {\"type\":\"error\",\"error\":{\"type\":\"authentication_error\",\"message\":\"OAuth token has expired.\"}}", "API Error: 401 {\"type\":\"error\",\"error\":{\"type\":\"authentication_error\",\"message\":\"OAuth token has expired.\"}}"},
		{"codex refresh failed", "Error: Your access token could not be refreshed. Please log in again.", "Error: Your access token could not be refreshed. Please log in again."},
		{"copilot no auth", "Error: No authentication information found.", "Error: No authentication information found."},
		{"gh bad credentials", "HTTP 401: Bad credentials (https://api.github.com/graphql)", "HTTP 401: Bad credentials (https://api.github.com/graphql)"},
		{"ordinary failure", "panic: runtime error: index out of range\nexit status 2", ""},
		{"line number 401 is not auth", "main.go:401: undefined: foo", ""},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := AuthFailureLine(c.output); got != c.want {
				t.Errorf("AuthFailureLine() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestAuthFailureLine_IgnoresEarlyLines(t *testing.T) {
	early := "Please run /login\n" + strings.Repeat("compiling package\n", authScanLines+5)
	if got := AuthFailureLine(early); got != "" {
		t.Errorf("auth phrase far above the tail should be ignored, got %q", got)
	}
}

func TestLoginHint(t *testing.T) {
	for _, name := range []string{"claude", "codex", "copilot", "antigravity"} {
		if LoginHint(name) == "" {
			t.Errorf("LoginHint(%q) is empty", name)
		}
	}
	if !strings.Contains(LoginHint("claude"), "claude auth login") {
		t.Errorf("claude hint should name the login command, got %q", LoginHint("claude"))
	}
}
