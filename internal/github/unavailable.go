package github

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

var ErrAppNotInstalled = errors.New("app not installed")

const movedRepoHint = "if this repository was transferred or renamed, point its Linear project's Repo: line at the new name and delete the old clone shown by `noctra repos list`; repeats are logged at debug level until Noctra restarts"

var warnedUnavailable sync.Map

func WarnRepoUnavailable(logger *slog.Logger, msg, ownerRepo string, err error) {
	if !errors.Is(err, ErrAppNotInstalled) {
		logger.Warn(msg, "repo", ownerRepo, "err", err)
		return
	}
	if _, seen := warnedUnavailable.LoadOrStore(strings.ToLower(ownerRepo), true); seen {
		logger.Debug(msg, "repo", ownerRepo, "err", err)
		return
	}
	logger.Warn(msg, "repo", ownerRepo, "err", err, "hint", movedRepoHint)
}

var ErrRepoNotFound = errors.New("repository not found")

func RepoFullName(ctx context.Context, ownerRepo string) (string, error) {
	cmd, err := Command(ctx, ownerRepo, "api", "repos/"+ownerRepo, "--jq", ".full_name")
	if err != nil {
		return "", err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if strings.Contains(msg, "HTTP 404") {
			return "", fmt.Errorf("%w: %s", ErrRepoNotFound, msg)
		}
		return "", fmt.Errorf("gh api repos/%s: %w: %s", ownerRepo, err, msg)
	}
	return strings.TrimSpace(string(out)), nil
}
