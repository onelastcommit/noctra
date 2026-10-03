---
name: naming
description: Use when changing how Noctra names or labels what it creates on GitHub or disk: branch names, ticket and sweep identifiers, worktree or log paths, commit messages, PR titles and footers, the model label, or the hidden markers the PR watcher matches.
---

# Naming in Noctra

Every name Noctra writes is read back later by something else: a branch name by the PR watcher, a footer by the lessons extractor, an identifier by the active set. Treat each one as a **round trip**. A change to how a name is written is only finished when every reader listed below recognises the new form.

## Branches

| Kind | Form | Written by | Read back by |
|---|---|---|---|
| Ticket | `noctra/<identifier, lowercased>`, e.g. `noctra/eng-460` | `repo.BranchName` | `github.ListNoctraPRs` (prefix), `identifierFromBranch` (`pipeline/iterate.go`), `cleanup` (prefix) |
| Sweep | `noctra/sweep-<task>`, e.g. `noctra/sweep-deps-update` | `sweep.SweepBranchName` | `sweep.TaskSuffixFromBranch`, `identifierFromBranch`, the open-PR check in `processSweepTask` |

- The repo is left out of sweep branch names because a branch already lives in its repo. It stays in the **identifier** (below).
- `noctra/` is reserved for branches Noctra creates. The rule for humans and other agents is in `CLAUDE.md`.
- `github.ListNoctraPRs` claims a PR only when all three match: the `noctra/` prefix, a body marker (below), and the author: `app/<slug>` in GitHub App mode, `@me` in personal-token mode (`github.Client.Author`).

## Identifiers, directories and logs

- Ticket identifier: the tracker's ID, e.g. `ENG-460` or `GH-<SLUG>-<n>`.
- Sweep identifier: `SWEEP-<repo-slug>-<task>` (`sweep.SweepIdentifier`). It must stay **repo-qualified**: it keys the active set, the worktree directory and the log file, so two repos running the same task would otherwise block each other. `identifierFromBranch` rebuilds it from a short sweep branch plus the PR's `owner/name`.
- Repo slug: `repo.Slug(owner/name)`, e.g. `onelastcommit-onenote-mcp`. It equals the clone directory under `~/.noctra-repos`, which is where the scheduler reads it (`repo.SlugFromPath`), so both sides agree.
- Worktree `~/.noctra-worktrees/<identifier>`; log `logs/<identifier>.log` under the config dir.
- Show people `owner/name`, never the slug: the sweep PR body's **Repo:** line uses `sweepRepoName`.

## Commit messages and PR titles

- Subject and PR title follow Conventional Commits when the target repo uses them (`repo.UsesConventionalCommits`). The type comes from the agent's release-bump suggestion (`patch` gives `fix`, `minor` gives `feat`, `major` gives `feat!` plus `BREAKING CHANGE`), built by `conventionalSubject`. Otherwise the subject is `feat: implement <id> — <title>` or `<id>: <title>`.
- The body opens with one of three fixed phrases: `Implemented by Noctra`, `Follow-up commit by Noctra`, `Autonomous maintenance by Noctra`. `lessons.noctraCommitRe` matches them to keep Noctra from learning from its own commits, so the phrases stay word for word; anything may follow them.
- A `Co-authored-by:` trailer names the agent backend (`Backend.CoAuthor`).

## PR bodies and replies

- Footer: `*<phrase> by [Noctra](https://github.com/onelastcommit/noctra) 🌙 using <runner label>*`.
- `github.NoctraPRBodyMarker` (`<!-- noctra-authored -->`) goes in every PR body Noctra writes, ticket, sweep or salvaged draft alike. The watcher also accepts the legacy visible footer `by [Noctra]` for PRs that predate the marker.
- `github.NoctraReplyMarker` (`<!-- noctra-reply -->`) goes in every comment and thread reply Noctra posts, including Gemini's inline findings. Without it the watcher re-reads Noctra's own words as review feedback and loops.

## The model label

`agent.RunnerLabel(backend.Label(), usage.Model)` produces the runner label, e.g. `Claude Code (Opus 5.5)` or `OpenAI Codex (GPT-5.5)`. With no model it is the bare backend label. `agent.ModelDisplayName` prettifies `claude-<family>-<major>[-<minor>][-<date>]` IDs (dropping a `[1m]`-style suffix) and `gpt-*` IDs, and shows anything else verbatim.

| Backend | Where the model comes from |
|---|---|
| Claude Code | `modelUsage` in the run's JSON result, taking the costliest model since smaller ones handle sub-tasks. In the streaming path (token-capped runs) an aborted run never gets a result, so it falls back to the assistant messages seen, weighted by output tokens. |
| Codex | The session header line `model: <id>` that `codex exec` prints before the transcript (`codexModel`). |
| Copilot, Antigravity | Not read yet. Both accept `--model`, but neither output format has been checked against a real run, so their label stays bare. |

Parse a backend's model only from output you have captured from a real run. A guessed format fails silently and shows nothing or the wrong model.

## Bot identity

In GitHub App mode commits are authored and committed as `noctra-agent[bot] <336615789+noctra-agent[bot]@users.noreply.github.com>`, taken from the linked instance (`ghauth.IdentityEnv`). The slug, bot login and bot ID come from the token service's `/config`, so a self-hosted app gets its own names with no code change.
