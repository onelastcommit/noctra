---
name: sweeps
description: Use when changing autonomous maintenance sweeps (SWEEP_ENABLED) — the scheduler, task catalog and prompts, cooldowns, per-run cost guards, aborted-run salvage, base-branch resolution, or the manual /sweep trigger.
---

# Autonomous maintenance sweeps

With `SWEEP_ENABLED=true` a **third** loop runs beside the Linear and PR-watcher loops, sharing the `WaitGroup` so shutdown drains in-flight tasks. Off by default.

```
sweep loop → scheduler.DueIn (cron SWEEP_SCHEDULE or fixed SWEEP_INTERVAL) → scheduler.Plan (SWEEP_REPOS or all cloned repos × task catalog)
  → filter by cooldown (per-repo, per-task, from state store)
  → pipeline.processSweepTask (bounded, shares the worker pool + active-set)
  → repo.CreateWorktreeWithBranch → agent.Run (task-specific prompt) → commit/push → gh pr create
```

## Tasks, cooldowns, PRs

- The catalog lives in `internal/sweep/task_*.go`; each file registers a task at init. Current tasks: `lint-cleanup` (weekly), `dead-code` (biweekly), `deps-update` (weekly), `test-coverage` (biweekly), `doc-drift` (biweekly), `modernize` (biweekly), `bug-scan` (biweekly, high-confidence defects only). `SWEEP_TASKS` scopes them.
- Each task has a per-repo **cooldown** in the state DB (`sweep_states`, `state.SweepState`).
- Branches are `noctra/sweep-<task>`, identifiers `SWEEP-<repo-slug>-<task>`; the [`naming`](../naming/SKILL.md) skill has why they differ. Sweep PR bodies carry `NoctraPRBodyMarker`, so auto-iterate claims them, and a `maintenance` label so humans can bulk-close them.
- Budget pause/exceeded skips sweeps, like ticket work.
- One cycle can land **two tasks on the same repo** (`scheduler.roundRobin` revisits repos on a second pass), and every worktree helper mutates the **shared clone** under its `.git/config` lock. `internal/repo` serialises them per clone via `lockRepo`; without it the loser fails with `could not lock config file .git/config` and its task is silently dropped. Keep any new clone-mutating `git` call inside that lock.

## Base branch

Precedence, resolved in `scheduler.repoTargets`/`Plan` into `Job.MainBranch` (which feeds the worktree base, the `branchAhead` compare and the `gh pr create` base together):

1. an `@branch` suffix on the `SWEEP_REPOS` entry (`scheduler.parseSweepRepoRef`);
2. the matching Linear project's `Branch:` directive (`linear.ListProjects` → `Project.RepoDirective`, matched by `github.ExtractOwnerRepo`; discovery-path repos are keyed off their `origin` via `repo.OriginRemoteOf`);
3. the repo's GitHub default branch (`origin/HEAD`);
4. `MAIN_BRANCH`.

## Per-run cost guards

The daily `MAX_DAILY_*` caps only gate *between* dispatches; a run in flight isn't stopped by them. Two per-run bounds close that gap:

1. `SWEEP_TIMEOUT_MINUTES` (default 20) caps wall-clock, shorter than the ticket `AGENT_TIMEOUT`.
2. `AGENT_MAX_TOKENS` (`config.AgentMaxTokens`, wired into every `agent.RunOptions.MaxTokens`) aborts *any* run once cumulative tokens cross it. Claude-only: `claudeBackend.runCapped` streams `--output-format stream-json`, sums per-turn usage, and cancels on breach (`agent.ErrTokenCapExceeded`); other backends fall back to the timeout. Sweeps get a `config.DefaultSweepMaxTokens` (2M) floor when it is unset — a productive sweep lands around 0.6M, so 2M means mis-scoped, not nearly done. (History: one dead-code run burned 18.4M tokens / $33 before any guard; an 8M floor was then burned whole by a bug-scan in 4.5 min for nothing.)

**The token ceiling is a guard, not a budget — raising it does not rescue a run.** Every aborted sweep consumed *exactly* its ceiling (8M → 8,097,478; 2M → ~2.04M ×3; 5M → ~5.05M ×3), while every run that produced something finished under 1M. A task that hits the ceiling is non-converging; fix the task prompt, not `AGENT_MAX_TOKENS`. `dead-code`, `test-coverage` and `deps-update` shared an unbounded verify loop over a slow JS suite (bump → full suite → revert → repeat), so their prompts now cap the work (≤20 removals, ≤5 bumps) and the verification (≤2–4 full-suite runs) and say to ship what is already green. `lint-cleanup` converged on the same repo `deps-update` died on twice, which ruled the repo out.

## Aborted runs

- **Accounted for.** `runCapped` learns true cost only from the terminal `result` event, which never arrives after `cancel()`. It accumulates per-turn usage plus the model name while streaming and, when `result` is missing, estimates from `agent.PricesForModel` (`internal/agent/pricing.go`). Before this, aborted runs recorded `$0.00` and ~$30 was invisible to `MAX_DAILY_USD`.
- **Recorded.** Both abort paths (`ErrTimedOut`, `ErrTokenCapExceeded`) go through `pipeline.abortSweepTask`: record usage, record the **cooldown**, write a `run_history` row with status **`aborted`** (not `failed` — the run was healthy, we killed it), notify. Recording the cooldown is deliberate: an aborting task aborts again identically, so skipping it would re-burn the ceiling every cycle. `/sweep --force` is the escape hatch.
- **Salvaged.** `pipeline.salvageAbortedWork` commits any worktree changes, pushes, and opens a **draft** PR `<prefix>: <description> (partial)` whose body opens by saying the diff is **UNVERIFIED** (the agent never reached its build/test step). It carries the marker and `maintenance` label, so auto-iterate drives a cheap CI fix instead of a fresh full scan. The `run_history` row stays `aborted` but carries the PR URL.
- **Announced.** Every terminal outcome notifies through `pipeline.notifySweepOutcome` with tokens and estimated cost: aborted, failed, blocked, no-change, PR opened. It sends on `context.WithoutCancel`, because `markDone` cancels the task context the moment `processSweepTask` returns.

## Manual trigger

`/sweep` (Telegram), `noctra sweep` (CLI) and `POST /api/sweep` (dashboard, admin token) all funnel into `Pipeline.TriggerSweep`, which hands a `sweep.PlanOptions` to the **existing** `runSweepLoop` over the buffered `sweepNow` channel. The loop dispatches through its own `WaitGroup`, active-set and `MAX_CONCURRENT` accounting.

- A manual trigger never plans and dispatches inline: a second dispatcher beside the loop would bypass the worker-pool cap and double-run tasks.
- A manual sweep skips `MarkSwept`, so it never shifts the scheduled cadence. `PlanOptions.IgnoreCooldown` (`--force`) is the only way to re-run inside a cooldown. Budget pause still refuses it.
- `noctra sweep` is a thin HTTP client over the dashboard admin API, not a second sweeper — two processes planning against one state DB would double-dispatch and race on cooldowns.

## Config

| Env var | Default | Description |
|---------|---------|-------------|
| `SWEEP_ENABLED` | `false` | Enable the sweep scheduler |
| `SWEEP_SCHEDULE` | (empty) | 5-field cron (`sweep.ParseCron`, zero-dep); empty = use `SWEEP_INTERVAL`; invalid → warn + fall back. Cron mode waits for the next match |
| `SWEEP_INTERVAL` | `86400` | Seconds between cycles; fires immediately on startup |
| `SWEEP_MAX_TASKS` | `5` | Max tasks per run |
| `SWEEP_TIMEOUT_MINUTES` | `20` | Per-task wall-clock cap |
| `SWEEP_TASKS` | (all) | Comma-separated task names |
| `SWEEP_REPOS` | (all cloned) | `owner/name` or git URLs, resolved via `repo.ResolveDirect` (clone-on-demand); **replaces** `AllRepoPaths()` discovery; unresolvable entries warn and skip. `@branch` suffix pins the base (also on full URLs and scp `git@host:owner/name@staging`) |
