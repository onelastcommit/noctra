package agent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/onelastcommit/noctra/internal/ghauth"
	"github.com/onelastcommit/noctra/internal/plugins"
)

var ErrTimedOut = errors.New("agent timed out")

var ErrTokenCapExceeded = errors.New("per-run token ceiling exceeded")

type RunOptions struct {
	Workdir       string
	Prompt        string
	LogFile       string
	Timeout       time.Duration
	UseAgentTeams bool
	MaxTokens     int64
	Env           []string
	PluginDirs    []string
}

func stageSkills(opts RunOptions) func() {
	cleanup, err := plugins.Stage(opts.Workdir, opts.PluginDirs)
	if err != nil {
		slog.Warn("could not stage every agent skill; running with what was staged", "workdir", opts.Workdir, "err", err)
	}
	return cleanup
}

func childEnv(env []string, opts RunOptions) []string {
	if len(opts.Env) == 0 {
		return env
	}
	if env == nil {
		env = os.Environ()
	}
	return ghauth.MergeEnv(env, opts.Env)
}

type Backend interface {
	Name() string
	Label() string
	CLI() string
	CoAuthor() string
	Run(ctx context.Context, opts RunOptions) (Usage, error)
	HasRateLimit(output string) bool
}

func New(name string) (Backend, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "claude":
		return claudeBackend{}, nil
	case "codex":
		return codexBackend{}, nil
	case "copilot":
		return copilotBackend{}, nil
	case "antigravity":
		return antigravityBackend{}, nil
	default:
		return nil, fmt.Errorf("unknown agent backend %q (want \"claude\", \"codex\", \"copilot\", or \"antigravity\")", name)
	}
}

func runCLI(ctx context.Context, bin string, args, env []string, opts RunOptions) (string, error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	logF, err := os.OpenFile(opts.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return "", fmt.Errorf("open log: %w", err)
	}
	defer logF.Close()

	branch, _ := currentBranch(ctx, opts.Workdir)
	fmt.Fprintf(logF, "DEBUG: pwd = %s\nDEBUG: branch = %s\n", opts.Workdir, branch)

	var (
		combined lockedBuffer
		stdout   bytes.Buffer
	)
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = opts.Workdir
	cmd.Stdout = io.MultiWriter(&stdout, &combined)
	cmd.Stderr = io.MultiWriter(logF, &combined)
	if env := childEnv(env, opts); env != nil {
		cmd.Env = env
	}

	err = cmd.Run()
	writeResponseBlock(logF, stdout.String())
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return combined.String(), fmt.Errorf("%w: %w", ErrTimedOut, err)
		}
		return combined.String(), err
	}
	return combined.String(), nil
}

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func runCLICapture(ctx context.Context, bin string, args, env []string, opts RunOptions) (stdout, stderr string, err error) {
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}

	var outBuf, errBuf bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = opts.Workdir
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if env := childEnv(env, opts); env != nil {
		cmd.Env = env
	}

	runErr := cmd.Run()
	if runErr != nil && ctx.Err() == context.DeadlineExceeded {
		runErr = fmt.Errorf("%w: %w", ErrTimedOut, runErr)
	}
	return outBuf.String(), errBuf.String(), runErr
}

func writeRunLog(ctx context.Context, opts RunOptions, diagnostics, response string) {
	logF, err := os.OpenFile(opts.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer logF.Close()
	branch, _ := currentBranch(ctx, opts.Workdir)
	fmt.Fprintf(logF, "DEBUG: pwd = %s\nDEBUG: branch = %s\n", opts.Workdir, branch)
	if d := strings.TrimSpace(diagnostics); d != "" {
		fmt.Fprintln(logF, d)
	}
	writeResponseBlock(logF, response)
}

func writeResponseBlock(w io.Writer, response string) {
	fmt.Fprintln(w, ResponseHeader)
	if r := strings.TrimRight(response, "\n"); r != "" {
		fmt.Fprintln(w, r)
	}
}

func currentBranch(ctx context.Context, workdir string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "branch", "--show-current")
	cmd.Dir = workdir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(trimNL(out)), nil
}

func trimNL(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r') {
		b = b[:len(b)-1]
	}
	return b
}
