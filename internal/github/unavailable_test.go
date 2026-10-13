package github

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func captureLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), &buf
}

func TestWarnRepoUnavailable_WarnsOncePerRepoWhenAppNotInstalled(t *testing.T) {
	logger, buf := captureLogger()
	err := fmt.Errorf("GitHub App token for Old/Repo: %w", ErrAppNotInstalled)

	WarnRepoUnavailable(logger, "github: skipping repo", "Old/Repo", err)
	WarnRepoUnavailable(logger, "github: skipping repo", "old/repo", err)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 log lines, got %d:\n%s", len(lines), buf.String())
	}
	if !strings.Contains(lines[0], "level=WARN") || !strings.Contains(lines[0], "Repo:") {
		t.Fatalf("first failure should warn with the remediation hint, got %s", lines[0])
	}
	if !strings.Contains(lines[1], "level=DEBUG") {
		t.Fatalf("a repeat for the same repo should drop to debug, got %s", lines[1])
	}
}

func TestWarnRepoUnavailable_OtherErrorsAlwaysWarn(t *testing.T) {
	logger, buf := captureLogger()
	err := errors.New("token service: unexpected_response")

	WarnRepoUnavailable(logger, "github: skipping repo", "other/flaky", err)
	WarnRepoUnavailable(logger, "github: skipping repo", "other/flaky", err)

	if got := strings.Count(buf.String(), "level=WARN"); got != 2 {
		t.Fatalf("transient failures must keep warning, got %d warnings:\n%s", got, buf.String())
	}
	if strings.Contains(buf.String(), "hint=") {
		t.Fatalf("the moved-repository hint belongs to not-installed failures only:\n%s", buf.String())
	}
}

func TestRepoFullName_SurfacesAFailedMint(t *testing.T) {
	SetTokenSource(func(context.Context, string) (string, error) {
		return "", fmt.Errorf("GitHub App token for old/repo: %w", ErrAppNotInstalled)
	})
	defer SetTokenSource(nil)
	if _, err := RepoFullName(context.Background(), "old/repo"); !errors.Is(err, ErrAppNotInstalled) {
		t.Fatalf("a failed mint must surface as ErrAppNotInstalled, got %v", err)
	}
}

func TestIsGitHubRemote(t *testing.T) {
	cases := map[string]bool{
		"https://github.com/o/r":                             true,
		"https://GitHub.com/o/r.git":                         true,
		"git@github.com:o/r.git":                             true,
		"o/r":                                                true,
		"git@gitlab.com:team/repo.git":                       false,
		"https://gitlab.com/team/repo":                       false,
		"https://github.example.com/o/r":                     false,
		"[https://github.com/o/r](<https://github.com/o/r>)": true,
	}
	for remote, want := range cases {
		if got := IsGitHubRemote(remote); got != want {
			t.Errorf("IsGitHubRemote(%q) = %v, want %v", remote, got, want)
		}
	}
}
