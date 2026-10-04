---
name: architecture
description: Use when modifying Noctra's own source code to understand non-obvious invariants, package boundaries, testability conventions, and the patterns an agent must respect to avoid breaking subtle runtime behaviour.
---

# Architecture & Contributing

Use this playbook before changing Noctra's core code. The invariants below are easy to violate in a plausible-looking patch; each has caused a real bug or regression.

## Package map

| Package | Purpose |
|---------|---------|
| `cmd/noctra` | Entry point + subcommand dispatch (`run` / `setup` / `config` / `repos` / `sweep` / `github` / `git-credential` / `cleanup` / `doctor [--json]` / `update` / `install-service [--start/--force]` / `logs` / `tail` / `start` / `stop` / `restart` / `status` / `completion` / `version`). `start`…`status` are thin `systemctl --user <verb> noctra.service` wrappers; `completion bash\|zsh` is the pure `completionScript`; startup banner; `--help` |
| `internal/config` | `.env` parser, validated `Config`, `DefaultConfigDir` (`~/.noctra/`) |
| `internal/configcmd` | `noctra config path\|edit\|get\|set` — atomic, comment-preserving `.env` edits |
| `internal/source` | Ticket sources behind one interface: Linear, GitHub Issues (`TICKET_SOURCES`), Jira |
| `internal/linear` | Linear GraphQL client: `ResolveStateIDs`, `FetchTriggerIssues`, `FetchLabeledIssues` (both fetch issue `comments` → `Issue.ClarificationComments`, which filters Noctra's own notices by the `"**Noctra"` body prefix, and the project `description`/`content` → `Project.RepoDirective`), `ResolveLabelID`, `RemoveLabel`, `SetState`, `Comment`; Telegram read queries (`ProjectIssueCounts`, `ListProjectIssues`, `SearchIssues`, `GetIssueByIdentifier`); `ListProjects` (+ `id`/`slugId`/`url` so `MatchProjects` resolves a pasted link) and `UpdateProjectContent`/`UpsertRepoDirective`. Auth: personal API key (`New`, sent verbatim), static app-actor OAuth token (`NewOAuth`, `Bearer`, `LINEAR_OAUTH_TOKEN`), or — preferred — the self-renewing actor=app `TokenManager` (`oauth.go`: `client_credentials&actor=app` from `LINEAR_OAUTH_CLIENT_ID`/`SECRET`, or `refresh_token` when `LINEAR_OAUTH_REFRESH_TOKEN` is set, rotations persisted via `state.Store`). OAuth paths set `Client.FallbackAPIKey`, so an expired app token **degrades to the personal key** (with `OnDegrade`) instead of crash-looping; a partial actor=app config warns and falls back |
| `internal/linearclient` | Builds the authenticated `linear.Client` from config; one place for credential precedence |
| `internal/repo` | `ResolveDirect` (`Repo:` directive or a PR's own repo, `origin/HEAD` default-branch detection) + `Resolve` (`REPO_PATH` fallback); `AllRepoPaths`/`AllRepoRemotes` (scan `ReposBase`); clone-on-demand (`mkdir(2)` lock); `CreateWorktree` / `ResumeWorktree` / `CreateOrResumeWorktree` (picks on `RemoteBranchExists`); `lockRepo`; `BranchName` |
| `internal/repoadd` | Shared "add a repository" core: clone via `ResolveDirect` **first**, then write the project's `Repo:` directive, so a directive never points at a repo the host can't reach. A `Result` with a `Path` plus an error means only the directive failed |
| `internal/reposcmd` | `noctra repos add` / `list` — CLI channel over `repoadd` |
| `internal/agent` | Backends behind `Backend` ([`agent-backends`](../agent-backends/SKILL.md)); shared prompt builders, `BuildFixPrompt`, `BlockedLine`, log_offset, `ExtractSummary`, `ExtractFindingReplies`, pricing |
| `internal/plugins` | Curated, commit-pinned agent skills ([`agent-backends`](../agent-backends/SKILL.md#plugins)): `Catalog`/`Resolve` (packs + `AGENT_PLUGINS_EXTRA`), `Install` (fetch at the pinned SHA, verify, build a trimmed plugin dir), `Stage` (copy skills into a worktree for non-Claude backends), `ExcludeStaged` |
| `internal/review` | Optional Gemini review gate. API mode requests JSON (`verdict` + `summary` + line-anchored `findings`); `process.go` posts findings as inline PR comments (`github.PostInlineComments`, each with `NoctraReplyMarker`) and leaves a concise verdict in the PR body. CLI mode / unparseable JSON fall back to the prose verdict |
| `internal/budget` | Daily token/USD caps (`MAX_DAILY_TOKENS` / `MAX_DAILY_USD`), reset at UTC midnight |
| `internal/notify` | Fire-and-forget `Notifier` (`Send`/`SendSync`): Telegram, Slack, Discord; `Multi` fans out (`buildNotifier` in `pipeline`). Slack/Discord are on when their webhook URL is non-empty; Telegram keeps `TELEGRAM_ENABLED`. Messages use single-`*` mrkdwn; Discord rewrites to `**x**` and sends `allowed_mentions:{parse:[]}` |
| `internal/telegram` | Inbound listener: long-poll `getUpdates`, sender auth, dispatcher. `Register` for one-shot commands; `RegisterConversation` for guided flows — while live, plain messages route to it; it ends on completion, `/cancel`, a 5-minute `sessionTTL`, or any other command (which interrupts it and still runs) |
| `internal/github` | `gh` wrapper: `ListNoctraPRs`, `GetPR` (comments + reviews + inline comments via REST + `statusCheckRollup`), `CheckLogs`, thread replies/resolution; `Command`/`CommandInDir` ([`github-identity`](../github-identity/SKILL.md)) |
| `internal/ghauth` / `internal/ghauthcmd` | GitHub App client side and `noctra github login\|logout\|status`, `git-credential`, `Activate` |
| `internal/state` | SQLite store (`STATE_DB`, default `~/.noctra/state.db`; `modernc.org/sqlite`, single conn): PR cursors + CI SHA + iteration count (`pr_states`), sweep cooldowns (`sweep_states`), plan/run-history/usage, the rotating Linear OAuth token. `OpenMigrating` imports legacy JSON (`STATE_FILE`) only when the DB is new; the JSON is never deleted |
| `internal/watch` | Side-effect-free PR classifier ([`auto-iterate`](../auto-iterate/SKILL.md)) |
| `internal/lessons` | Learns repo conventions from human commits on merged Noctra PRs (`ProcessMergedPRs`) |
| `internal/pipeline` | Poll loop + worker pool + per-ticket lifecycle (`process.go`); PR-watch (`iterate.go`); sweeps (`sweep.go`); Telegram handlers `/status` `/tickets` `/ticket` `/search-tickets` (`/find`) `/start` `/move` `/pause` `/resume` `/kill` `/requeue` `/sweep` (`commands.go`) and guided `/addrepo` (`addrepo.go`) |
| `internal/sweep` | Sweep task catalog + scheduler ([`sweeps`](../sweeps/SKILL.md)) |
| `internal/sweepcmd` | `noctra sweep [--task] [--repo] [--force]` — thin client over `POST /api/sweep` |
| `internal/dashboard` | Operations dashboard server + SSE hub; UI in `web/` ([`dashboard`](../dashboard/SKILL.md)) |
| `internal/setup` | Interactive wizard (`noctra setup`); writes `.env` only, merging into an existing one |
| `internal/cleanup` | Branches, worktrees, old logs |
| `internal/service` | `install-service`: renders the `systemd --user` unit (pure `unitFile(exePath, pathEnv)`), `daemon-reload`; `--start` enables/starts + `loginctl enable-linger`; refuses without `--force` if the unit exists. Pairs with `scripts/install.sh` |
| `internal/doctor` | Preflight checks; `gather` is side-effect-free, `Run` renders the report, `RunJSON` emits `{name, ok, detail, hint, note}` + non-zero on failure |
| `internal/selfupdate` | `Latest`/`IsNewer`/`assetName` (pure) + `Update` (download the GoReleaser archive via `gh`, verify SHA-256 against `checksums.txt`, atomic swap). `noctra run` fires a best-effort `checkForUpdate` at startup |

## Invariant 1: the `log_offset` pattern

Agent logs append across attempts. `agent.OffsetBefore` records the file size *before* the agent runs; `agent.ReadAfter` reads only the new tail. `BlockedLine` and `HasRateLimit` operate on that tail.

**Rule:** never scan the full log file to detect failures. That re-detects failures from previous attempts and causes false positives (e.g. a ticket that was rate-limited on attempt 1 would be falsely detected as rate-limited on attempt 2 even when the agent succeeded).

**Where it lives:** `internal/agent/exec.go` (`OffsetBefore`, `ReadAfter`), consumed in `internal/pipeline/process.go` and `internal/pipeline/iterate.go`.

## Invariant 2: the `noctra/` branch-prefix guardrail

The auto-iterate watcher identifies its own PRs by the `noctra/<id>` branch prefix. `repo.BranchName` generates it; `github.ListNoctraPRs` filters on it; `watch` and `iterate` depend on it.

**Rules:**
- Keep `CreateWorktree` and `ResumeWorktree` on the same prefix or the watcher silently never finds its own PRs.
- Never create a `noctra/`-prefixed branch for manual work or agent-assisted PRs in this repo. The watcher will claim it and push conflicting follow-up commits.
- Use `fix/`, `feat/`, `docs/`, `chore/`, or `eng-<n>-<slug>` prefixes for non-Noctra branches.

## Invariant 3: backend-agnostic `internal/agent` split

Shared agent logic goes in the common code (`exec.go`, `prompt.go`); a backend file holds only its invocation args and rate-limit regex. Details and per-backend quirks: [`agent-backends`](../agent-backends/SKILL.md).

## Invariant 4: cursor semantics

The PR-watch system tracks what feedback has been acted on via cursors in `internal/state`:

- **Comment/review cursors** are **timestamps** (naturally ordered). Conversation comments and inline review-thread comments share the comment cursor.
- **CI cursor** is keyed by **head commit SHA** (`LastCISHA`) — acted on once per commit. A fix push changes the SHA, making a fresh failure on the new commit eligible again.

**Rule:** don't change cursor types or comparison logic without understanding that timestamps work for comments (monotonically increasing) but not for CI (where the same SHA can have multiple check runs; what matters is "have we already acted on this SHA?").

## Invariant 5: pure functions for testability

Noctra follows a convention of extracting logic into **pure, side-effect-free functions** that are unit-tested independently from the I/O that calls them:

| Pure function | Package | What it does |
|--------------|---------|-------------|
| `claudeArgs` / `codexArgs` / `copilotArgs` | `internal/agent` | Build CLI argument slices |
| `unitFile(exePath, pathEnv)` | `internal/service` | Render the systemd unit file |
| `completionScript(shell)` | `cmd/noctra` | Generate shell-completion scripts |
| `watch.diff` / `watch.actionable` | `internal/watch` | Classify PR changes, apply trusted-reviewer rules |
| `selfupdate.IsNewer` / `assetName` | `internal/selfupdate` | Version comparison, archive name selection |

**Rule:** when adding new logic, prefer extracting the decision/formatting into a pure function with table-driven tests, then call it from the I/O layer. This keeps tests fast and deterministic.

## Invariant 6: golangci-lint v2 syntax

The repo uses `.golangci.yml` with `version: "2"` syntax. This means:
- Linters are configured under `linters.default` and `linters.settings`, not the v1 `linters.enable` list.
- Formatters are under `formatters.enable` (e.g. `gofmt`), not mixed with linters.

**Rule:** if adding linter config, use v2 syntax. v1 keys will cause a parse error.

## Invariant 7: the codebase carries no comments

Source files hold **zero comments** — the only exception is a compiler/tooling directive (`//go:embed`,
`//go:build`, `//nolint`, `// Code generated`). Do not reintroduce explanatory comments, doc comments on
exported symbols included. Names and structure carry the *what*; this file and `CLAUDE.md` carry the *why*.

The non-obvious facts that previously lived in comments, kept here so removing them lost nothing:

| Where | Invariant |
|---|---|
| `repo/worktree.go` | Worktree mutations serialize per clone via `lockRepo` — concurrent `git worktree add` on one clone collides on the `.git/config` lock. Remove the worktree **before** the branch: git won't delete a branch still checked out, and a leftover branch fails `worktree add -b`. |
| `repo/worktree.go` | Ticket dispatch uses `CreateOrResumeWorktree`, never `CreateWorktree` directly: a run that pushed its branch but died before `gh pr create` leaves an orphaned remote branch, and re-branching from main produces a non-descendant origin rejects as non-fast-forward. |
| `repo/worktree.go` | `clearWorktreePath` falls back to `os.RemoveAll` + `git worktree prune` when `worktree remove` fails. A directory git no longer tracks as a worktree (e.g. a half-deleted `node_modules` left by an interrupted cleanup) makes `worktree remove` a no-op and `worktree add` fail with "already exists" — forever, since nothing else deletes it. |
| `pipeline/sweep_branch.go` | Sweep branch names are fixed per repo+task, so they are reused across runs. A sweep skips while its previous PR is still open (a fresh run would collide with it), and otherwise pushes with `--force-with-lease` pinned to the SHA `ls-remote` just reported: the only remote branch left at that point belongs to a merged or closed PR, and a fresh branch from main is never its descendant. |
| `pipeline/iterate.go` | Every failure path must record the iteration before returning, or the cursor never advances and the same feedback loops forever. Two deliberate exceptions: infra failures (timeout / rate-limit) don't increment — they weren't real attempts — and a shutdown cancellation isn't recorded at all, since it would bump the count and could fire the cap warning. |
| `lessons/lessons.go` | Lessons come only from the PR branch's own (`--first-parent`, `--no-merges`) commits after `LastPushedSHA` that are neither `[bot]`-authored nor Noctra's. Noctra commits under the host's git identity, so it is recognised by the commit-body lines `Implemented by Noctra` / `Follow-up commit by Noctra` / `Autonomous maintenance by Noctra` (`noctraCommitRe`) — reword those in `pipeline` and Noctra starts learning from itself. A plain `git diff` also swept in everything an "Update branch" merge pulled from main, which is how feature descriptions ended up stored as conventions. |
| `pipeline/iterate.go` | Push whenever the branch is ahead, not just when the worktree is dirty: the agent sometimes self-commits, and gating on dirtiness alone silently drops those commits (ENG-182). |
| `pipeline/plan.go` | `hasPendingPlan` takes `p.mu` itself — callers must **not** already hold it. |
| `pipeline/sweep.go` | A manual sweep deliberately skips `MarkSwept` so an ad-hoc run never shifts the scheduled cadence. |
| `pipeline/sweep.go` | Both abort paths (timeout, token cap) **do** record the cooldown. A task that aborts will abort again identically, so skipping it would re-burn the full ceiling every cycle; `/sweep --force` is the escape hatch. Their `run_history` status is `aborted`, not `failed` — the run was healthy, we killed it. |
| `pipeline/sweep.go` | Outcome notifications send on `context.WithoutCancel(ctx)`: `markDone` cancels the task context the instant `processSweepTask` returns, and `notifier.Send` is fire-and-forget, so a plain `ctx` races with its own cancellation. |
| `agent/claude.go` | `runCapped` learns the real cost only from the terminal `result` event, which never arrives when the cap fires `cancel()`. It accumulates the per-turn usage breakdown plus the model name while streaming and estimates from `PricesForModel` whenever `result` is missing — otherwise aborted runs record `$0.00` and the daily cost cap never sees them. |
| `sweep/task_*.go` | A task that hits the token ceiling is non-converging, not under-provisioned — every aborted run consumed exactly its ceiling at 2M, 5M and 8M, while every productive one finished under 1M. Any task that verifies in a loop must cap both its work and its full-suite runs in the prompt, and be told to ship what is already green. |
| `notify/discord.go` | `allowed_mentions.parse` must be a **non-nil** empty slice so it marshals to `[]` and not `null`; `null` re-enables `@everyone`/`@here` parsing on untrusted ticket text. |
| `notify/telegram.go` | `EscapeMarkdown` covers Telegram's strict legacy-Markdown chars. Apply it to dynamic values only, leaving the template's own `*bold*` alone — an unescaped `snake_case` title returns 400 and the notification vanishes (PR #52). |
| `linear/types.go` | The self-comment filter still matches the pre-rename `**Nightshift` prefix (ENG-204 tickets carry it). Classify by the **first non-empty line** only: a leading `>` quote is a human quoting our notice, never a system comment. |
| `github/types.go` | `RepoURL` is the discovering clone's remote URL, not gh JSON, and preserves its scheme — synthesizing HTTPS from `owner/name` breaks auto-iterate on SSH-only private repos. |
| `github/client.go` | Truncation skips UTF-8 continuation bytes (top bits `10`) so a log slice never cuts mid-rune. |
| `github/client.go` | Every `Repo:` directive ref goes through `NormalizeRepoRef` before parsing: Linear rewrites a bare URL in project content into `[url](<url>)` on save, and the raw markdown reaches `ExtractOwnerRepo` as the ref. Unwrapped, a GitHub link errors as "no repo mapping" and a non-GitHub link silently resolves to `github.com/<owner>/<name>`. |
| `telegram/listener.go` | The HTTP client timeout must exceed `pollTimeout`, or every long-poll `getUpdates` aborts client-side. |
| `state/state.go` | `Update`'s callback runs under the store lock — never call back into the `Store` from it. `OpenMigrating` imports the legacy JSON only when the DB is newly created; an existing DB is never clobbered. |
| `agent/log.go` | `usageFooterRe` is anchored to end-of-string so it never eats a mid-summary mention of token usage. |
| `agent/copilot.go` | Bridge only tokens Copilot accepts (`gho_` / `github_pat_`). It rejects classic `ghp_` PATs and reads the env token before its own store, so injecting one breaks an otherwise-valid `copilot /login`. |
| `agent/antigravity.go` | `agy`'s `--print` is a **string** flag whose value *is* the prompt: the auto-approve flag must precede it and the prompt must be the final token, or `--print` swallows the next flag. |
| `dashboard/dashboard.go` | `/fonts/` stays unauthenticated: `@font-face` `url()` subrequests don't carry the page's `?token=`, so gating them silently breaks the brand fonts. |
| `selfupdate` | Dev/snapshot builds never advertise an update — they can't be compared to a release tag. |
| `config/config.go` | `MigrateLegacyPaths` renames `~/.nightshift*` → `~/.noctra*` only when the old exists and the new doesn't; best-effort, never clobbers. |
| `cmd/noctra` | `uninstall --help` must never trigger the destructive action, and an unrecognized flag errors rather than falling through. |
| `ghauthcmd/credential.go` | `git-credential` never fails git outright: when it cannot help (not linked, not installed, non-GitHub host) it prints the reason to stderr and returns no credentials, so public repositories still clone anonymously and private ones fail with git's own auth error. It reads `--dir` instead of resolving the config dir, because git runs helpers with the worktree as cwd and the cwd-checkout override would pick the wrong directory. |
| `ghauth/session.go` | Noctra's own `gh` calls reuse a cached `write` token until 10 minutes before expiry; the git helper and agent runs always mint fresh (`FreshToken`), so a long run never inherits a token that is about to lapse. An agent's `GH_TOKEN` is fixed at spawn and lives an hour, which is why `Activate` warns when `AGENT_TIMEOUT_MINUTES` exceeds 50. |
| `ghauth/sign.go` | `CanonicalString` is a wire format shared with `noctra-auth` (`src/signing.ts`); `TestCanonicalString_MatchesTokenServiceFormat` pins a vector checked against the TypeScript implementation. Change both sides together or every signed request is refused. |
| naming | Branch names, identifiers, commit and PR wording, the model label and the watcher's hidden markers are all read back by other code. Their rules live in the [`naming`](../naming/SKILL.md) skill rather than here. |
| `agent/auth.go` | `AuthFailureLine` only scans the last 40 non-empty log lines and is only consulted after a non-zero exit, so an agent that merely *edited* auth code mid-run is not misread as logged out. It deliberately does not match a bare `401` (that matches `file.go:401`). |
| `agent/backend.go` | `RunOptions.Env` is merged over the backend's own env (or `os.Environ()`) by key, so the agent's `GH_TOKEN` and `GIT_CONFIG_*` replace the process-wide app-mode values rather than appearing twice. |
| `doctor/agentauth.go` | The `agent auth` check is informational and always `ok`. It reports `unknown` rather than guess: Codex's precedence between `OPENAI_API_KEY` and a stored login is undocumented, so only `codex login status` decides; `GEMINI_API_KEY` alone means nothing to `agy` (it also feeds the review gate), so API key needs `modelProvider: gemini` in `~/.gemini/antigravity-cli/settings.json` as well. Copilot tokens are never an "API key": every route bills the Copilot plan. |
| `plugins/install.go` | `Install` builds a fresh plugin directory instead of reusing the upstream one: a generated `plugin.json` (named `noctra-<plugin>` so it never collides with a plugin the user installed) plus only the listed skills. Upstream hooks, commands and MCP servers never load, symlinks are skipped (they could point outside the skill), and the commit is checked after fetch so a moved ref can't slip in. A skill whose `SKILL.md` sits at the repo root (`Path: "."`) must set `Name` and `Only`, so it copies its own files rather than the whole repository; `Only` refuses anything that isn't a regular file. Install directories are immutable: the name is `<plugin>@<commit>-<variant>`, where the variant hashes the selected skills (paths, names, `Only`, `Exclude`), because one pinned plugin can resolve to different skill sets (`agent-skills` sits in two packs). An existing directory is never rebuilt, so `noctra setup` choosing other packs can't change skills under a running daemon; a new build is renamed into place atomically, and a lost rename race reuses the winner. Old variants stay on disk (small) until removed by hand. |
| `plugins/install.go` (`git`) | Every plugin `git` call sets `cmd.WaitDelay` and `http.lowSpeedLimit`/`lowSpeedTime`. Cancelling the context kills `git` but not its `git-remote-http` child, which keeps the output pipe open, so without `WaitDelay` a stalled fetch hangs `CombinedOutput` forever, deadline or not (`TestInstall_GivesUpWhenTheFetchStalls`). Startup and setup both bound plugin work with `plugins.SetupTimeout`, because `Pipeline.Run` installs plugins before the first poll. |
| `plugins/requirements.go` | A skill that drives a runtime (webapp-testing needs Python Playwright and its Chromium) declares `Requires`. `CheckRequirements` runs each check command once per process and drops unmet skills **before** `Install`, so the built plugin never offers a skill the host can't run; otherwise the agent tries to install the runtime mid-run, a slow download on a Pi. A missing dependency is a warning in setup, the banner and `doctor`, never a failure, and once the dependency appears the next start builds the larger skill set as a new variant directory. |
| `plugins/stage.go` | Codex, Copilot and Antigravity have no per-run plugin flag, but all three read project skills from `.agents/skills` (verified for Codex/Antigravity by probing the CLIs; Copilot per GitHub's docs). `Stage` copies skills there as `noctra-<plugin>-<skill>` for the run and the backend's `Run` removes them on return, before Noctra commits. Copies, not symlinks: an agent editing a staged file must not corrupt the shared install. `Stage` never removes a directory it didn't create: each staged copy carries a `.noctra-staged` marker, so a crashed run's leftover is replaced but a project's own skill at the same path is skipped. `info/exclude` hides untracked files, not deletions of tracked ones, so deleting a project's skill would land in the PR. `info/exclude` is also the lowest-precedence ignore source: a repo `.gitignore` negation such as `!.agents/skills/**` re-includes staged skills, so after staging each one `Stage` asks `git ls-files --others --exclude-standard` whether `git add -A` would pick it up, and removes it if so (or if the check fails). |
| `repo/worktree.go` | Every worktree creator calls `plugins.ExcludeStaged` inside `lockRepo`, adding `/.agents/skills/noctra-*/` to the clone's shared `info/exclude`. Removal after the run isn't enough on its own: agents sometimes commit mid-run, and `git add -A` would sweep the staged skills into the PR. |

## Quality gates

Before submitting changes:

```bash
go test ./...       # all tests must pass
go vet ./...        # must be clean
```

If `golangci-lint` is available:

```bash
golangci-lint run
```

These are the same checks CI runs. A PR that fails either will not be merged.
