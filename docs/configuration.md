# Configuring Noctra

`noctra setup` writes `.env` interactively. Re-running it is safe: it merges into the existing file and keeps hand-added keys. Every variable, with its default, is documented in [`.env.example`](../.env.example); this page covers what each feature does.

Config lives in `~/.noctra/` (`.env`, `logs/`, `state.db`). If the current directory contains `.env`, `.env.example` or `go.mod`, that directory is used instead.

```bash
noctra config path              # resolved .env path
noctra config edit              # open it in $EDITOR
noctra config get KEY
noctra config set KEY=VALUE     # atomic write, keeps comments and other keys
```

## Picking up work

| Variable | Default | |
|----------|---------|---|
| `TICKET_SOURCES` | `linear` | `linear`, `github` (Issues), `jira`, or a comma-separated mix |
| `LINEAR_TEAM_KEY` | `ENG` | Prefix before ticket numbers |
| `TRIGGER_MODE` | `state` | `state` watches a column; `label` watches a label in any column |
| `TRIGGER_STATE` / `TRIGGER_LABEL` | `Next` / — | What to watch. The label is removed after dispatch |
| `IN_REVIEW_STATE` | `In Review` | Set once the PR exists |
| `MAX_CONCURRENT` | `3` | Tickets in flight at once |
| `AGENT_BACKEND` | `claude` | `claude`, `codex`, `copilot` or `antigravity` |

State and label names are case-sensitive. Repos are routed by each Linear project's `Repo:` directive (see the [README](../README.md#map-projects-to-repos)), with `REPO_PATH` as a single-repo fallback. GitHub Issues use their own repo unless the issue body has a `Repo:` line; Jira reads `Repo:` from the issue description.

To post on Linear as an app rather than as you, set `LINEAR_OAUTH_CLIENT_ID` and `LINEAR_OAUTH_CLIENT_SECRET`; Noctra mints and renews the token itself and falls back to `LINEAR_API_KEY` if it fails.

## Cost and safety caps

| Variable | Default | |
|----------|---------|---|
| `MAX_DAILY_TOKENS` / `MAX_DAILY_USD` | `0` (off) | Pause new dispatches for the rest of the UTC day once reached |
| `MAX_DISPATCHES` | `40` | Dispatches per UTC day |
| `AGENT_TIMEOUT_MINUTES` | `45` | Wall-clock limit per run |
| `AGENT_MAX_TOKENS` | `0` (off) | Abort a single Claude run past this many tokens |
| `MAX_RETRIES` | `3` | Attempts per ticket |

## Quality knobs

| Knob | Off (default) | On |
|------|---------------|-----|
| `USE_AGENT_TEAMS` *(Claude only)* | One agent per ticket — cheap, runs anywhere | A lead agent delegates implementation, tests and review to teammates in parallel |
| `GEMINI_API_KEY` | No external review | Gemini reviews the diff before the PR opens |

A second model has different blind spots. With the review gate on, Noctra sends the diff and ticket to Gemini, posts its findings as inline PR comments, and gives the agent `MAX_REVIEW_RETRIES` fix passes. If it still fails, the PR opens anyway with the verdict in the body. Expect roughly $0.01–$0.05 per ticket with `gemini-2.5-pro`.

## Auto-iterate on PR feedback

```env
AUTO_ITERATE_PRS=true
MAX_PR_ITERATIONS=3      # re-engagements per PR, review and CI combined
PR_POLL_INTERVAL=120
TRUSTED_REVIEWERS=       # bot logins to act on; empty = humans only
```

Noctra watches the PRs it opened. When review feedback lands (conversation comments, reviews, inline threads) or CI fails on the head commit, it re-runs the agent on the same branch and pushes a follow-up commit. It replies to each review thread, and resolves only the ones it addressed. Bot reviews are ignored unless listed in `TRUSTED_REVIEWERS`. Progress survives restarts, and you get a ping on every re-engagement and when the cap is hit.

## Maintenance sweeps

```env
SWEEP_ENABLED=true
SWEEP_SCHEDULE=0 3 * * *   # cron; or SWEEP_INTERVAL in seconds
SWEEP_TASKS=               # empty = all
SWEEP_REPOS=               # empty = every cloned repo; owner/name@branch pins a base
```

Sweeps open small maintenance PRs on their own: `lint-cleanup`, `dead-code`, `deps-update`, `test-coverage`, `doc-drift`, `modernize` and `bug-scan`. Each task has a per-repo cooldown, runs under the same budget caps, and is limited to `SWEEP_TIMEOUT_MINUTES` (20). Sweep PRs carry a `maintenance` label. Trigger one now with `/sweep` on Telegram, `noctra sweep`, or the dashboard; `--force` ignores the cooldown.

## Notifications and Telegram control

Set `TELEGRAM_ENABLED=true` with a bot token and chat ID, and Noctra sends updates **and** takes commands from that chat:

| Command | |
|---------|---|
| `/status` | Active runs and session stats |
| `/tickets [project] [state]` · `/ticket ENG-42` · `/find <text>` | Browse Linear |
| `/start ENG-42` · `/move ENG-42 "In Review"` | Dispatch a ticket on the next poll; move a ticket |
| `/requeue ENG-42 [context]` | Retry a blocked or failed ticket, optionally with more context |
| `/kill ENG-42` · `/pause` · `/resume` | Stop a run; pause or resume new dispatches |
| `/sweep` · `/addrepo` | Trigger sweeps; add a repository step by step |

`/help` lists everything.

`SLACK_WEBHOOK_URL` and `DISCORD_WEBHOOK_URL` add one-way notifications; a non-empty URL turns each on. `VERBOSE_NOTIFICATIONS=true` also pings on every dispatch.

## Dashboard

Set `DASHBOARD_ADDR` (e.g. `:8080`) and `DASHBOARD_TOKEN`, then open `http://<host>:8080/?token=<DASHBOARD_TOKEN>` for live runs, history, token and cost charts, budget, and the sweep matrix. `DASHBOARD_ADMIN_TOKEN` (passed as `&admin_token=…`) unlocks kill, requeue, retry, pause and sweep controls. For a remote host, set `DASHBOARD_SSH=user@host` and run `make dashboard` to tunnel to it.
