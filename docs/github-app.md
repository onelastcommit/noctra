# GitHub identity (`noctra-agent[bot]`)

By default Noctra pushes branches, opens PRs and replies to reviews as whoever is logged into `gh` on the host. The public [**noctra-agent** GitHub App](https://github.com/apps/noctra-agent) gives it its own identity instead.

## Set up

1. **Install the app** on the accounts and repositories Noctra should work on: <https://github.com/apps/noctra-agent>. You can change the selection later.
2. **Link the host** that runs Noctra:
   ```bash
   noctra github login     # shows a code to enter at github.com/login/device
   noctra restart
   ```
3. **Check it:** `noctra doctor` shows `github app ✓`, the startup banner shows `GitHub as: noctra-agent[bot]`, and `noctra github status --repo owner/name` mints a test token for one repository.

PRs, commits, labels and review replies then come from `noctra-agent[bot]`. In Docker, run `docker exec -it <container> noctra github login` once; the key is stored on the `/data` volume.

## How it works

The app's private key never reaches your machine. It lives only in a small token service, [`onelastcommit/noctra-auth`](https://github.com/onelastcommit/noctra-auth), at `https://auth.getnoctra.dev`. `noctra github login` proves who you are with GitHub's device flow, generates an Ed25519 key pair in `~/.noctra/github` (folder `0700`, files `0600`) and registers the public half. Noctra then asks the service for short-lived tokens, signing each request with that key.

- **One repository per token**, each expiring within an hour.
- **Least privilege per job.** Push and fetch use a contents-only token, PR work uses a write token, and the coding agent gets a **read-only** token, so it cannot push or open PRs itself.
- **Fresh tokens for git.** Git asks Noctra's credential helper for a new token on every operation. Your `~/.gitconfig` is not changed.
- **No silent fallback.** If a token can't be minted (for example, the app isn't installed on that repository), the operation fails and says so.
- **Live permission check.** A token is minted only if the GitHub account that linked the host can still push to that repository.

The app has no Workflows permission (pushes touching `.github/workflows/` are rejected), cannot touch repositories where it isn't installed, and has no access to organisation or account settings.

## What the token service stores

Your **GitHub user ID**, the **installation IDs** you can access, and each host's **public key**, instance ID and link time. Not your GitHub user token, the tokens it mints, your login name, email address, repository names or code. Replay and rate-limit counters are deleted within the hour. Uninstalling the app drops that installation from every linked host; revoking it under GitHub **Settings → Applications**, or running `noctra github logout`, deletes the host's record. Full list: [`noctra-auth` README](https://github.com/onelastcommit/noctra-auth#what-this-service-stores).

## Moving from personal credentials

Existing setups keep working until you run `noctra github login`. After you link and restart:

- New PRs are opened by `noctra-agent[bot]`, and the PR watcher follows only the app's PRs. Finish or close PRs opened earlier under your account by hand.
- If branch protection limits who can push, allow `noctra-agent`.
- Keep `gh` logged in: `noctra update` and the Copilot backend still use it.
- To go back, set `GITHUB_AUTH_MODE=token` and restart, or run `noctra github logout`.

| Setting | Default | Meaning |
|---|---|---|
| `GITHUB_AUTH_MODE` | `auto` | `auto` uses the app once linked, `app` refuses to start unless linked, `token` always uses personal credentials |
| `NOCTRA_AUTH_URL` | `https://auth.getnoctra.dev` | Token service to link with |
| `GITHUB_AUTH_DIR` | `~/.noctra/github` (`/data/github` in Docker) | Where the instance ID and private key are kept |

## Self-hosting

Create your own app from [`deploy/github-app/manifest.json`](../deploy/github-app/manifest.json) (guide in [`deploy/github-app/`](../deploy/github-app/)), deploy [`noctra-auth`](https://github.com/onelastcommit/noctra-auth#self-hosting-with-your-own-app) to your Cloudflare account, and set `NOCTRA_AUTH_URL` before running `noctra github login`.
