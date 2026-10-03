---
name: dashboard
description: Use when changing the operations dashboard (internal/dashboard) — its Preact + TypeScript UI under web/, the esbuild bundle committed to static/, auth tokens, or the local dev server.
---

# Dashboard frontend (Preact + esbuild)

The UI is a **Preact + TypeScript** project under `internal/dashboard/web/` (`src/components/` = one component per panel: KPIs, active runs, queue, run history, donut, token/cost, spend, runs-by-repo, throughput, budget, repo cards, sweep matrix, log overlay), bundled with **esbuild** (`web/build.mjs`, ~3 deps, no Vite). It builds into a **single self-contained `index.html`** committed to `internal/dashboard/static/`, which `dashboard.go` serves via `//go:embed static`.

## After changing anything under `web/`

```bash
cd internal/dashboard/web
yarn install --frozen-lockfile   # first time / after dependency changes
yarn build                       # type-checks, then writes internal/dashboard/static/index.html (+ fonts/)
```

Commit the regenerated `internal/dashboard/static/` with the source change. The `dashboard-bundle` CI job runs `yarn build` and fails when the committed `static/` differs from a fresh build — it guards, it doesn't regenerate.

- `yarn dev` (`web/devserver.mjs`): esbuild watch + a mock `/api` (sample data + SSE) at `http://localhost:8080/?token=dev&admin_token=dev`. Builds to a gitignored `web/.dev/`, so it never dirties `static/`.
- `yarn watch` rebuilds `static/` on change, for testing against the real backend. `//go:embed` bakes `static/` in at compile time, so rebuild the Go binary to see UI edits.
- Keep ports faithful: the live `index.html` in git history is the source of truth for exact visuals and derivations.

## Why one inlined file

The page sits behind a read token, and `@font-face`/`<script>`/`<link>` subrequests don't carry the page's `?token=` query param. `build.mjs` inlines all JS + CSS into the HTML so the only subresource is fonts, which stay unauthenticated (`mux.Handle("/fonts/", …)`). **Keep the build emitting one file**: separate `/assets/*.js|css` chunks would hit the token gate, 401, and the page would silently fail to boot. Fonts live in `web/public/fonts/` (source of truth), are copied to `static/fonts/` at build, and are referenced by the absolute `/fonts/` URL (`external: ['/fonts/*']` in `build.mjs`).

## Why the build output is committed

`go build ./...` embeds `static/`, so it must hold real files to compile, and `dashboard_test.go` serves the page under `go test ./...` with no Node. Building only in CI would put a Node step in front of every `go build`/`go test`. Committing the output keeps `tag-on-merge.yml` → GoReleaser and the Dockerfile pure Go. `static/index.html` is marked `linguist-generated` in `.gitattributes` so its minified diff collapses; `web/node_modules/` is gitignored.

## Auth

`DASHBOARD_TOKEN` gates the page (`?token=`). `DASHBOARD_ADMIN_TOKEN` unlocks operator controls (kill / requeue / retry / pause / `POST /api/sweep`); it travels only in a request header, never the query string.
