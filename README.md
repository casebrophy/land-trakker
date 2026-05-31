# land_trakker

Personal tool for scraping and aggregating land listings across the Intermountain West (starting with Idaho). Combines deal-finding (daily alerts on new listings and price drops) and comps (historical sales analysis).

## Stack

- Go + Postgres 16 + PostGIS
- Ardan Labs domain-driven layering (`cmd / business / foundation / storage`)
- pgx + sqlc for DB access; goose for migrations
- chi + html/template + HTMX for the web UI

## Quick start

```bash
# Build
make build

# Unit tests
make test

# Integration tests (requires Docker)
make test-integration

# Lint
make lint

# Regenerate sqlc output
make sqlc-generate
```

## Layout

```
cmd/             binaries (api, scraperd, scrape-once, backfill)
business/        domain types + business-logic SDK
foundation/      scraper plugin interface, parsers, geocoding, web infra
storage/         SQL implementations (pgx+sqlc), migrations, query files
docs/            planning documents
```

## Tools

goose and sqlc are pinned as Go tool dependencies — no global install needed:

```bash
go tool goose ...
go tool sqlc ...
```
