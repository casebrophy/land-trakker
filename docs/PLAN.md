# Land Trakker — Project Plan (Land Sales Aggregator)

A personal tool for scraping and aggregating land listings (and later, recorded sales) across the Intermountain West, starting with Idaho. Combines two use cases: **deal-finding** (daily alerts on matching new listings and price drops) and **comps** (historical sales analysis to evaluate whether a listing is well-priced).

> Naming note: this project is **land_trakker** (Go module `github.com/cbrophy/land_trakker`). The binary is `land_trakker`, config is `land_trakker.toml`, the VPS install path is `/opt/land_trakker`, and the Postgres database/user is `land_trakker`. (An earlier draft used the name "landsales"; it has been rebranded throughout.)

---

## 0. Quick reference — decisions already made

| Area | Decision |
|---|---|
| Region (v1) | Idaho. Expansion: Montana → Utah → Wyoming → Colorado |
| Property scope | All land types, any acreage |
| Listing freshness | Daily scrape cadence |
| Stack | Go + Postgres + PostGIS, Ardan Labs domain-driven layering |
| Hosting | Existing Linux/Docker VPS |
| UI | Whatever's fastest — Go + HTMX + templates |
| Auth | Single-user, hardcoded password |
| Alerts | Daily digest in the web UI (no email/push) |
| Photos | Hotlink URLs, don't store binaries |
| Geocoding | Mapbox or Google free tier |
| Recorded sales | Phase 2; manual CSV import OK for hard counties |
| Dedup across sources | Show as separate listings, manual dismiss |
| History retention | Forever |
| Raw HTML | Stored per fetch |
| Backups | Same VPS, `/backups` dir, rsync |
| Testing | Substantial — table-driven Go tests, parser fixtures |
| Deploy | Git push → VPS poll-and-pull job |
| LandWatch | Included; try and adapt if blocked |
| Auctions | Included in v1 |
| Attribute extraction | Regex/keyword first, LLM fallback for hard cases |
| Broker contact info | Stored |
| Monitoring | Health dashboard in web UI + log aggregation + Postgres metrics |
| DB access layer | pgx + sqlc |
| Migrations | goose |
| Module path | `github.com/cbrophy/land_trakker` |

---

# Part A — Architecture Reference

## 1. Goals and non-goals

**Goals**
- Daily scrape of active land listings across multiple Idaho sources, normalized into one schema.
- Plugin-style scraper architecture: add a new source without touching the pipeline.
- Full history: every snapshot of every listing preserved, so price drops, relistings, and days-on-market are queryable retroactively.
- Saved searches with daily digest of new matches and price drops.
- Web UI for searching, browsing, dismissing duplicates, and viewing scraper health.
- Comps capability (Phase 2): recorded county sales joinable to listings via parcel matching.
- Geocoding every listing to a lat/long, mappable in the UI.
- Best-effort extraction of unstructured attributes (water frontage, road access, off-grid, utilities) using regex first and LLM fallback.

**Non-goals**
- Not multi-tenant. Not a public product. One user.
- Not real-time. Daily resolution.
- Not breaking captchas or rotating residential proxies. If a site requires that, skip it.
- Not a parcel data product per se — parcel data is internal infrastructure for comps and matching.
- Not a CRM. Save / dismiss are the only user actions on a listing.
- Not storing image binaries. Hotlink and accept that some images go stale.

## 2. System overview

One Go binary, one Postgres database, one VPS, one Docker network. Resist microservices.

```
                            ┌─────────────────────────┐
                            │  scheduler (in-process) │
                            └────────────┬────────────┘
                                         │
                       ┌─────────────────┼─────────────────┐
                       ▼                 ▼                 ▼
                ┌────────────┐    ┌────────────┐    ┌────────────┐
                │ scraper:   │    │ scraper:   │    │ scraper:   │
                │ landwatch  │    │ knipe      │    │ fay        │  …
                └─────┬──────┘    └─────┬──────┘    └─────┬──────┘
                      │                 │                 │
                      └─────────────────┼─────────────────┘
                                        ▼
                            ┌──────────────────────┐
                            │ raw_fetches  (blob   │
                            │ in Postgres or       │
                            │ filesystem)          │
                            └──────────┬───────────┘
                                       ▼
                            ┌──────────────────────┐
                            │ parser stage         │
                            │ (per-source parser)  │
                            └──────────┬───────────┘
                                       ▼
                            ┌──────────────────────┐
                            │ normalizer +         │
                            │ enrichment           │
                            │ (geocode, attrs, …)  │
                            └──────────┬───────────┘
                                       ▼
                            ┌──────────────────────┐
                            │ canonical tables     │
                            │ (listings, snapshots,│
                            │  parcels, sales, …)  │
                            └──────────┬───────────┘
                                       ▼
                            ┌──────────────────────┐
                            │ web UI (HTMX + Go)   │
                            │ search, alerts,      │
                            │ health, comps        │
                            └──────────────────────┘
```

Pipeline is **explicitly multi-stage** because we want every stage idempotent and re-runnable. If LandWatch's HTML structure changes tomorrow, we re-parse from stored raw fetches without re-scraping.

## 3. Ardan Labs four-layer layout

```
/cmd
  /api          # web UI server
  /scraperd     # scheduler daemon (long-running)
  /scrape-once  # CLI to run a single scraper manually (debugging)
  /backfill     # CLI to re-parse stored raw fetches
/business
  /domain
    /listing    # Listing, ListingSnapshot, ListingStatus
    /parcel     # Parcel, Sale
    /source     # Source config, ScrapeRun
    /search     # SavedSearch, MatchResult
  /sdk
    /listingbus # business logic over listings (dedup, status transitions)
    /parcelbus
    /searchbus
/foundation
  /scraper      # plugin interface, orchestrator, rate limiter, retry
  /parser       # shared parsing helpers (acres, prices, addresses)
  /geocode      # geocoding client (Mapbox/Google)
  /llm          # Anthropic API client for attribute extraction fallback
  /storage      # raw fetch storage abstraction
  /web          # router, middleware, auth, templates
/storage
  /listingdb    # SQL implementations (pgx + sqlc)
  /parceldb
  /sourcedb
  /searchdb
  /migrations   # versioned SQL migrations (goose)
```

**Foundation has no domain knowledge.** `scraper` knows about HTTP, rate limits, fetch records — not "listings."

**Business depends only on domain types and abstract repository interfaces** defined in domain packages. `storage/*db` packages implement those interfaces. Lets tests swap in in-memory implementations.

**Cmd binaries are thin** — they wire dependencies and call business APIs.

## 4. The Scraper plugin interface

This is the single most important interface in the project. Get it right early.

```go
// foundation/scraper

type Source struct {
    ID              string        // "landwatch", "knipe", "fay", ...
    DisplayName     string
    BaseURL         string
    RateLimit       time.Duration // min interval between requests
    Concurrency     int           // 1 for most sites
    UserAgent       string
    RespectRobots   bool

    // Inactivity policy (per-source, config-driven)
    AbsenceDaysBeforeStale         int // default 14
    AbsenceDaysBeforeInactive      int // default 30
    ConsecutiveMissedRunsThreshold int // default 3

    // Health gate: don't mark anything inactive if this run returned
    // fewer than this fraction of the prior run's count
    MinResultRatioForInactivation float64 // default 0.5
}

type Scraper interface {
    Source() Source
    ParserVersion() string   // e.g. "landwatch.v3"

    // Discover returns IDs of listings currently visible on the source.
    Discover(ctx context.Context) ([]ListingRef, error)

    // Fetch retrieves the full detail of one listing.
    Fetch(ctx context.Context, ref ListingRef) (RawFetch, error)

    // Parse extracts a structured ParsedListing from a RawFetch.
    // Pure function — no I/O. Allows re-parsing stored fetches.
    Parse(raw RawFetch) (ParsedListing, error)
}

type ListingRef struct {
    SourceListingID string
    URL             string
    Summary         map[string]any
}

type RawFetch struct {
    SourceID        string
    SourceListingID string
    URL             string
    FetchedAt       time.Time
    StatusCode      int
    ContentType     string
    Body            []byte
    Headers         http.Header
}

type ParsedListing struct {
    SourceID        string
    SourceListingID string
    URL             string
    Title           string
    Description     string
    PriceCents      *int64
    Acres           *float64
    Address         *Address
    County          *string
    State           *string
    Photos          []string
    Broker          *Broker
    StructuredAttrs map[string]any
    PostedAt        *time.Time
    UpdatedAt       *time.Time
    SourceStatus    string
}
```

**Adding a new site** = one new package implementing `Scraper`, plus a row in the `sources` config table. The orchestrator does the rest.

**Parser separation** is critical. `Fetch` does network I/O and stores bytes; `Parse` is pure and runs against stored bytes. When a site's HTML changes, we update `Parse`, run the `backfill` command, and reprocess everything — no re-fetching.

## 5. Orchestrator

A small in-process scheduler in `cmd/scraperd`. No external job queue needed at this scale.

Loop per source:
1. Check `sources.enabled` and `sources.next_run_at`.
2. Acquire per-source semaphore.
3. Call `Discover()` → list of `ListingRef`.
4. Compare against last run's discovered IDs → diff into: new, still-present, disappeared.
5. For new + still-present where TTL has expired, `Fetch()` and store `raw_fetches` row.
6. For each new raw fetch, call `Parse()` → normalize → upsert `listings`, insert `listing_snapshots`.
7. For disappeared: increment `consecutive_misses`. Apply inactivity policy (see §11).
8. Write `scrape_runs` row with stats: discovered, fetched, parsed, errors, duration.
9. Schedule next run.

Per-source TTL on detail fetches (default 24h) prevents re-fetching unchanged pages on every run — only listings whose summary hash changes get re-fetched.

## 6. Parser versioning and backfill

### 6.1 The failure mode it solves

A site changes its HTML. Our `Parse()` starts returning errors, or worse, silently returns degraded data. Discovery and fetching keep working — `raw_fetches` rows accumulate with fresh HTML — but new `listings`/`listing_snapshots` rows stop being written (or are written with bad data).

With parser versioning + backfill, you fix the parser, bump its version, run backfill, and every raw fetch from the broken window gets re-parsed. Snapshots, price changes, and status transitions are reconstructed retroactively from stored HTML.

### 6.2 Parser versioning

Every scraper declares a parser version string via `ParserVersion()`, bumped manually whenever `Parse()` changes in a way that could produce different output. Convention is `<source>.v<N>`.

### 6.3 Parse attempts table

Every call to `Parse()` — success or failure — writes a row:

```sql
CREATE TABLE parse_attempts (
    id              bigserial PRIMARY KEY,
    raw_fetch_id    bigint NOT NULL REFERENCES raw_fetches(id) ON DELETE CASCADE,
    parser_version  text NOT NULL,
    attempted_at    timestamptz NOT NULL DEFAULT now(),
    outcome         text NOT NULL,
        -- 'success'      = parsed cleanly, snapshot written
        -- 'partial'      = parser ran, some fields missing, snapshot still written
        -- 'parser_error' = parser threw, no snapshot — eligible for reparse on version bump
        -- 'unparseable'  = page recognized as not-a-listing (captcha, 404, etc.) — NOT eligible
    error_message   text,
    snapshot_id     bigint REFERENCES listing_snapshots(id)
);
CREATE INDEX ON parse_attempts (raw_fetch_id, attempted_at DESC);
CREATE INDEX ON parse_attempts (parser_version, outcome);
```

`unparseable` means the parser correctly identified the page as junk; those should *not* be retried on every parser bump. `parser_error` means something went wrong inside the parser; those are eligible for reparse when the parser changes.

### 6.4 What needs reparsing

```sql
SELECT rf.id
FROM raw_fetches rf
LEFT JOIN LATERAL (
    SELECT parser_version, outcome
    FROM parse_attempts
    WHERE raw_fetch_id = rf.id
    ORDER BY attempted_at DESC
    LIMIT 1
) latest ON true
WHERE rf.source_id = $1
  AND (
        latest.parser_version IS NULL
     OR latest.outcome = 'parser_error'
     OR (latest.outcome IN ('success','partial')
         AND latest.parser_version <> $2)
  );
```

`$2` is the current parser version. `unparseable` rows are deliberately excluded.

### 6.5 The backfill command

`cmd/backfill` is a CLI that walks eligible raw fetches and re-parses them.

```
land_trakker backfill --source landwatch
land_trakker backfill --source landwatch --dry-run
land_trakker backfill --source landwatch --since 2026-04-01
land_trakker backfill --source landwatch --force-unparseable
land_trakker backfill --all
```

Each reparse: loads `raw_fetches.body`, calls `Parse()`, writes a new `parse_attempts` row, on success/partial inserts a new `listing_snapshots` row tied to the same `raw_fetch_id`, recomputes the listing's denormalized "current" fields from its newest snapshot, and re-derives price changes (idempotent).

Snapshots are append-only. We never overwrite a bad snapshot, we add a better one.

### 6.6 Manual vs. automatic trigger

For v1, **manual.** The health dashboard surfaces eligibility and offers a [Run backfill] button that kicks off `cmd/backfill --source <id>` as a background job, streams progress, and writes a `backfill_runs` log row when finished. Phase 3+ can add auto-mode for small backfills (<500 fetches).

### 6.7 What happens to listings during the broken window

While broken: `Discover()`/`Fetch()` keep running; `Parse()` returns `parser_error`; `last_seen_at` still updates; denormalized fields stay frozen at the last good snapshot; no new snapshots/price_changes. After backfill: every raw fetch in the window gets a new snapshot; denormalized fields refresh; price_changes derived from the now-complete sequence; dashboard shows the gap as "recovered".

### 6.8 Edge cases

Backfill rewrites history when the parser was silently wrong; old (wrong) snapshots stay for audit, new (correct) ones become latest. Don't retroactively delete user-visible history (`search_hits` stay). Require `--dry-run` validation on large backfills. Tag every backfill run with parser version.

## 7. Data model

Postgres 16 + PostGIS.

### Core

```sql
CREATE TABLE sources (
    id                                  text PRIMARY KEY,
    display_name                        text NOT NULL,
    base_url                            text NOT NULL,
    enabled                             boolean NOT NULL DEFAULT true,
    rate_limit_ms                       int  NOT NULL DEFAULT 1000,
    concurrency                         int  NOT NULL DEFAULT 1,
    user_agent                          text NOT NULL,
    respect_robots                      boolean NOT NULL DEFAULT true,
    absence_days_before_stale           int  NOT NULL DEFAULT 14,
    absence_days_before_inactive        int  NOT NULL DEFAULT 30,
    consecutive_missed_runs_threshold   int  NOT NULL DEFAULT 3,
    min_result_ratio_for_inactivation   numeric(4,3) NOT NULL DEFAULT 0.500,
    last_run_at                         timestamptz,
    next_run_at                         timestamptz,
    notes                               text
);

CREATE TABLE scrape_runs (
    id                  bigserial PRIMARY KEY,
    source_id           text NOT NULL REFERENCES sources(id),
    started_at          timestamptz NOT NULL,
    finished_at         timestamptz,
    status              text NOT NULL,  -- 'running','ok','partial','failed'
    discovered_count    int,
    fetched_count       int,
    parsed_count        int,
    error_count         int,
    error_sample        text,
    notes               text
);
CREATE INDEX ON scrape_runs (source_id, started_at DESC);

CREATE TABLE raw_fetches (
    id                  bigserial PRIMARY KEY,
    source_id           text NOT NULL REFERENCES sources(id),
    source_listing_id   text NOT NULL,
    scrape_run_id       bigint REFERENCES scrape_runs(id),
    url                 text NOT NULL,
    fetched_at          timestamptz NOT NULL,
    status_code         int  NOT NULL,
    content_type        text,
    body                bytea NOT NULL,
    body_sha256         bytea NOT NULL,
    headers_json        jsonb
);
CREATE INDEX ON raw_fetches (source_id, source_listing_id, fetched_at DESC);
CREATE UNIQUE INDEX ON raw_fetches (source_id, source_listing_id, body_sha256);
```

Body stays in Postgres as `bytea` for v1. Move to filesystem only if Postgres bloats past ~50 GB.

### Canonical listings

```sql
CREATE TABLE listings (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id           text NOT NULL REFERENCES sources(id),
    source_listing_id   text NOT NULL,
    url                 text NOT NULL,
    first_seen_at       timestamptz NOT NULL,
    last_seen_at        timestamptz NOT NULL,
    status              text NOT NULL,        -- see §11
    consecutive_misses  int  NOT NULL DEFAULT 0,
    dismissed           boolean NOT NULL DEFAULT false,
    dismissed_reason    text,
    saved               boolean NOT NULL DEFAULT false,

    title               text,
    description         text,
    price_cents         bigint,
    acres               numeric(10,2),
    price_per_acre_cents bigint GENERATED ALWAYS AS (
        CASE WHEN acres > 0 AND price_cents IS NOT NULL
             THEN (price_cents / acres)::bigint END
    ) STORED,
    address_line        text,
    city                text,
    county              text,
    state               text,
    postal_code         text,
    geom                geometry(Point, 4326),
    photos              text[] NOT NULL DEFAULT '{}',
    broker_name         text,
    broker_phone        text,
    broker_email        text,
    posted_at           timestamptz,
    source_updated_at   timestamptz,

    attr_water_frontage boolean,
    attr_off_grid       boolean,
    attr_road_access    text,
    attr_power          boolean,
    attr_well           boolean,
    attr_septic         boolean,
    attr_property_type  text,
    attrs_extra         jsonb NOT NULL DEFAULT '{}',
    attrs_extraction    jsonb NOT NULL DEFAULT '{}'
);
CREATE UNIQUE INDEX ON listings (source_id, source_listing_id);
CREATE INDEX ON listings USING GIST (geom);
CREATE INDEX ON listings (state, county) WHERE status IN ('active','stale');
CREATE INDEX ON listings (price_per_acre_cents) WHERE status IN ('active','stale');
CREATE INDEX ON listings USING GIN (to_tsvector('english', coalesce(description,'') || ' ' || coalesce(title,'')));

CREATE TABLE listing_snapshots (
    id                  bigserial PRIMARY KEY,
    listing_id          uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    raw_fetch_id        bigint NOT NULL REFERENCES raw_fetches(id),
    captured_at         timestamptz NOT NULL,
    price_cents         bigint,
    acres               numeric(10,2),
    status              text,
    title               text,
    description         text,
    structured_attrs    jsonb,
    diff                jsonb
);
CREATE INDEX ON listing_snapshots (listing_id, captured_at DESC);

CREATE TABLE price_changes (
    id              bigserial PRIMARY KEY,
    listing_id      uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    changed_at      timestamptz NOT NULL,
    old_price_cents bigint,
    new_price_cents bigint NOT NULL,
    delta_cents     bigint GENERATED ALWAYS AS (new_price_cents - old_price_cents) STORED,
    snapshot_id     bigint REFERENCES listing_snapshots(id)
);
CREATE INDEX ON price_changes (listing_id, changed_at DESC);
CREATE INDEX ON price_changes (changed_at DESC) WHERE delta_cents < 0;
```

### Parcels and sales (Phase 2)

```sql
CREATE TABLE parcels (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    state               text NOT NULL,
    county              text NOT NULL,
    apn                 text NOT NULL,
    geom                geometry(MultiPolygon, 4326),
    centroid            geometry(Point, 4326),
    acres               numeric(10,2),
    owner_name          text,
    assessor_address    text,
    source              text NOT NULL,
    source_updated_at   timestamptz,
    raw                 jsonb
);
CREATE UNIQUE INDEX ON parcels (state, county, apn);
CREATE INDEX ON parcels USING GIST (geom);
CREATE INDEX ON parcels USING GIST (centroid);

CREATE TABLE sales (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    parcel_id           uuid REFERENCES parcels(id),
    sold_at             date NOT NULL,
    price_cents         bigint NOT NULL,
    deed_type           text,
    buyer               text,
    seller              text,
    source              text NOT NULL,
    raw                 jsonb
);
CREATE INDEX ON sales (parcel_id, sold_at DESC);
CREATE INDEX ON sales (sold_at);

CREATE TABLE listing_parcels (
    listing_id  uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    parcel_id   uuid NOT NULL REFERENCES parcels(id) ON DELETE CASCADE,
    match_type  text NOT NULL,                 -- 'apn','geo_acres','manual'
    confidence  numeric(3,2) NOT NULL,
    PRIMARY KEY (listing_id, parcel_id)
);
```

### Saved searches and alerts

```sql
CREATE TABLE saved_searches (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL,
    query       jsonb NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    enabled     boolean NOT NULL DEFAULT true
);

CREATE TABLE search_hits (
    id                  bigserial PRIMARY KEY,
    saved_search_id     uuid NOT NULL REFERENCES saved_searches(id) ON DELETE CASCADE,
    listing_id          uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    hit_at              timestamptz NOT NULL,
    reason              text NOT NULL,         -- 'new','price_drop','attribute_added'
    seen                boolean NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX ON search_hits (saved_search_id, listing_id, reason, hit_at);
CREATE INDEX ON search_hits (hit_at DESC) WHERE seen = false;
```

## 8. Geocoding strategy

- Mapbox Geocoding API free tier (~100k req/month free).
- Geocode at normalization stage, after parser produces `address`/`city`/`county`/`state`.
- Cache forever in `geocode_cache` keyed on a normalized address string.
- Fall back to county centroid if specific address geocoding fails (rural land often has no street address).
- Store `precision` (`rooftop`, `street`, `locality`, `county_centroid`) and the geocoder's confidence.

```sql
CREATE TABLE geocode_cache (
    id              bigserial PRIMARY KEY,
    address_key     text NOT NULL UNIQUE,
    geom            geometry(Point, 4326),
    precision       text NOT NULL,
    provider        text NOT NULL,
    confidence      numeric(3,2),
    raw             jsonb,
    cached_at       timestamptz NOT NULL DEFAULT now()
);
```

The geocoder is accessed through a `Geocoder` interface; the build track ships an in-repo fake. The real Mapbox client is wired in later (gated on secrets).

## 9. Attribute extraction

Two-stage, regex-first with LLM fallback. Cost-controlled.

**Stage 1 — Deterministic patterns.** Per-attribute extractor functions that scan title + description + structured fields:
- `attr_water_frontage`: `creek|river|stream|pond|lake(front|frontage)?|riparian` + negation
- `attr_off_grid`: `off[- ]?grid|no power|no utilities`
- `attr_road_access`: cascade `paved` → `gravel` → `seasonal|forest service` → `no road|landlocked` → `unknown`
- `attr_power`: explicit yes/no else null
- `attr_well` / `attr_septic`: keyword presence + negation
- `attr_property_type`: hierarchical scoring `timber|forest`, `ag|farm|hay|crop|pasture`, `hunting|recreational`, `lot|home site|residential`, `commercial`

Each extractor returns `(value, confidence, evidence_snippet)` stored in `attrs_extraction`.

**Stage 2 — LLM fallback.** Trigger only when deterministic extractors returned null/low confidence, description >200 chars, and LLM hasn't run on this snapshot. Anthropic API, structured JSON. Cache per `snapshot_id`. Budget cap `llm.daily_call_limit` (default 200). The LLM is accessed through an interface; the build track ships a fake.

**Full-text search** as always-available fallback: tsvector GIN index over title + description.

## 10. Cross-source duplicates

**Don't auto-merge.** Each `(source_id, source_listing_id)` is a distinct listing row. The UI surfaces likely duplicates and lets you dismiss either side.

```sql
CREATE TABLE possible_duplicates (
    listing_a_id    uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    listing_b_id    uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    score           numeric(3,2) NOT NULL,
    reasons         text[] NOT NULL,           -- 'geo','acres','price','broker','title'
    detected_at     timestamptz NOT NULL DEFAULT now(),
    user_decision   text,                      -- 'same','different', null=undecided
    PRIMARY KEY (listing_a_id, listing_b_id),
    CHECK (listing_a_id < listing_b_id)
);
```

Scoring inputs: geocoded distance, acreage delta, price delta, broker name similarity, title token overlap.

## 11. Listing status state machine

Statuses: `active`, `stale`, `presumed_inactive`, `confirmed_sold`, `withdrawn`.

| From | To | Trigger |
|---|---|---|
| (new) | `active` | First successful fetch |
| `active` | `stale` | Absent from `Discover()` for `absence_days_before_stale` days (default 14) AND ≥ `consecutive_missed_runs_threshold` consecutive missed runs (default 3) AND scrape run was healthy |
| `stale` | `presumed_inactive` | Absent for `absence_days_before_inactive` days (default 30) under same conditions |
| `active`/`stale` | `confirmed_sold` | Source explicitly reports sold/under-contract |
| `active`/`stale` | `withdrawn` | Source explicitly reports withdrawn/cancelled |
| `presumed_inactive` | `active` | Listing reappears |

**Health gate.** A scrape run is "healthy" for inactivation only if `discovered_count >= MinResultRatioForInactivation * prior_run.discovered_count`. If a site returns 5 results on a day it usually returns 500, no listings get marked inactive from that run. All thresholds live on the `sources` row.

## 12. Source-by-source notes

| Source | Type | Risk | Strategy |
|---|---|---|---|
| LandWatch | Big aggregator (CoStar) | High | Low rate (1 req/3s), realistic UA, respect robots; escalate to `chromedp` if blocked; kill switch |
| Lands of America | Same network | High | Likely identical infra; drop if redundant |
| Knipe Land Co | Idaho specialty | Low | Standard HTTP + goquery; v1 happy path |
| Fay Ranches | Intermountain West | Low–medium | Standard scraping, modern site |
| Whitetail Properties | Recreational | Medium | Check protections; good hunting data |
| Hall and Hall | High-end ranches | Medium | Smaller volume, high quality |
| Live Water Properties | Water-frontage | Low | Niche, well-targeted |
| Idaho Country Properties | Local broker | Low | Very scrapable |
| AcreTrader / auction | Auction model | Medium | Extra fields: auction end date, current bid, reserve |

**Politeness defaults:** 1 req/sec per source (3 sec for LandWatch), jitter 0–500ms, contactable User-Agent. Check `robots.txt` once/day and cache. Don't fetch detail pages whose summary hash hasn't changed.

## 13. Web UI

**Stack:** Go `net/http` + `chi` + `html/template` + HTMX + Tailwind (CDN). Map via Leaflet.

**Pages:** `/` (search + map), `/listings/{id}` (detail, photos, snapshot history, comps panel, save/dismiss), `/searches` (saved searches CRUD), `/digest` (today's matches + drops), `/duplicates` (review queue), `/health` (dashboard), `/admin/sources` (per-source config).

**Auth:** single bcrypt-hashed password in env/config, one session cookie, no user table.

## 14. Health dashboard and monitoring

Per-source panels: last successful run, 30-day status sparkline, discovered-count trend, active listing count, parser error rate, recent errors. Database panel: DB/table sizes, slowest queries (pg_stat_statements), connections, last backup. System panel: disk, mem/CPU, goroutine count via expvar.

**Logging:** structured JSON via `log/slog` to stdout. `/health/logs` tail view with grep. Alerting is in-UI only.

## 15. Geocoding rate limits and costs

Mapbox free tier 100k/month. Hard cap `geocoding.daily_request_limit` default 5000; if exceeded, defer geocoding to next day (listing still ingests, `geom` stays null). Dashboard surfaces deferred backlog.

## 16. LLM budget controls

`llm.daily_call_limit` (default 200), `llm.monthly_call_limit` (default 3000), enforced in the LLM client wrapper. Per-call cost logged. Disable via `llm.enabled = false`.

## 17. Deployment

```
/opt/land_trakker/
  ├─ binaries/
  │    └─ land_trakker-v0.0.42
  │    └─ land_trakker -> v0.0.42
  ├─ config/
  │    └─ land_trakker.toml
  ├─ logs/
  └─ backups/
       ├─ db/
       └─ rawfetches/
docker-compose.yml                 # postgres+postgis service + land_trakker service
```

**Deploy job:** systemd timer `land_trakker-deploy.timer` runs every 5 minutes, executing `deploy.sh`: `git fetch`; if behind, pull, `go build` into a versioned binary, run migrations (`goose up`), swap the symlink, restart the container; log success/failure. Builds happen on the VPS. If build fails, symlink is not swapped.

**Migrations:** `goose`, forward-only, one numbered SQL file per migration.

## 18. Backups

**Postgres:** nightly `pg_dump --format=custom` into `/opt/land_trakker/backups/db/land_trakker-YYYY-MM-DD.dump`. Keep 14 dailies, 8 weeklies, 6 monthlies. **Raw fetches** stay in Postgres for v1. **Restoration drill** quarterly; document in `BACKUP_RESTORE.md`. No offsite — same-VPS `/backups` + rsync.

## 19. Testing

**Unit tests:** `business/sdk/listingbus` (state machine, dedupe scoring, price-change detection), `foundation/parser` (acreage/price/address), `foundation/scraper` (rate limiter, retry, orchestrator scheduling), per-source `Parse()` (table-driven with HTML fixtures).

**Parser fixture pattern:** each scraper gets `testdata/<source>/*.html` + expected JSON. When a site changes, re-capture fixtures and the diff makes the change explicit.

**Integration tests:** real Postgres via `testcontainers-go`. Behind `//go:build integration`; run where Docker is present.

**End-to-end tests:** full HTTP server against test Postgres, exercising search filter, saved-search creation, listing detail.

**Coverage target:** 70% on `business/` and `foundation/parser/`.

**CI on deploy:** `go test ./...` before symlink swap; if tests fail, no deploy.

## 20. Configuration

One TOML file at `/opt/land_trakker/config/land_trakker.toml`. Most operational knobs live in the `sources` table; the TOML is bootstrap.

```toml
[server]
listen = ":8080"
admin_password_hash = "..."   # bcrypt
session_secret = "..."

[database]
url = "postgres://land_trakker:...@postgres:5432/land_trakker?sslmode=disable"

[geocoding]
provider = "mapbox"
api_key = "..."
daily_request_limit = 5000

[llm]
enabled = true
api_key = "..."
model = "claude-haiku-4-5"
daily_call_limit = 200
monthly_call_limit = 3000

[scraper]
default_user_agent = "land_trakker-bot/0.1 (+contact: you@example.com)"
default_rate_limit_ms = 1000
default_concurrency = 1

[backup]
daily_dir = "/opt/land_trakker/backups/db"
retention_daily = 14
retention_weekly = 8
retention_monthly = 6
```

---

# Part B — Phased Roadmap

## Phase 0 — Foundation

Skeleton everything; no real scraping. Repo init + Ardan layout; `docker-compose.yml` (Postgres+PostGIS + app); goose migrations with core tables; config loading; `cmd/api` skeleton (chi, auth, login, `/health`); `foundation/scraper` interface + orchestrator + rate limiter; `fakebroker` stub scraper end-to-end writing `parse_attempts`; `cmd/backfill` skeleton exercising the reparse query against a version bump; unit tests; deploy script + backup cron on the VPS.

**DoD:** push a commit, VPS auto-deploys, `fakebroker` produces 3 listings in the UI, bumping its `ParserVersion()` and running `backfill --source fakebroker` produces fresh snapshots tied to the same raw fetches.

## Phase 1 — First real scrapers

`knipe` + `idaho-country-properties` scrapers (discover/fetch/parse/fixtures/tests); parser helpers; Mapbox geocoding + `geocode_cache`; search UI (acres/price/county + map); listing detail with photos + snapshot history; snapshot diffing → `price_changes`; status state machine with health gate.

**DoD:** two scrapers run daily on the VPS, listings appear in the UI, filter by acres+county and see results on a map.

## Phase 2 — Specialty + auction sources

`fay-ranches`, `whitetail-properties`, `hall-and-hall`, `live-water-properties` (pick 3 of 4); first auction scraper (AcreTrader) with auction-specific fields; saved searches (schema, CRUD, daily-eval job); daily digest; possible-duplicates detection + review UI.

## Phase 3 — LandWatch + hardening

LandWatch scraper (conservative HTTP → `chromedp` if needed, kill switch); Lands of America if not redundant; attribute extraction Phase 1 (regex/keyword for 8 attributes); full-text GIN + UI filter; health dashboard fleshed out; structured logging + `/health/logs`.

**DoD:** all major Idaho sources flowing, health dashboard usable as the daily operational view.

## Phase 4 — Smarter

LLM attribute-extraction with budget caps; attribute filters in search UI; CSV import for parcel data; first automated parcel scraper for one Idaho county (Ada/Beacon); listing-to-parcel matching (APN-exact then geo+acres fuzzy); listing detail shows linked parcel + sales.

## Phase 5 — Comps and expansion

Idaho recorded sales for Ada, Canyon, Kootenai, Bonner, Twin Falls (Beacon-template); comps panel (nearby sales last 24 months, price/acre distribution); Montana statewide cadastral ingest; first Montana listing scrapers; property-type-aware comp filtering.

## Phase 6+ — Backlog

Wyoming/Utah/Colorado expansion; LLM listing summarization; anomaly detection (under-comp-median); public records integration; mobile-friendly polish; map clustering; per-county pricing trends.

---

# Appendix

## A. Open questions to revisit during build
- Does LandWatch require headless browser? Resolve in Phase 3.
- Is bytea-in-Postgres for `raw_fetches` sustainable at one year? Reassess end of Phase 3.
- How often is APN actually in listing data vs needing geo-fuzzy match? Check empirically in Phase 4.
- Are Idaho counties' sales data consistently downloadable? Survey before Phase 5.

## B. Risk register
| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| LandWatch blocks scraping | Medium | High | Specialty brokers fallback; kill switch |
| Site HTML changes silently break parsers | High | Medium | Raw-fetch storage + reparse via `backfill`; alert on parser-error rates |
| Postgres bloats from `raw_fetches` | Medium | Medium | Plan migration to filesystem at >50 GB |
| Geocoding free tier exhausted | Low | Low | Daily cap + deferred backlog |
| Cease-and-desist | Low | High | Conservative rate limits, contactable UA, comply quickly |
| VPS dies | Low | High | Backups + restore drill |
| Single-user password compromised | Low | Medium | Long random password, optional Tailscale-only access |

## C. Things explicitly deferred
Multi-user/proper auth; public API; email/push alerts; mobile app; image storage; anything outside the Intermountain West; investor-grade analytics.
