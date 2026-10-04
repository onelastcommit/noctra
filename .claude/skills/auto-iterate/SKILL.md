---
name: auto-iterate
description: Use when changing the PR watcher (AUTO_ITERATE_PRS) — how Noctra finds its own PRs, classifies review feedback and CI failures, advances cursors, re-engages the agent, or replies to and resolves review threads.
---

# Auto-iterate on PR feedback

With `AUTO_ITERATE_PRS=true` a **second** poll loop runs beside the Linear one, on the same `WaitGroup`, so shutdown drains in-flight iterations. Off by default.

```
PR poll loop → github.ListNoctraPRs → github.GetPR (comments+reviews+statusCheckRollup)
  → watch.Scan (diff vs state cursor: new feedback OR failing CI on a new head SHA)
  → pipeline.iteratePR (bounded, shares the worker pool + active-set)
  → repo.ResumeWorktree → [github.CheckLogs for CI] → agent.BuildFixPrompt → agent.Run
  → commit/push (same branch) → state.Update (advance comment/review/CI cursor + bump iteration count)
```

## Which PRs it claims

Only PRs **Noctra authored**: the `noctra/<id>` branch prefix (`repo.BranchName`) **plus** a body marker (`github.IsNoctraAuthoredBody`, matching the hidden `github.NoctraPRBodyMarker` that every PR body builder embeds, or the legacy `"by [Noctra]"` footer), **plus** the author (`--author @me` in token mode, `--author app/<slug>` in App mode).

Keep creation and watching in sync — same prefix, same marker — or the watcher silently never finds its own PRs. Sweep PRs were once missed exactly this way: their footer lacked the old marker. The [`naming`](../naming/SKILL.md) skill owns every one of these strings.

## What counts as feedback

- Captured: conversation comments, review summaries (`CHANGES_REQUESTED` / non-empty `COMMENTED`), and inline review-thread comments (fetched separately via `gh api`, non-fatal on failure). `APPROVED`, `DISMISSED` and empty `COMMENTED` advance the cursor without acting.
- **Trusted-reviewer rule** (`watch.actionable`): humans are always actionable; bots only when their login is in `TRUSTED_REVIEWERS`. CI is not gated by this. Two non-actionable exceptions, cursor still advancing:
  1. a comment whose **whole** body is a bot-directed command (`@codex review`, `@gemini`, `/review`) — it targets another tool. A comment mixing a command with real feedback still acts.
  2. Noctra's **own** replies, identified by the hidden `github.NoctraReplyMarker`. Under a personal GitHub account those replies look human; without the marker they get re-read as feedback → re-engage → reply → loop until `MAX_PR_ITERATIONS` (observed before the marker existed).
- **CI failures** are a second trigger into the same `iteratePR`: when every check on the head commit (`statusCheckRollup`) has completed and ≥1 failed, `watch.diff` sets `PRChanges.CIFailure`; `iteratePR` fetches failed-step logs (`gh run view --log-failed`, truncated, best-effort) and folds them into the same fix prompt. Pending review feedback and CI are handled in one re-engagement. If the token can't read the rollup (GitHub refuses the whole `gh pr view` with "Resource not accessible by integration" when, say, the App lacks Commit statuses), `GetPR` retries without it and warns: feedback still works, the CI trigger is off for that PR, and the CI cursor (only advanced on a failure) is untouched.

## Cursors and guards

- Comment/review cursors are **timestamps** (naturally ordered; conversation and inline comments share the comment cursor). The CI cursor is the **head commit SHA** (`LastCISHA`) — acted on once per commit, since a fix changes the SHA and makes a fresh failure eligible again.
- `MAX_PR_ITERATIONS` is per PR and **shared** across review and CI re-engagements. Timeouts and rate limits don't count.
- Every failure path in `iteratePR` must record the iteration before returning, or the cursor never advances and the same feedback loops forever (architecture Invariant 7 has the exceptions).
- `pipeline.active` dedupes, so a ticket can't be freshly dispatched and iterated at once.

## Replying per finding

The fix prompt asks the agent for a JSON array — one `{finding, addressed, reply}` per numbered review finding — wrapped in `agent.FindingsStartMarker`/`EndMarker` and parsed by `agent.ExtractFindingReplies`.

- The prompt's `### N)` findings map 1:1 to `watch.PRChanges.Events[N-1]`, and an inline finding's `Event.ThreadID` equals its thread's `FirstCommentDatabaseID` (REST `id` == GraphQL `databaseId`). `ThreadID` is the comment's `in_reply_to_id` when it is a reply inside a thread, else its own `id`; keying on `CommentID` silently dropped every reply to a human who answered inside an existing thread (noctra-site#66). A reply event also carries its parent (`Event.ReplyTo`), which the fix prompt quotes so the agent knows what "let's fix this" refers to, and the prompt treats a human reviewer's request as an instruction rather than a suggestion. So `pipeline.postIterationReplies` routes each `reply` to the **exact** thread it came from via `github.ReplyToThreadsByComment`, carrying `NoctraReplyMarker`.
- A thread is **resolved only when its finding is `addressed`**. A finding Noctra pushed back on stays open; a thread with no matching finding gets neither reply nor resolution. One summary is never broadcast to every thread — that was the bug behind PR #246/#250, where a single `gofmt`-fix reply landed on several unrelated Gemini findings.
- The reply prefixes "Addressed in `<sha>`." when HEAD advanced. Key it on HEAD movement, **not** branch-ahead-of-remote, because the agent sometimes self-pushes.
- **Fallback:** no parseable per-finding block (older logs, a non-compliant backend) → one conversation comment with the run summary (`agent.ExtractSummary`), every thread left open. Conversation-level feedback always gets one comment, so a no-diff review isn't silent.
- The summary is persisted to `state.PRState.LastReasoning` and fed into the next fix prompt (`agent.FixPromptInput.PriorReasoning`) so re-engagements don't re-litigate settled feedback. It also notifies via Telegram/Linear.
