package github

import (
	"errors"
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
