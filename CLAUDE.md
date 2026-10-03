# Noctra

> `AGENTS.md` at the repo root is a symlink to this file, so the Codex backend (`AGENT_BACKEND=codex`) reads the same project guidance Claude does. Edit `CLAUDE.md`; `AGENTS.md` follows.

Autonomous Linear-to-PR agent in Go. Polls Linear (or GitHub Issues / Jira) for tickets in a trigger state or with a trigger label, dispatches a coding agent to implement them, creates PRs, and moves tickets to review. Up to three work loops share one `WaitGroup`, worker pool (`MAX_CONCURRENT`) and active-set, plus a credential check that only alerts:

```
ticket loop  → source.Fetch → pipeline.process (bounded goroutine)
  → repo.Resolve → repo.CreateOrResumeWorktree → agent.Run → check output
  → (optional) review.Gate → commit/push → gh pr create → source.MarkReady
PR loop      (AUTO_ITERATE_PRS) → watch.Scan → pipeline.iteratePR → push to the same branch
sweep loop   (SWEEP_ENABLED)    → scheduler.Plan → pipeline.processSweepTask → maintenance PR
auth loop    (AUTH_CHECK_SCHEDULE, daily at noon by default) → authcheck.RunAll → authcheck.Tracker → notify while broken / on recovery
```

- `TRIGGER_MODE=state` (default) polls the `TRIGGER_STATE` column. `TRIGGER_MODE=label` polls for `TRIGGER_LABEL` regardless of column and **removes** the label after dispatch so the ticket isn't re-polled; the trigger-state ID is then not resolved, but the in-review state still is.
- Worktrees live at `~/.noctra-worktrees/<IDENTIFIER>` so tickets run concurrently.

## Guardrails

Each of these has caused a real incident; a plausible-looking patch breaks them easily.

- ⚠️ **The `noctra/` branch prefix is reserved for branches Noctra creates — applies to humans and any agent working here.** For hand-written work use `fix/`, `feat/`, `docs/`, `chore/` or `eng-<n>-<slug>`. In personal-token mode a running instance claims any `noctra/*` PR authored by its own GitHub account (`ListNoctraPRs` filters `--author @me` plus the prefix), iterates on its review feedback and pushes commits that collide with yours (PR #143). App mode filters by `--author app/<slug>`, but the rule holds for anyone in token mode.
- **Zero comments in Go source** — see Code style.
- **log_offset:** agent logs (`logs/<IDENTIFIER>.log` under the config dir) append across attempts, each headed `--- Attempt <timestamp> ---`. `agent.OffsetBefore` records the size before a run and `agent.ReadAfter` reads only the new tail; `BlockedLine` and `HasRateLimit` scan that tail. Scanning the whole file re-detects failures from earlier attempts.
- **Every `gh` call goes through `github.Command` / `CommandInDir`**, never a raw `exec.CommandContext(ctx, "gh", …)`, or app mode silently falls back to personal credentials ([`github-identity`](.claude/skills/github-identity/SKILL.md)).
- **Fresh ticket dispatch uses `repo.CreateOrResumeWorktree`**, not `CreateWorktree`. A run that pushed but died before `gh pr create` leaves an orphaned remote branch; re-branching from `origin/<main>` builds a non-descendant that origin rejects, so the ticket burns a full run per retry until `MAX_RETRIES`.
- **Any `git` call that mutates a shared clone runs inside `repo.lockRepo`.** Two tasks on one clone otherwise race on `.git/config` and one is silently dropped.
- **Names and hidden markers are read back** by the PR watcher and lessons extractor. Change branch names, identifiers, commit/PR wording, footers or markers only through the [`naming`](.claude/skills/naming/SKILL.md) skill.
- **Manual sweeps go through `Pipeline.TriggerSweep`** into the existing sweep loop; a second dispatcher would bypass the worker-pool cap ([`sweeps`](.claude/skills/sweeps/SKILL.md)).
- **After changing `internal/dashboard/web/`, run `yarn build` and commit `static/`**, and keep the bundle a single inlined file ([`dashboard`](.claude/skills/dashboard/SKILL.md)).
- **Report an agent failure through `pipeline.describeAgentFailure`**, not `runErr.Error()`. The exec error is just `exit status 1`; the CLI's own reason (and an expired login, via `agent.AuthFailureLine` + `agent.LoginHint`) is in the log tail.
- **Keep new operationally significant config visible in the startup banner** (`pipeline.banner`).

## Multi-repo routing

The target repo is chosen per ticket; there is no repo registry.

1. **Linear project directive** — a `Repo: <owner/name | git URL>` line (optionally `Branch: <name>`) in the project's `content`, falling back to `description` (`linear.Project.RepoDirective`). `repo.ResolveDirect` clones it on demand into `~/.noctra-repos/<slug>` (lock-guarded via `mkdir(2)`). `owner/name` expands to GitHub HTTPS; full `https://`/`git@` URLs are used verbatim. With no `Branch:`, the default branch comes from `origin/HEAD` (fallback `MAIN_BRANCH`).
2. **`REPO_PATH`** — single-repo fallback (`repo.Resolve`); otherwise the ticket is skipped with a Linear comment.

Auto-iterate resolves the same way from the PR URL (`prRepoOwnerRepo` → `ResolveDirect`). The set of known repos is whatever has been cloned: `repo.Resolver.AllRepoPaths` scans `ReposBase` (plus `REPO_PATH`), and `AllRepoRemotes` feeds the PR watcher, re-read on every scan.

## Config directory

Config defaults to `~/.noctra/` (`.env`, `logs/`, `state.db`). If the current directory contains `.env`, `.env.example` or `go.mod`, Noctra uses cwd instead, so `go run` works without touching `~/.noctra/` (`resolveScriptDir()` in `cmd/noctra/main.go`; `config.DefaultConfigDir()`). Every variable is documented in `.env.example`.

## Code style

### ⚠️ Zero comments

**This codebase contains no comments. Do not add any.** This applies to humans and every agent, and covers doc comments on exported symbols, package docs, `TODO`/`FIXME`, section banners and end-of-line asides. The urge to explain is a signal to rename a symbol or extract a function.

The only permitted `//` lines are directives: `//go:embed`, `//go:build`, `//go:generate`, `//nolint:…`, `// Code generated … DO NOT EDIT.` Enforced in CI by `make check-comments` (`scripts/check-comments.sh`).

The *why* lives in this file for cross-cutting behaviour, in the subsystem skills below, and in [`architecture`](.claude/skills/architecture/SKILL.md) Invariant 7 for file-level facts (lock ordering, cursor semantics, flag quirks, encoding traps). When a change's reasoning isn't evident from the diff, add a row there — not in the source.

## Skills

Read `.claude/skills/<name>/SKILL.md` before working in its area. Codex and Copilot don't auto-discover skills but can open these paths.

| Skill | When to use |
|-------|-------------|
| [`architecture`](.claude/skills/architecture/SKILL.md) | Modifying Noctra's source — package map, invariants, testability conventions |
| [`auto-iterate`](.claude/skills/auto-iterate/SKILL.md) | The PR watcher: claiming PRs, feedback and CI triggers, cursors, per-finding replies |
| [`sweeps`](.claude/skills/sweeps/SKILL.md) | Maintenance sweeps: scheduler, tasks, cost guards, aborted-run salvage, manual trigger |
| [`github-identity`](.claude/skills/github-identity/SKILL.md) | `GITHUB_AUTH_MODE`, the GitHub App, token scopes, git credential helper, agent credentials |
| [`agent-backends`](.claude/skills/agent-backends/SKILL.md) | Adding or changing a coding-agent backend: invocation, rate limits, auth quirks |
| [`dashboard`](.claude/skills/dashboard/SKILL.md) | The Preact dashboard: building, committing `static/`, dev server, auth |
| [`naming`](.claude/skills/naming/SKILL.md) | Branch names, identifiers, commit messages, PR footers, model label, hidden markers |
| [`build-and-release`](.claude/skills/build-and-release/SKILL.md) | Builds, Pi cross-compile, Docker image, systemd upgrade, GoReleaser, cutting releases |
| [`setup-and-config`](.claude/skills/setup-and-config/SKILL.md) | Installing, the setup wizard, `.env`, the `Repo:` directive |
| [`troubleshooting`](.claude/skills/troubleshooting/SKILL.md) | Tickets not picked up, agent errors, PR creation, auto-iterate not reacting |
| [`writing-good-tickets`](.claude/skills/writing-good-tickets/SKILL.md) | Drafting Linear tickets Noctra can implement autonomously |

## Build and test

```bash
go build -o noctra ./cmd/noctra
go test ./...
go vet ./...          # must be clean
make check-comments
```
