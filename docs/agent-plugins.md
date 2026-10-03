# Agent plugins (proposal)

Status: proposed, not built. Spike first (phase 1); nothing ships until it passes.

Noctra is feature-frozen. This is a deliberate exception, justified as a quality improvement to every PR rather than new surface area for users to operate.

## Goal

Give Noctra's agent runs a vetted set of Claude Code plugins (skills, plus the hooks some of them carry) that make PRs better: better-tested, smaller, and better-designed on UI work. The setup wizard asks what the user mostly builds and enables the matching packs.

## Decisions

| Question | Decision |
|---|---|
| Where plugins live | **Noctra-only.** Noctra loads them into its own runs; the user's interactive Claude setup is untouched |
| Catalogue | **Curated + escape hatch.** A small pinned catalogue, plus any plugin by repo URL at the user's own risk |
| Granularity | **Per install.** One wizard question; every repo gets the same packs |
| Backends | **Claude only in v1.** Other backends ignore the setting and the banner says so |

## How it works

### Loading

`claude` accepts a repeatable `--plugin-dir <path>` that loads a plugin **for that session only** (checked on CLI 2.1.288). `RunOptions` gains `PluginDirs []string`; `claudeArgs` and `claudeStreamArgs` append one `--plugin-dir` per entry. Nothing is written into the worktree, so a plugin can never end up in a PR. Plugins the user installed on the host keep loading exactly as they do today.

### Catalogue

A new `internal/plugins` package holds a Go table:

```
Pack    { Name, Description, Plugins []Plugin }
Plugin  { Name, Repo, Commit, Subdir, Licence, Exclude []string }
```

`Commit` is a full SHA. `Subdir` covers repos whose plugin isn't at the root (`context7`). `Exclude` lists paths dropped when the plugin is materialised, which is how a plugin is trimmed for unattended use.

Candidate packs, all confirmed to ship `.claude-plugin/plugin.json` or a marketplace entry:

| Pack | Plugin | Licence | Notes |
|---|---|---|---|
| `engineering` (always on) | `obra/superpowers` | MIT | Keep TDD, systematic debugging, verification. **Exclude** `brainstorming`, `writing-plans`, `executing-plans` and the SessionStart hook: they wait for a human who isn't there |
| | `addyosmani/agent-skills` | MIT | Ships hooks; review before inclusion |
| | `mattpocock/skills` | MIT | |
| | `DietrichGebert/ponytail` | MIT | Pushes towards the smallest diff. Ships hooks |
| | `multica-ai/andrej-karpathy-skills` | **none** | Fetched from upstream, not redistributed, but drop it if the licence stays absent |
| `frontend` | `pbakaus/impeccable` | Apache-2.0 | |
| | `Leonxlnx/taste-skill` | MIT | |
| | `nextlevelbuilder/ui-ux-pro-max-skill` | MIT | |
| later | `upstash/context7` | MIT | Current library docs; useful for `deps-update`/`modernize` sweeps. Needs network at run time |
| later | `ChromeDevTools/chrome-devtools-mcp` | Apache-2.0 | Lets the agent check its own UI change. Needs headless Chrome — heavy on a Pi |

Ruled out: memory plugins (`claude-mem`, `mem0`, `mempalace`, `agentmemory`, `beads`) duplicate Noctra's lessons system, run background services and could leak context across repos. Orchestrators (`ruflo`, `oh-my-claudecode`, `ralph`) would bypass `MAX_CONCURRENT` and the budget caps. Output compressors (`caveman`) change the agent's output style and risk breaking the `BLOCKED:` line and findings JSON Noctra parses. Mega-catalogues (`agentic-awesome-skills`) bloat every run's context.

### Fetching

Each plugin is shallow-cloned at its pinned commit into `~/.noctra/plugins/<name>@<sha>/` (under `/data` in Docker), the commit is verified, and `Exclude` paths are removed. This runs during `noctra setup` and at startup for anything missing. Public repos clone anonymously; in GitHub App mode the credential helper already declines non-installed repos without failing git.

A failed fetch logs a warning and the run proceeds without that plugin. Plugins never block a ticket.

### Config and wizard

```env
AGENT_PLUGIN_PACKS=engineering,frontend
AGENT_PLUGINS_EXTRA=owner/repo@<sha-or-tag>,...
```

- The wizard asks "What do you mostly build?" (web frontend, backend/API, CLI/library, mixed) and writes `AGENT_PLUGIN_PACKS`. `engineering` is always included. Empty or `none` disables plugins.
- `AGENT_PLUGINS_EXTRA` requires a pinned ref; an unpinned entry is rejected at config load. Extras are shown as **unvetted**.
- The startup banner shows `plugins: engineering, frontend (claude only)` and lists unvetted extras. `doctor` gains a check that every enabled plugin is fetched at its pinned commit.

### Trust

A plugin is third-party instructions running under `--dangerously-skip-permissions`, and plugin hooks execute shell commands on the host as Noctra's user. Pinning to a SHA means an upstream change can't reach a run until the catalogue is bumped and re-evaluated. Every hook in a catalogue plugin is read during review and either kept deliberately or excluded.

## Plan

### Phase 1 — spike (no product code)

1. By hand, clone each candidate at a fixed commit and trim superpowers as above.
2. On the sandbox repo (`ahmadAlMezaal/sandbox` + the Linear "Sandbox" project), run six tickets — two UI, two bug fixes, two small features — under three configurations: no plugins, `engineering`, `engineering + frontend`. Inject plugins by temporarily wrapping `claude` with the `--plugin-dir` flags.
3. Record per run: PR opened, Gemini verdict, auto-iterate rounds, tokens and cost, and whether Noctra parsed the `BLOCKED:` line, summary and findings JSON.
4. Go/no-go per plugin. A plugin stays only if it does not break parsing and does not raise cost without a visible quality gain. Superpowers' trimmed form gets its own row.

Output: a results table appended to this file and the final catalogue.

### Phase 2 — build (only if phase 1 passes)

1. `internal/plugins`: catalogue table, materialise (clone, verify, exclude), resolve packs + extras to directories. Pure resolution logic, table-tested.
2. `internal/config`: `AGENT_PLUGIN_PACKS`, `AGENT_PLUGINS_EXTRA`, pinned-ref validation.
3. `internal/agent`: `RunOptions.PluginDirs`; append `--plugin-dir` in both Claude arg builders (tests extend the existing `claudeArgs` cases).
4. `internal/pipeline`: resolve dirs once at startup, pass on every ticket, iterate and sweep run; banner line.
5. `internal/setup`: the "What do you mostly build?" question. `internal/doctor`: the plugin check.
6. Docs: `docs/configuration.md`, `.env.example`, a short `agent-backends` skill note.

### Later

Other backends (superpowers already ships Codex and Antigravity packaging), `context7` and `chrome-devtools-mcp`, and a per-repo `Plugins:` directive if per-install proves too coarse.
