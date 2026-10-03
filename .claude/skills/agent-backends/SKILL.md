---
name: agent-backends
description: Use when adding or changing a coding-agent backend (AGENT_BACKEND claude/codex/copilot/antigravity) — CLI invocation flags, rate-limit detection, per-backend auth and environment, or the required-CLI checks.
---

# Coding-agent backends (`AGENT_BACKEND`)

The runner is pluggable behind `agent.Backend`: `claude` (default), `codex`, `copilot`, `antigravity`. `agent.New(name)` returns the implementation; the `Pipeline` holds one instance and routes `Run` / `HasRateLimit` through it.

Almost everything in `internal/agent` is **shared**: the prompt builders (`BuildPrompt`, `BuildFixPrompt`), `BlockedLine` (keys off the `BLOCKED:` line our prompt asks for), the log_offset helpers and `ExtractSummary`. Put new agent logic there. Only two things belong in a backend file:

1. **Invocation** — `claudeArgs` (`claude --print`), `codexArgs` (`codex exec --dangerously-bypass-approvals-and-sandbox <prompt>`), `copilotArgs` (`copilot --allow-all-tools --no-ask-user -p <prompt>`), `antigravityArgs` (`agy --dangerously-skip-permissions --print <prompt>`). All go through the shared `runCLI` (timeout → `ErrTimedOut`, DEBUG header, log streaming).
2. **Rate-limit parsing** — `claudeRateLimitRe` / `codexRateLimitRe` / `copilotRateLimitRe` / `antigravityRateLimitRe`, since the CLIs phrase quota errors differently.

A new backend implements `Backend`, registers its name in `agent.New`, and adds its CLI to `config.RequiredCLIs` (the required set is `git` + `gh` + the selected agent CLI; `CheckCLIs`, `doctor` and the wizard surface it).

## Per-backend quirks

- **Antigravity:** unlike Claude's boolean `--print` (prompt passed via `-p`), `agy`'s `--print`/`--prompt`/`-p` is a **string flag whose value is the prompt**. The auto-approve flag must precede it and the prompt must be the very next token, or `agy` swallows the next flag as its prompt and improvises. Auth is a one-time `agy` login (Google AI Pro).
- **Codex:** one-time `codex login` on the host, or `OPENAI_API_KEY`.
- **Copilot:** headless (e.g. under systemd) the CLI does **not** read gh's credential store; it only checks `COPILOT_GITHUB_TOKEN`/`GH_TOKEN`/`GITHUB_TOKEN`. `copilotEnv` (`agent/copilot.go`) bridges this: with no token env set it mints one via `gh auth token` and injects `GH_TOKEN`. Copilot **rejects classic PATs** (`ghp_`), so the bridge skips them with a warning — gh must hold an OAuth token (`gho_`, from the `gh auth login` web flow) or a fine-grained PAT, or use `copilot /login`. Requires **Node 22+** (the Docker image ships Node 24). In GitHub App mode see the [`github-identity`](../github-identity/SKILL.md) skill for `COPILOT_GITHUB_TOKEN`.
- **Claude:** token-capped runs stream `--output-format stream-json`; see the [`sweeps`](../sweeps/SKILL.md) skill for `runCapped` and cost estimation on abort.

The model a run used is read back per backend for PR footers; the [`naming`](../naming/SKILL.md) skill covers where each backend's model comes from.
