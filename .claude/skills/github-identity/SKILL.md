---
name: github-identity
description: Use when touching how Noctra authenticates to GitHub — GITHUB_AUTH_MODE, the noctra-agent[bot] App and token service, `noctra github login`, the git credential helper, per-repo tokens, or the credentials an agent run receives.
---

# GitHub identity (`GITHUB_AUTH_MODE`)

Noctra talks to GitHub either as the user whose credentials are on the host (`token` mode, the original behaviour) or as `noctra-agent[bot]`, the public GitHub App owned by `onelastcommit` (`app` mode). `auto` (default) picks `app` once `noctra github login` has linked the host.

The App's private key never reaches the host. The token service in [`onelastcommit/noctra-auth`](https://github.com/onelastcommit/noctra-auth) (`NOCTRA_AUTH_URL`, default `https://auth.getnoctra.dev`) holds it and mints installation tokens, each scoped to **one repository** and one of three scopes: `write` (PRs/labels/replies), `git` (push/fetch), `read` (agents). Setup notes for the App itself live in `deploy/github-app/`.

```
noctra github login → device flow (user token, never stored) → Ed25519 keypair in GITHUB_AUTH_DIR (0700/0600) → POST /link
noctra run → ghauthcmd.Activate → process env: git credential helper + bot commit identity; github.SetTokenSource
  git (clone/fetch/push) → `noctra git-credential --scope git` → signed POST /token per operation
  gh (every call) → github.Command(ctx, ownerRepo, …) → GH_TOKEN for that repo (write scope, cached until 10 min before expiry)
  agent run → RunOptions.Env: read-scope GH_TOKEN + read-scope git helper
```

## Rules

- **Every `gh` call goes through `github.Command` / `CommandInDir`**, which needs the repository to pick the right installation token. In app mode it **fails closed**: a failed mint or unknown repository is an error, never a silent fall-back to the host's personal `gh` auth. A raw `exec.CommandContext(ctx, "gh", …)` reintroduces personal-credential use in app mode.
- **Git is configured purely through the process environment** (`GIT_CONFIG_COUNT`/`KEY_n`/`VALUE_n`): an empty `credential.helper` first resets every helper from the user's git config (including `gh auth setup-git`'s URL-scoped one), then Noctra's helper is added, plus `credential.useHttpPath=true` so git sends the repository path. `~/.gitconfig` is never edited. Commit identity comes from `GIT_AUTHOR_*`/`GIT_COMMITTER_*`.
- **Agents get read-only credentials.** Noctra does all pushing and PR work itself, so `Pipeline.agentEnv` hands each run a `read` token for the worktree's repository and a `read`-scope git helper. If minting fails the agent gets the sentinel `GH_TOKEN=noctra-token-unavailable`, because an empty `GH_TOKEN` would let `gh` fall back to the host's personal login. This guards against accidental pushes; it is not a sandbox — an agent runs as the same OS user and could read the instance key or a personal `gh` login from disk.
- **Copilot** needs a *user* token for model access, which an installation token can't provide, so `copilotEnv` pins the user's token as `COPILOT_GITHUB_TOKEN` (read before `GH_TOKEN`).
- The PR watcher author (`github.Client.Author`), the banner (`GitHub as:`) and `doctor` (`github app` check) all reflect the mode. `GITHUB_AUTH_MODE=token` forces the old behaviour even when linked.
- `ghauth.CanonicalString` is a wire format shared byte-for-byte with `noctra-auth`; change both sides together (architecture Invariant 7).

## Config

| Env var | Default | Description |
|---------|---------|-------------|
| `GITHUB_AUTH_MODE` | `auto` | `auto` (app when linked), `app` (fail if not linked) or `token` (personal credentials) |
| `NOCTRA_AUTH_URL` | `https://auth.getnoctra.dev` | Token service used by `noctra github login`; self-hosters point it at their own deployment. The linked URL is stored with the instance |
| `GITHUB_AUTH_DIR` | `~/.noctra/github` | Instance ID and private key. Deliberately outside the cwd-checkout config override so a key is never written into a repository |
