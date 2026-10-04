# Configuring Noctra

`noctra setup` writes `.env` interactively. It saves only the settings that differ from the defaults, and removes a line when you set it back to its default. The one exception: if your shell exports a different value for that setting, setup keeps the line so your choice still wins. Re-running it is safe: it merges into the existing file and keeps hand-added keys. Every variable, with its default, is documented in [`.env.example`](../.env.example); this page covers what each feature does.

Config lives in `~/.noctra/` (`.env`, `logs/`, `state.db`). If the current directory contains `.env`, `.env.example` or `go.mod`, that directory is used instead.

```bash
noctra config path              # resolved .env path
noctra config edit              # open it in $EDITOR
noctra config get KEY           # falls back to the default when unset (except paths, which depend on your machine)
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

## Agent plugins

```env
AGENT_PLUGIN_PACKS=engineering,frontend,security
AGENT_PLUGINS_EXTRA=          # owner/repo@<commit SHA>, at your own risk
```

The wizard asks what you mostly build, then which optional packs to add. Every agent run then gets those skills, whichever backend you use:

| Pack | Skills |
|------|--------|
| `engineering` (always on with any pack) | superpowers' TDD, systematic debugging, verification-before-completion and receiving-code-review; agent-skills' code simplification; ponytail |
| `frontend` | impeccable, taste-skill, GSAP (core, timeline, ScrollTrigger, React, performance), Vercel's React best practices, Anthropic's webapp-testing |
| `backend` | agent-skills' API and interface design; Matt Pocock's codebase design |
| `security` (optional) | Trail of Bits' differential review and sharp edges |
| `content` (optional) | Corey Haines' copywriting; humanizer |

Each plugin is pinned to an exact upstream commit. `noctra setup` downloads your packs into `~/.noctra/plugins` as soon as you choose them, and every start re-checks them, so editing `.env` by hand works too. A failed download never blocks a ticket. Only skills are loaded. Hooks, commands and anything else in a plugin are left out, and so are skills that wait for a human (brainstorming, planning). The startup banner and `noctra doctor` show what is active.

- **webapp-testing** lets the agent check a UI change in a real browser during the run. It needs Python Playwright with Chromium (`pip install playwright && python3 -m playwright install --with-deps chromium`). Without them that one skill is left out with a warning, and everything else still loads.
- **humanizer** helps with user-facing copy the ticket asks for. It does not shape Noctra's own output: commit messages and the PR title, framing and footer are written by Noctra itself, and only the PR's "What was implemented" summary comes from the agent.

**Licences.** Skills are fetched from their upstream repositories on your own machine; Noctra never redistributes them. Each plugin keeps its upstream licence file, and the copies placed in a worktree for a run are git-excluded, so they can't end up in a PR.

## Cost and safety caps

| Variable | Default | |
|----------|---------|---|
| `MAX_DAILY_TOKENS` / `MAX_DAILY_USD` | `0` (off) | Pause new dispatches for the rest of the UTC day once reached |
| `MAX_DISPATCHES` | `40` | Dispatches per UTC day |
| `AGENT_TIMEOUT_MINUTES` | `45` | Wall-clock limit per run |
| `AGENT_MAX_TOKENS` | `0` (off) | Abort a single Claude run past this many tokens |
| `MAX_RETRIES` | `3` | Attempts per ticket |

The daily caps are worth setting on a subscription login too: sweeps and Agent Teams can use a plan heavily, and that usage counts against your plan's limits.

## Quality knobs

| Knob | Off (default) | On |
|------|---------------|-----|
| `USE_AGENT_TEAMS` *(Claude only)* | One agent per ticket — cheap, runs anywhere | A lead agent delegates implementation, tests and review to teammates in parallel |
| `GEMINI_API_KEY` | No external review | Gemini reviews the diff before the PR opens |

A second model has different blind spots. With the review gate on, Noctra sends the diff and ticket to Gemini, posts its findings as inline PR comments, and gives the agent `MAX_REVIEW_RETRIES` fix passes. If it still fails, the PR opens anyway with the verdict in the body. Expect roughly $0.01–$0.05 per ticket with `gemini-2.5-pro`.

### Language

`ENGLISH_VARIANT` sets the spelling for everything the agent and the Gemini reviewer write: code comments, docs, commit messages, PR text and review replies. Use `british` (the default, e.g. "colour", "behaviour") or `american` (e.g. "color", "behavior"). Spellings fixed by a language or API, such as CSS properties and library identifiers, are left alone, and new names follow the codebase's existing convention.

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

## Credential health check

Once a day at noon (`AUTH_CHECK_SCHEDULE`, a cron expression in the host's local time, default `0 12 * * *`; `off` disables) Noctra checks that GitHub, Linear and the agent CLI are still logged in, without spending tokens. While a login is broken that check sends one 🔑 message naming the service and the command to fix it; the first check after it is fixed sends a ✅. A healthy check sends nothing. Antigravity has no status command, so its login is only reported when a run fails.

## Dashboard

Set `DASHBOARD_ADDR` (e.g. `:8080`) and `DASHBOARD_TOKEN`, then open `http://<host>:8080/?token=<DASHBOARD_TOKEN>` for live runs, history, token and cost charts, budget, and the sweep matrix. `DASHBOARD_ADMIN_TOKEN` (passed as `&admin_token=…`) unlocks kill, requeue, retry, pause and sweep controls. For a remote host, set `DASHBOARD_SSH=user@host` and run `make dashboard` to tunnel to it.
