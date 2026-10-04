# Deploying and operating Noctra

Noctra is one binary that polls outbound over HTTPS. It needs no inbound port, webhook or tunnel. Run it on a laptop, a Raspberry Pi, a container or a PaaS.

## Install options

```bash
curl -fsSL https://raw.githubusercontent.com/onelastcommit/noctra/main/scripts/install.sh | sh   # release binary into ~/.local/bin
brew install onelastcommit/tap/noctra                                                          # macOS
go install github.com/onelastcommit/noctra/cmd/noctra@latest                                   # Go toolchain
git clone https://github.com/onelastcommit/noctra.git && cd noctra && go build -o noctra ./cmd/noctra
```

Prebuilt archives for linux amd64/arm64/armv7 and macOS amd64/arm64 are on the [Releases page](https://github.com/onelastcommit/noctra/releases). If `noctra` isn't found after the one-liner, add `~/.local/bin` to your `PATH` (the installer prints the line for your shell).

## Docker

The GHCR image ships `git`, `gh` and the Claude Code, Codex and Copilot CLIs. It doesn't include Antigravity's `agy`.

```bash
cp .env.example .env    # fill in LINEAR_API_KEY, AGENT_BACKEND, agent + GitHub keys
mkdir -p data           # /data holds the repo cache, worktrees, logs and state DB
docker run -d --name noctra --env-file .env -v "$PWD/data:/data" ghcr.io/onelastcommit/noctra:latest
docker logs -f noctra
```

Or `docker compose up -d` with [`docker-compose.yml`](../docker-compose.yml).

A container has no interactive login, so authenticate with keys in `.env`:

| Env var | For |
|---------|-----|
| `LINEAR_API_KEY` | Linear (required) |
| `AGENT_BACKEND` | `claude`, `codex` or `copilot` |
| `ANTHROPIC_API_KEY` *or* `OPENAI_API_KEY` | The agent backend you chose (Copilot uses `GH_TOKEN`) |
| `GH_TOKEN` | `gh` and `git push` (a token with repo + PR scope); also authenticates Copilot |
| `GIT_USER_NAME` / `GIT_USER_EMAIL` | Commit identity (defaults to a `Noctra` bot) |

Use HTTPS URLs or `owner/name` in each project's `Repo:` directive so `GH_TOKEN` authenticates clones; SSH would need a mounted key. To use the [GitHub App](github-app.md), run `docker exec -it noctra noctra github login` once; the key lands on `/data`.

## Cloud (Fly · Render · Railway · DigitalOcean)

Each template deploys the GHCR image and persists `/data`. Set the same secrets as the Docker table.

| Platform | File | Deploy |
|----------|------|--------|
| **Fly.io** | [`fly.toml`](../fly.toml) | `fly volumes create noctra_data --size 1` → `fly secrets set …` → `fly deploy` |
| **Render** | [`render.yaml`](../render.yaml) | New → Blueprint → pick repo → fill secret env vars |
| **Railway** | [`railway.json`](../railway.json) | New → Deploy from repo; add a `/data` volume + variables |
| **DigitalOcean** | [`deploy/digitalocean-cloud-init.yaml`](../deploy/digitalocean-cloud-init.yaml) | Paste into a droplet's *User data* (read the secrets warning in the file) |

## Raspberry Pi

Download the **`linux_arm64`** (Pi 4/5, 64-bit OS) or **`linux_armv7`** (Pi 3 / 32-bit) archive from Releases, or cross-compile:

```bash
GOOS=linux GOARCH=arm64 go build -o noctra ./cmd/noctra         # Pi 4 / 5
GOOS=linux GOARCH=arm GOARM=7 go build -o noctra ./cmd/noctra   # Pi 3 / 32-bit
```

## Running as a service (systemd)

```bash
noctra install-service --start   # write the systemd --user unit, enable + start it, enable lingering
noctra start | stop | restart | status
noctra logs [-f]                 # service logs; `noctra tail` = logs -f
noctra update [--restart]        # download the latest release, verify its checksum, swap the binary
```

`install-service` points the unit at the installed binary and inherits your current `PATH`, so the service finds the same `git`/`gh`/agent CLI. `--force` overwrites an existing unit. Off systemd (macOS, Docker) these commands print a hint instead.

`noctra update` only replaces the binary; logs and state are untouched. From a git checkout, `make update` does the same by pulling `main`, rebuilding to a side file and swapping it atomically (`make help` lists the other targets, including `make tail TICKET=ENG-42` for one ticket's transcript).

The startup banner prints the live configuration (backend, trigger, review gate, auto-iterate, notifications, GitHub identity), followed by a hint when a newer release exists.

## Other commands

```bash
noctra doctor [--json]               # preflight checks; --json emits {name, ok, detail, hint} and exits non-zero on failure
noctra cleanup [--force]             # remove merged/stale noctra/* branches, worktrees and logs older than 7 days
noctra completion bash|zsh           # shell completion script
noctra repos add | list              # clone a repo and write its Linear project's Repo: directive
noctra sweep [--task] [--repo] [--force]   # trigger maintenance sweeps now (needs the dashboard admin token)
```
