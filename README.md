# 🌙 Noctra

> Move tickets to Next. Go to sleep. Wake up to PRs.

[![CI](https://github.com/onelastcommit/noctra/actions/workflows/ci.yml/badge.svg)](https://github.com/onelastcommit/noctra/actions/workflows/ci.yml)
[![Docker](https://github.com/onelastcommit/noctra/actions/workflows/docker.yml/badge.svg)](https://github.com/onelastcommit/noctra/actions/workflows/docker.yml)
[![Release](https://img.shields.io/github/v/release/onelastcommit/noctra?sort=semver)](https://github.com/onelastcommit/noctra/releases)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8.svg)](go.mod)
[![Website](https://img.shields.io/badge/website-getnoctra.dev-7C3AED.svg)](https://getnoctra.dev)

Part of [One Last Commit](https://github.com/onelastcommit): small ideas, taken further than strictly necessary.

Noctra picks up your Linear tickets, implements them with your coding agent of choice — **Claude Code, OpenAI Codex, GitHub Copilot or Google Antigravity** — and opens PRs while you sleep. It can keep iterating on review feedback and CI failures, and you can drive it from Telegram.

<!-- TODO(maintainer): drop a ~20s demo GIF here — drag a ticket to "Next" → PR appears — e.g. ![demo](docs/demo.gif) -->

## How it works

```
You: Move 3 tickets to "Next" → go to sleep

Noctra:
  1. Polls Linear for tickets in "Next" (or carrying a trigger label)
  2. Creates an isolated git worktree per ticket
  3. Runs your agent: it reads the ticket, plans, implements and self-reviews
  4. (Optional) Gemini reviews the diff; the agent gets a fix pass if it finds issues
  5. Pushes the branch and opens a PR
  6. Moves the ticket to "In Review" and links the PR on Linear
  7. (Optional) Keeps iterating on PR feedback and CI failures

You: Wake up → review 3 PRs → merge
```

It can also give the agent curated **plugins** (TDD, debugging, design craft…) on any backend, run scheduled **maintenance sweeps** (lint, dead code, dependency bumps, doc drift…), cap daily token spend, pull work from GitHub Issues or Jira, and serve a live dashboard. See [Configuration](docs/configuration.md).

## Quickstart

```bash
curl -fsSL https://raw.githubusercontent.com/onelastcommit/noctra/main/scripts/install.sh | sh

claude              # authenticate your agent once (or: codex login / gh auth login / agy)
gh auth login       # GitHub access for PRs
noctra setup        # interactive: backend, Linear key, options → writes .env
noctra doctor       # check everything is wired up
noctra              # start polling
```

Homebrew, `go install`, Docker, cloud and Raspberry Pi options are in [Deploying and operating](docs/deployment.md), along with running Noctra as a service.

### Map projects to repos

Tell each Linear **project** which repo its tickets belong to by adding a line to the project's description:

```
Repo: your-org/your-repo
Branch: main        (optional — defaults to the repo's default branch)
```

`Repo:` takes `owner/name` or any git URL (SSH, GitLab and other hosts work). Noctra clones on demand, so nothing needs to be set up in advance. Then drag a ticket into **Next**.

> ⚠️ **The one thing newcomers trip on:** if a ticket's project has no `Repo:` line, the agent has nowhere to work and the ticket bounces back. `REPO_PATH` in `.env` is a single-repo fallback.

## Requirements

- One agent CLI, already logged in with your subscription or an API key: [`claude`](https://docs.anthropic.com/en/docs/claude-code), [`codex`](https://github.com/openai/codex), [`copilot`](https://github.com/features/copilot) or [`agy`](https://antigravity.google). Noctra runs whichever one you've logged into on your machine and never stores agent credentials. Subscription usage counts against your plan's limits and terms; see [Agent backends](https://getnoctra.dev/docs#backends).
- `git` and an authenticated [`gh`](https://cli.github.com).
- A [Linear API key](https://linear.app/settings/api).
- Optional: a [Gemini API key](https://aistudio.google.com/apikey) for the review gate.

Noctra only makes outbound requests, so it needs no open port or webhook.

## Ticket flow

```
[Next] ──→ [In Progress] ──→ [In Review] ──→ [Done]
  ↑              │                              (you merge)
  └── blocked ←──┘
```

If the agent gets stuck it writes `BLOCKED: <reason>`. Noctra posts that on the ticket and moves it back to **Next**; add context and re-queue it (or `/requeue ENG-42 <context>` on Telegram).

## Writing good tickets

The agent needs to know *what* to change, *where*, and *how you'll know it's done*.

> **Good:** Login endpoint returns 500 when the refresh token is expired. Should return 401 and clear the session cookie. See `auth.controller.ts` line 42; tests in `auth.controller.spec.ts`. Acceptance: existing tests pass, a new test covers the expired-token case.
>
> **Bad:** Fix the auth bug

The [`writing-good-tickets`](.claude/skills/writing-good-tickets/SKILL.md) guide has a template.

## Security

Noctra runs the agent with no confirmation prompts (`--dangerously-skip-permissions` and equivalents), so it can read, write and execute anything in the repository. Use it on repos where you accept that, never on ones with committed secrets or write access to production. It opens PRs but never merges them, so your review is the safety gate.

By default it acts with your `gh` and git credentials. Linking the [GitHub App](docs/github-app.md) narrows that to single-repository tokens and gives the agent read-only access. This prevents accidental pushes but is not a sandbox: the agent runs as your OS user. For stronger isolation, [run it in Docker](docs/deployment.md#docker).

With the Gemini review gate on, diffs and ticket text are sent to Google's Gemini API.

## FAQ

**How much does it cost?** Noctra adds no cost of its own, but agent usage is billed or metered by your provider according to your plan: a subscription login counts against that plan's usage limits, and an API key is billed per token. The Gemini review gate costs roughly $0.01–$0.05 per ticket. `MAX_DAILY_TOKENS` / `MAX_DAILY_USD` cap usage per day, and are worth setting on a subscription too.

**Can a team share one instance?** Yes. If several people's tickets run through it, prefer an API key or a team or organisation plan over one person's individual subscription.

**Can it handle several repos?** Yes. Give each Linear project a `Repo:` line; tickets for different repos run concurrently up to `MAX_CONCURRENT`.

**A run crashed. How do I clean up?** `noctra cleanup` removes stale branches, worktrees and old logs (`--force` skips the prompts).

**Something isn't working.** Run `noctra doctor`, then see the [troubleshooting guide](.claude/skills/troubleshooting/SKILL.md).

## Documentation

- [Configuration](docs/configuration.md) — triggers, caps, review gate, auto-iterate, sweeps, Telegram, dashboard
- [Deploying and operating](docs/deployment.md) — install options, Docker, cloud, Pi, systemd, upgrades
- [GitHub identity](docs/github-app.md) — acting as `noctra-agent[bot]`
- [`.env.example`](.env.example) — every setting with its default

## Contributing

PRs welcome — bug fixes, deploy targets, or new agent, ticket-source or git backends. See [CONTRIBUTING.md](CONTRIBUTING.md) and the [Code of Conduct](CODE_OF_CONDUCT.md). Fun fact: Noctra implements many of its own tickets.

## Credits

Built on [Claude Code](https://docs.anthropic.com/en/docs/claude-code), [OpenAI Codex](https://github.com/openai/codex), [GitHub Copilot](https://github.com/features/copilot), [Google Antigravity](https://antigravity.google) and [Gemini](https://aistudio.google.com). Inspired by Damian Galarza's agent loop patterns and the wider agentic-coding community.

*MIT License — use freely, fork boldly, sleep soundly.*
