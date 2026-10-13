---
name: agent-backends
description: Use when adding or changing a coding-agent backend (AGENT_BACKEND claude/codex/copilot/antigravity) — CLI invocation flags, rate-limit detection, per-backend auth and environment, or the required-CLI checks.
---

# Coding-agent backends (`AGENT_BACKEND`)

The runner is pluggable behind `agent.Backend`: `claude` (default), `codex`, `copilot`, `antigravity`. `agent.New(name)` returns the implementation; the `Pipeline` holds the default instance, and a ticket's backend label (or, when iterating, the PR's cursor or label) can pick another per run (`resolveBackend`, `resolveIterateBackend`).

Almost everything in `internal/agent` is **shared**: the prompt builders (`BuildPrompt`, `BuildFixPrompt`), `BlockedLine` (keys off the `BLOCKED:` line our prompt asks for), the log_offset helpers and `ExtractSummary`. Put new agent logic there. Only two things belong in a backend file:

1. **Invocation** — `claudeStreamArgs` (`claude --print --output-format stream-json --verbose`), `codexArgs` (`codex exec --dangerously-bypass-approvals-and-sandbox <prompt>`), `copilotArgs` (`copilot --allow-all-tools --no-ask-user --output-format json -p <prompt>`), `antigravityArgs` (`agy --dangerously-skip-permissions --output-format stream-json --print <prompt>`). Codex, Copilot and Antigravity go through the shared `runCLI` (timeout → `ErrTimedOut`, DEBUG header, stderr streamed live, optional `streamParser` for stdout); Claude always uses `runStream`, which also enforces `AGENT_MAX_TOKENS` when set.

**Quick, tool-less runs.** `RunOptions.Model` and `RunOptions.NoTools` serve PR triage. `NoTools` swaps each CLI's auto-approval for a mode with no tools at all, because the triage prompt carries untrusted comment text. Each was verified against a "read this file" injection:
- Claude: `--tools "" --strict-mcp-config`. Without the latter, user MCP servers stay loaded (and a Haiku answer cost ~$0.19 instead of ~$0.002).
- Codex: `--ignore-user-config` (drops user MCP servers and plugins), `--disable` for its shell, exec, apps, plugins, browser, computer-use and image features (`codexToolFeatures`), `web_search="disabled"`, plus `--sandbox read-only --skip-git-repo-check`. A read-only sandbox alone still lets the shell read any file.
- Copilot: `--available-tools=noctra-no-tools`, an allow-list naming a tool that doesn't exist, plus `--disable-builtin-mcps`. Merely dropping `--allow-all-tools` still lets `view` read files and MCP tools run.
- Antigravity: no verified mode. Headless runs refuse commands and file reads, but `read_url_content` still fetches URLs, so `agent.TriageSupported` excludes it and its PRs engage directly.

`p.runAgent` skips plugins for `NoTools` runs.

**Log layout.** Each run's log entry ends with a `[noctra] agent response:` block (`agent.ResponseHeader`) holding only the agent's final answer; transcripts, stream events, stderr and abort notes go above it. When the agent omits the summary markers, `ExtractSummary` reads only the last response block, so nothing else can reach a PR description. With no `streamParser`, `runCLI` treats stdout as the answer (Codex prints only its final message there). Copilot and Antigravity emit JSON events, so their parsers pick the answer out (`phase: final_answer` / the `result` event) and keep only failure events as diagnostics.

**Tool lines.** Every tool call is logged live as one `[tool] <name>: <detail>` line (`agent.ToolLinePrefix`, `toolCallLine`), the detail being the command, path, pattern or URL, collapsed to one line and capped at 200 characters. Claude reads `tool_use` blocks, Copilot `tool.execution_start`, Antigravity tool `step_update`s; Codex's own transcript already shows each `exec`. `HasRateLimit` and `AuthFailureLine` skip tool lines (`withoutToolLines`), or a failed run whose agent ran `grep "rate limit"` would be treated as rate-limited.
2. **Rate-limit parsing** — `claudeRateLimitRe` / `codexRateLimitRe` / `copilotRateLimitRe` / `antigravityRateLimitRe`, since the CLIs phrase quota errors differently.

A new backend implements `Backend`, registers its name in `agent.New`, and adds its CLI to `config.RequiredCLIs` (the required set is `git` + `gh` + the selected agent CLI; `CheckCLIs`, `doctor` and the wizard surface it).

## Per-backend quirks

- **Antigravity:** unlike Claude's boolean `--print` (prompt passed via `-p`), `agy`'s `--print`/`--prompt`/`-p` is a **string flag whose value is the prompt**. The auto-approve flag must precede it and the prompt must be the very next token, or `agy` swallows the next flag as its prompt and improvises. Auth is a one-time `agy` login (Google AI Pro).
- **Codex:** one-time `codex login` on the host, or `OPENAI_API_KEY`.
- **Copilot:** headless (e.g. under systemd) the CLI does **not** read gh's credential store; it only checks `COPILOT_GITHUB_TOKEN`/`GH_TOKEN`/`GITHUB_TOKEN`. `copilotEnv` (`agent/copilot.go`) bridges this: unless `COPILOT_GITHUB_TOKEN` is already set, it takes `GH_TOKEN`/`GITHUB_TOKEN` or else mints one via `gh auth token`, and injects it as `COPILOT_GITHUB_TOKEN`. Copilot **rejects classic PATs** (`ghp_`), so the bridge skips them with a warning — gh must hold an OAuth token (`gho_`, from the `gh auth login` web flow) or a fine-grained PAT, or use `copilot /login`. Requires **Node 22+** (the Docker image ships Node 24). In GitHub App mode see the [`github-identity`](../github-identity/SKILL.md) skill for `COPILOT_GITHUB_TOKEN`.
- **Claude:** every run streams `--output-format stream-json`; see the [`sweeps`](../sweeps/SKILL.md) skill for the token cap and cost estimation on abort.

The model a run used is read back per backend for PR footers; the [`naming`](../naming/SKILL.md) skill covers where each backend's model comes from.

## Plugins

`AGENT_PLUGIN_PACKS` loads curated skills into every run (`internal/plugins`). `noctra setup` installs them right after writing `.env` (`setup.setUpPlugins`), `Pipeline.installPlugins` re-checks them at every start (instant when nothing changed), and `p.runAgent` puts their directories in `RunOptions.PluginDirs`, so a backend only decides how to deliver them:

- **Claude:** one `--plugin-dir <dir>` per plugin, before `-p`, in both `claudeArgs` and `claudeStreamArgs`. Skills appear namespaced as `noctra-<plugin>:<skill>`.
- **Codex, Copilot, Antigravity:** `defer stageSkills(opts)()` at the top of `Run` copies the skills into the worktree's `.agents/skills/` and removes them when the run ends.

A new backend must do one of the two; if it reads neither, its runs silently get no skills. Catalogue rules:

- The catalogue is data, not code: `internal/plugins/catalog.json`, embedded with `//go:embed`. Each plugin's repo, commit and licence appear once in its `plugins` map; packs refer to plugins by name. The private `onelastcommit/supervisor` repo bumps those commits with a reviewed PR, so keep that shape stable.
- Pin a full SHA and list skills explicitly. A skill at a repo's root sets `Name` and `Only` (humanizer).
- Take only self-contained skills: no `../` references, no `${CLAUDE_PLUGIN_ROOT}`, no instructions fetched from a moving branch at run time (Vercel's Web Design Guidelines was deferred for this).
- Leave out anything that waits for a human or downloads code at run time. impeccable ships without its `scripts/` launcher; its skill falls back to reading project files.
- A skill that drives a runtime declares `Requires` (webapp-testing needs Python Playwright with Chromium), so a host without it gets a warning instead of an agent installing it mid-run.
- Exclude files a coding agent could mistake for repository instructions, such as an `AGENTS.md` inside a skill folder.
- Packs that don't depend on the stack (`security`, `content`) are `Optional`; the wizard asks about them separately.
