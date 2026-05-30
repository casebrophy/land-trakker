# Autonomous build conventions

This repo's work is tracked as beads issues and executed unattended by `/batch-beads`
(which spawns `/execute-beads` per issue). Each issue ends with a hard gate:
`go build ./...`, `go vet ./...`, and scoped `go test`. The autonomous chain stays
green **only if every issue leaves the tree in a buildable, tested state.** These
conventions are load-bearing — every build issue's acceptance criteria reference them.

## The invariant (non-negotiable, per issue)

1. **Compiles & vets clean.** `go build ./...` and `go vet ./...` succeed after the issue.
2. **Tests pass.** New code ships with passing tests in the same change.
   - Pure logic → table-driven unit tests with in-memory fakes.
   - DB-touching code → `testcontainers-go` integration tests behind `//go:build integration`
     (Docker daemon must be running during the run).
3. **No real network, no real secrets.** Geocoding, the LLM, and every real scraper are
   reached through a Go **interface** with an in-repo **fake** in the build track. Real
   Mapbox/Anthropic clients and real per-site parsers are gated work (see below).
4. **Append-only history.** `listing_snapshots` are never overwritten; upserts are idempotent.
5. **Self-contained tooling.** `goose` and `sqlc` are pinned as Go tool dependencies and
   invoked via `make` targets — no global install assumed.

## Conventions

- **Module:** `github.com/cbrophy/land_trakker`. Layout per `docs/PLAN.md §3`.
- **DB:** Postgres 16 + PostGIS, accessed via **pgx + sqlc**; migrations via **goose**.
- **Web:** `chi` + `html/template` + HTMX + Tailwind (CDN), Leaflet for maps. Single binary.
- **Each issue** references its spec section as `Plan: docs/PLAN.md — §N` and carries
  `phase:N` + `complexity:{high|low}` labels and `metadata.test_packages`.
- **Naming:** binary `land_trakker`, config `land_trakker.toml`, install `/opt/land_trakker`.

## Tracks

- **Build track** (`Phase N` epics, `track:build`): runnable unattended now.
- **Deferred track** (`lt-deferred` epic, `track:deferred`): needs real secrets, the VPS,
  or captured real-site HTML. Each deferred issue depends on one of three decision issues —
  `gate-secrets`, `gate-vps`, `gate-fixtures` — so it never enters `bd ready` until you
  close that gate. Closing a gate is the human handoff point.

## Running it

```
# (start Docker first — integration tests need the daemon)
/batch-beads <phase-0-epic-id> --yes      # builds Phase 0, then stops
# later, after review:
/batch-beads <phase-1-epic-id> --yes
```
