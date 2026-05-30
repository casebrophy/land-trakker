#!/usr/bin/env bash
set -euo pipefail
cd /Users/casebrophy/personal/land_trakker

ACC="go build ./... and go vet ./... pass; new code has passing tests; tree left compiling."

# helper: mk <parent> <title> <labels> <prio> <desc> <metajson>
mk() {
  bd create --silent --type=task --parent="$1" --title="$2" -l "$3" -p "$4" \
    -d "$5" --acceptance "$ACC $6" --metadata "$7"
}

echo "Creating epics..."
P0=$(bd create --silent --type=epic --title="Phase 0 — Foundation" -l "phase:0,track:build" -p 1 -d "Skeleton + fakebroker proven end-to-end. Plan: docs/PLAN.md — Part B Phase 0. The only epic run unattended first.")
P1=$(bd create --silent --type=epic --title="Phase 1 — Core pipeline" -l "phase:1,track:build" -p 2 -d "Geocoding, search+map, snapshot diffing, fixture harness. Plan: docs/PLAN.md — Part B Phase 1.")
P2=$(bd create --silent --type=epic --title="Phase 2 — Search & dedup" -l "phase:2,track:build" -p 2 -d "Saved searches, digest, duplicates, auction ext. Plan: docs/PLAN.md — Part B Phase 2.")
P3=$(bd create --silent --type=epic --title="Phase 3 — Extraction & health" -l "phase:3,track:build" -p 2 -d "Regex attrs, full-text, health dashboard, logging, admin. Plan: docs/PLAN.md — Part B Phase 3.")
P4=$(bd create --silent --type=epic --title="Phase 4 — Comps infrastructure" -l "phase:4,track:build" -p 2 -d "LLM fallback, parcels schema, CSV import, matching. Plan: docs/PLAN.md — Part B Phase 4.")
P5=$(bd create --silent --type=epic --title="Phase 5 — Comps UI & expansion" -l "phase:5,track:build" -p 2 -d "Comps query+panel, Montana import. Plan: docs/PLAN.md — Part B Phase 5.")
DEF=$(bd create --silent --type=epic --title="Deferred — human-gated" -l "track:deferred" -p 3 -d "Real scrapers, deploy, backups, live keys. Each child depends on a gate-* decision. Plan: docs/PLAN.md.")

echo "Creating gate decisions..."
GSEC=$(bd create --silent --type=decision --parent="$DEF" --title="gate-secrets: provide Mapbox + Anthropic keys, admin password, session secret" -l "track:deferred,gate" -p 2 -d "Close once land_trakker.toml has real Mapbox + Anthropic API keys, a bcrypt admin password hash, and a session secret. Unblocks live-integration work. Plan: docs/PLAN.md §20.")
GVPS=$(bd create --silent --type=decision --parent="$DEF" --title="gate-vps: VPS reachable with docker-compose up and /opt/land_trakker dirs" -l "track:deferred,gate" -p 2 -d "Close once the VPS is reachable, docker-compose is up, and /opt/land_trakker layout exists. Unblocks deploy + backup. Plan: docs/PLAN.md §17-18.")
GFIX=$(bd create --silent --type=decision --parent="$DEF" --title="gate-fixtures: capture real per-site HTML fixtures and confirm robots/ToS" -l "track:deferred,gate" -p 2 -d "Close once representative HTML fixtures are captured under testdata/<source>/ and robots.txt/ToS reviewed per source. Unblocks real scrapers. Plan: docs/PLAN.md §12,§19.")

echo "Creating Phase 0 issues..."
bootstrap=$(mk "$P0" "bootstrap-module" "phase:0,complexity:high" 1 "Init go.mod (module github.com/cbrophy/land_trakker), Ardan dirs, Makefile (build/test/test-integration/lint/sqlc-generate; detect Docker), goose+sqlc as Go tool deps, sqlc.yaml, .gitignore, README. Plan: docs/PLAN.md §3,§17,§19; docs/AUTONOMY.md." "Leaves an empty-but-compiling module; make build and make test succeed on empty packages." '{"test_packages":["./..."]}')
config=$(mk "$P0" "config-toml" "phase:0,complexity:low" 2 "foundation/config TOML loader matching the §20 schema (server/database/geocoding/llm/scraper/backup) with env override. Plan: docs/PLAN.md §20." "Loads a sample land_trakker.toml into typed config; env vars override; unit tests cover precedence." '{"test_packages":["./foundation/config/..."]}')
migrations=$(mk "$P0" "migrations-core" "phase:0,complexity:high" 1 "goose migration infra + 0001_core.sql (sources, scrape_runs, raw_fetches, parse_attempts) + 0002_listings.sql (listings, listing_snapshots, price_changes). Plan: docs/PLAN.md §6,§7." "goose up/down run clean against Postgres+PostGIS; integration test (tag) applies all migrations." '{"test_packages":["./storage/migrations/..."]}')
domain=$(mk "$P0" "domain-types" "phase:0,complexity:high" 1 "business/domain/{source,listing}: structs, ListingStatus enum, repository interfaces (no SQL). Plan: docs/PLAN.md §3,§7,§11." "Domain types + interfaces compile; enum + basic invariants unit-tested." '{"test_packages":["./business/domain/..."]}')
parser=$(mk "$P0" "parser-helpers" "phase:0,complexity:high" 1 "foundation/parser: ParseAcres, ParsePrice, NormalizeAddress with table-driven + property tests. Plan: docs/PLAN.md §19." "Handles '40 acres','40 ac','40-acre tract', '\$1,200,000'; table + property tests pass." '{"test_packages":["./foundation/parser/..."]}')
scraperiface=$(mk "$P0" "scraper-iface" "phase:0,complexity:high" 1 "foundation/scraper: Scraper interface (with ParserVersion()), Source, ListingRef, RawFetch, ParsedListing, Address, Broker. Plan: docs/PLAN.md §4." "Interface + types compile; a compile-time assertion fixture implements Scraper." '{"test_packages":["./foundation/scraper/..."]}')
ratelimit=$(mk "$P0" "rate-limiter" "phase:0,complexity:low" 2 "foundation/scraper: per-source rate limiter (min interval + jitter) + retry/backoff. Plan: docs/PLAN.md §5,§12." "Rate limiter enforces interval; retry backs off; unit tests with a fake clock pass." '{"test_packages":["./foundation/scraper/..."]}')
storage=$(mk "$P0" "storage-impls" "phase:0,complexity:high" 1 "sqlc-generated queries + storage/{sourcedb,listingdb} (pgx) implementing domain interfaces. Plan: docs/PLAN.md §3,§7." "Repos satisfy domain interfaces; testcontainers integration tests (tag) cover CRUD + upsert." '{"test_packages":["./storage/sourcedb/...","./storage/listingdb/..."]}')
listingbus=$(mk "$P0" "listingbus" "phase:0,complexity:high" 1 "business/sdk/{listingbus,sourcebus}: upsert, snapshot insert + diff, status state machine + health gate, price-change derivation. Plan: docs/PLAN.md §5,§6,§11." "State machine transitions + health gate + price-change detection unit-tested with in-memory fakes." '{"test_packages":["./business/sdk/listingbus/...","./business/sdk/sourcebus/..."]}')
backfill=$(mk "$P0" "backfill-query" "phase:0,complexity:high" 1 "Reparse-eligibility query (§6.4) + parse_attempts writes + cmd/backfill skeleton. Plan: docs/PLAN.md §6." "Eligibility query excludes unparseable, includes never-parsed/parser_error/old-version; cmd/backfill --dry-run lists eligible fetches." '{"test_packages":["./cmd/backfill/...","./business/sdk/listingbus/..."]}')
orch=$(mk "$P0" "orchestrator" "phase:0,complexity:high" 1 "foundation/scraper orchestrator: Discover->diff->Fetch(TTL)->store raw->Parse->normalize->upsert->snapshot->scrape_runs. Plan: docs/PLAN.md §5." "One run over a fake scraper produces raw_fetches, snapshots, and a scrape_runs row; diff logic unit-tested." '{"test_packages":["./foundation/scraper/..."]}')
fake=$(mk "$P0" "fakebroker" "phase:0,complexity:low" 2 "In-repo stub scraper returning hardcoded fixtures, implementing Scraper incl. ParserVersion(). Plan: docs/PLAN.md Part B Phase 0." "fakebroker Discover/Fetch/Parse return 3 deterministic listings; ParserVersion bumpable." '{"test_packages":["./foundation/scraper/..."]}')
scraperd=$(mk "$P0" "cmd-scraperd" "phase:0,complexity:low" 2 "cmd/scraperd scheduler daemon wiring fakebroker through the orchestrator on a tick. Plan: docs/PLAN.md §5." "scraperd builds and runs one tick against fakebroker in a smoke test." '{"test_packages":["./cmd/scraperd/..."]}')
scrapeonce=$(mk "$P0" "cmd-scrape-once" "phase:0,complexity:low" 2 "cmd/scrape-once CLI to run a single scraper once for debugging. Plan: docs/PLAN.md §3." "scrape-once -source fakebroker runs end-to-end and reports counts." '{"test_packages":["./cmd/scrape-once/..."]}')
webshell=$(mk "$P0" "web-shell" "phase:0,complexity:high" 1 "foundation/web (chi, bcrypt password auth middleware, session cookie, html/template + HTMX + Tailwind CDN, login) + cmd/api + /health OK. Plan: docs/PLAN.md §13." "Unauthenticated requests redirect to login; valid password sets session; /health returns OK; handler tests pass." '{"test_packages":["./foundation/web/...","./cmd/api/..."]}')
weblist=$(mk "$P0" "web-listings" "phase:0,complexity:high" 1 "Listings list view + detail view (snapshot history) rendering store data. Plan: docs/PLAN.md §13." "/ lists ingested listings; /listings/{id} shows snapshot history; handler tests pass." '{"test_packages":["./foundation/web/..."]}')
e2e=$(mk "$P0" "p0-e2e-tests" "phase:0,complexity:high" 1 "Orchestrator/rate-limiter/backfill unit tests; fakebroker end-to-end (Discover->Insert); ParserVersion bump + backfill produces fresh snapshots tied to same raw_fetches. Plan: docs/PLAN.md Part B Phase 0 DoD." "End-to-end test ingests 3 listings; after ParserVersion bump, backfill adds new snapshots referencing the original raw_fetch ids." '{"test_packages":["./cmd/backfill/...","./foundation/scraper/..."]}')

echo "Creating Phase 1 issues..."
geocode=$(mk "$P1" "geocode-client" "phase:1,complexity:high" 2 "foundation/geocode: Geocoder interface + geocode_cache migration + cache logic + in-repo fake + county-centroid fallback + precision/confidence. Plan: docs/PLAN.md §8,§15." "Fake geocoder + cache covered by unit tests; cache migration applies; county-centroid fallback path tested." '{"test_packages":["./foundation/geocode/..."]}')
normgeo=$(mk "$P1" "normalize-geocode" "phase:1,complexity:low" 2 "Wire geocoding into the normalization stage via the fake geocoder; deferred-backlog accounting when daily cap hit. Plan: docs/PLAN.md §8,§15." "Normalized listings get geom from the fake; over-cap listings ingest with null geom; tests cover both." '{"test_packages":["./foundation/scraper/...","./foundation/geocode/..."]}')
searchf=$(mk "$P1" "search-filters" "phase:1,complexity:high" 2 "Search query model (acres/price/county/ppa/type/attrs) + listingbus query method + store query. Plan: docs/PLAN.md §13." "Filter combinations produce correct SQL/results; unit + integration (tag) tests pass." '{"test_packages":["./business/sdk/listingbus/...","./storage/listingdb/..."]}')
searchmap=$(mk "$P1" "web-search-map" "phase:1,complexity:high" 2 "Search UI: filters + Leaflet map + list + pagination (HTMX, no full reload). Plan: docs/PLAN.md §13." "Filter changes swap the result list via HTMX; map renders markers; handler tests pass." '{"test_packages":["./foundation/web/..."]}')
pricehist=$(mk "$P1" "price-history-ui" "phase:1,complexity:low" 2 "Detail page price/acres timeline from snapshots + price_changes. Plan: docs/PLAN.md §13." "Detail page renders a price timeline from snapshot history; handler test passes." '{"test_packages":["./foundation/web/..."]}')
fixharness=$(mk "$P1" "scraper-fixture-harness" "phase:1,complexity:low" 2 "Generic per-source fixture test pattern: testdata/<src>/*.html + expected JSON + a runner helper that deferred real scrapers plug into. Plan: docs/PLAN.md §19." "Harness runs a sample Parse() against a fixture and diffs against expected JSON; helper unit-tested." '{"test_packages":["./foundation/scraper/..."]}')

echo "Creating Phase 2 issues..."
savedsearch=$(mk "$P2" "saved-searches" "phase:2,complexity:high" 2 "saved_searches + search_hits schema + searchbus CRUD + daily-evaluation job (new/price_drop/attribute_added). Plan: docs/PLAN.md §7,§13." "Daily-eval job emits correct hit reasons; unique constraint dedupes; unit + integration (tag) tests pass." '{"test_packages":["./business/sdk/searchbus/...","./storage/searchdb/..."]}')
websaved=$(mk "$P2" "web-saved-searches" "phase:2,complexity:high" 2 "/searches CRUD UI + /digest daily digest page (new matches + price drops). Plan: docs/PLAN.md §13." "Create/edit/delete saved searches; /digest lists today's hits; handler tests pass." '{"test_packages":["./foundation/web/..."]}')
dedup=$(mk "$P2" "dedup-job" "phase:2,complexity:high" 2 "possible_duplicates schema + scoring (geo/acres/price/broker/title) + periodic job. Plan: docs/PLAN.md §10." "Scoring ranks known dup pairs above non-dups; canonical a<b ordering enforced; unit tests pass." '{"test_packages":["./business/sdk/listingbus/..."]}')
webdup=$(mk "$P2" "web-duplicates" "phase:2,complexity:low" 2 "/duplicates review queue UI (mark same/different/dismiss). Plan: docs/PLAN.md §10,§13." "Review queue lists undecided pairs; decision persists; handler test passes." '{"test_packages":["./foundation/web/..."]}')
auction=$(mk "$P2" "auction-ext" "phase:2,complexity:low" 2 "Auction extension table + ParsedListing auction fields (end date, current bid, reserve). Plan: docs/PLAN.md §12." "Auction fields persist via an extension table; migration applies; tests cover round-trip." '{"test_packages":["./storage/listingdb/..."]}')

echo "Creating Phase 3 issues..."
attrx=$(mk "$P3" "attr-extractors" "phase:3,complexity:high" 2 "foundation/attrs: regex/keyword extractors for the 8 attributes returning (value,confidence,evidence); fixture-tested. Plan: docs/PLAN.md §9." "Each extractor returns value+confidence+evidence; table-driven tests cover positive/negation cases." '{"test_packages":["./foundation/attrs/..."]}')
attrwire=$(mk "$P3" "attr-wire" "phase:3,complexity:low" 2 "Wire extractors into normalization; populate attr_* + attrs_extraction. Plan: docs/PLAN.md §9." "Normalized listings carry attr_* + attrs_extraction; test asserts evidence is recorded." '{"test_packages":["./foundation/scraper/...","./foundation/attrs/..."]}')
fts=$(mk "$P3" "fulltext-search" "phase:3,complexity:low" 2 "tsvector GIN index migration + listingbus full-text query + UI filter. Plan: docs/PLAN.md §9,§13." "Full-text query matches title/description tokens; integration (tag) test on the GIN index passes." '{"test_packages":["./storage/listingdb/...","./foundation/web/..."]}')
health=$(mk "$P3" "health-dashboard" "phase:3,complexity:high" 2 "/health per-source panels (last run, 30d sparkline, discovered trend, parser error rate), DB stats, system stats via expvar. Plan: docs/PLAN.md §14." "/health renders per-source + DB + system panels from real stats; handler tests pass." '{"test_packages":["./foundation/web/..."]}')
logging=$(mk "$P3" "logging-tail" "phase:3,complexity:low" 2 "Structured slog JSON to stdout + /health/logs tail+grep view. Plan: docs/PLAN.md §14." "slog emits JSON; /health/logs tails and greps; handler test passes." '{"test_packages":["./foundation/web/..."]}')
adminsrc=$(mk "$P3" "web-admin-sources" "phase:3,complexity:high" 2 "/admin/sources per-source config + recent runs + backfill-eligibility + [Run backfill] trigger. Plan: docs/PLAN.md §6.6,§13." "Edit per-source thresholds; eligibility count shown; [Run backfill] starts a background job; handler tests pass." '{"test_packages":["./foundation/web/..."]}')

echo "Creating Phase 4 issues..."
llm=$(mk "$P4" "llm-client" "phase:4,complexity:high" 2 "foundation/llm: LLM interface (Anthropic) + budget caps (daily/monthly) + cost logging + enabled flag + in-repo fake. Plan: docs/PLAN.md §9,§16." "Budget caps enforced in the wrapper; disabled flag short-circuits; fake-backed unit tests pass." '{"test_packages":["./foundation/llm/..."]}')
llmfb=$(mk "$P4" "llm-attr-fallback" "phase:4,complexity:high" 2 "Trigger LLM extraction for low-confidence rows (budget-gated), cache per snapshot_id, via the fake. Plan: docs/PLAN.md §9." "LLM invoked only when deterministic confidence low + desc>200 + not already run; result cached per snapshot; tests cover the gate." '{"test_packages":["./foundation/attrs/...","./foundation/llm/..."]}')
attrui=$(mk "$P4" "attr-filters-ui" "phase:4,complexity:low" 2 "Attribute filters in search UI (off-grid, water frontage, road access, utilities). Plan: docs/PLAN.md §13." "Attribute filters narrow results; handler test passes." '{"test_packages":["./foundation/web/..."]}')
parcels=$(mk "$P4" "parcels-schema" "phase:4,complexity:high" 2 "parcels, sales, listing_parcels migrations + parcelbus + storage/parceldb. Plan: docs/PLAN.md §7." "Migrations apply; parcelbus CRUD covered by integration (tag) tests." '{"test_packages":["./business/sdk/parcelbus/...","./storage/parceldb/..."]}')
csv=$(mk "$P4" "csv-import" "phase:4,complexity:low" 2 "cmd tool to import county parcel CSV exports into parcels. Plan: docs/PLAN.md §7, Part B Phase 4." "Importer ingests a sample CSV into parcels idempotently; unit test on the parser passes." '{"test_packages":["./cmd/...","./business/sdk/parcelbus/..."]}')
match=$(mk "$P4" "parcel-matching" "phase:4,complexity:high" 2 "Listing<->parcel matching: APN-exact then geo+acres fuzzy; populate listing_parcels with match_type+confidence. Plan: docs/PLAN.md §7, Part B Phase 4." "APN-exact and geo+acres fuzzy paths each tested; confidence + match_type recorded." '{"test_packages":["./business/sdk/parcelbus/..."]}')
parcelpanel=$(mk "$P4" "web-parcel-panel" "phase:4,complexity:low" 2 "Listing detail shows linked parcel + recorded sales. Plan: docs/PLAN.md §13." "Detail page renders linked parcel + sales when present; handler test passes." '{"test_packages":["./foundation/web/..."]}')

echo "Creating Phase 5 issues..."
compsq=$(mk "$P5" "comps-query" "phase:5,complexity:high" 2 "Nearby-sales query (last 24mo, distance, ppa distribution) + property-type filter. Plan: docs/PLAN.md Part B Phase 5." "Query returns nearby sales within radius+window with ppa stats; integration (tag) test on seeded data passes." '{"test_packages":["./business/sdk/parcelbus/...","./storage/parceldb/..."]}')
compspanel=$(mk "$P5" "web-comps-panel" "phase:5,complexity:high" 2 "Comps panel on listing detail (nearby sales + ppa distribution) over seeded data. Plan: docs/PLAN.md §13, Part B Phase 5." "Comps panel renders nearby sales + distribution; handler test passes." '{"test_packages":["./foundation/web/..."]}')
mtimport=$(mk "$P5" "montana-import" "phase:5,complexity:low" 2 "Montana cadastral CSV/shapefile import tool into parcels. Plan: docs/PLAN.md Part B Phase 5." "Importer ingests a sample Montana cadastral file into parcels; unit test passes." '{"test_packages":["./cmd/...","./business/sdk/parcelbus/..."]}')

echo "Creating deferred (human-gated) issues..."
opsdeploy=$(bd create --silent --type=task --parent="$DEF" --title="ops-deploy" -l "track:deferred,complexity:low" -p 3 -d "deploy.sh + systemd timer (land_trakker-deploy.timer, every 5 min) + deploys log + health surface. Requires VPS. Plan: docs/PLAN.md §17.")
opsbackup=$(bd create --silent --type=task --parent="$DEF" --title="ops-backup" -l "track:deferred,complexity:low" -p 3 -d "Nightly pg_dump + rotation (14d/8w/6m) + BACKUP_RESTORE.md + quarterly restore drill. Requires VPS. Plan: docs/PLAN.md §18.")
secretscfg=$(bd create --silent --type=task --parent="$DEF" --title="secrets-config" -l "track:deferred,complexity:high" -p 3 -d "Populate real keys in land_trakker.toml and wire real Mapbox geocoder + real Anthropic LLM impls behind the existing interfaces. Requires secrets. Plan: docs/PLAN.md §8,§16,§20.")
sknipe=$(bd create --silent --type=task --parent="$DEF" --title="scraper-knipe" -l "track:deferred,complexity:high" -p 3 -d "Real Knipe Land Co parser against captured fixtures + live verification. Plan: docs/PLAN.md §12.")
sicp=$(bd create --silent --type=task --parent="$DEF" --title="scraper-idaho-country" -l "track:deferred,complexity:high" -p 3 -d "Real Idaho Country Properties parser against captured fixtures. Plan: docs/PLAN.md §12.")
sfay=$(bd create --silent --type=task --parent="$DEF" --title="scraper-fay" -l "track:deferred,complexity:high" -p 3 -d "Real Fay Ranches parser against captured fixtures. Plan: docs/PLAN.md §12.")
swhite=$(bd create --silent --type=task --parent="$DEF" --title="scraper-whitetail" -l "track:deferred,complexity:high" -p 3 -d "Real Whitetail Properties parser against captured fixtures. Plan: docs/PLAN.md §12.")
shall=$(bd create --silent --type=task --parent="$DEF" --title="scraper-hall" -l "track:deferred,complexity:high" -p 3 -d "Real Hall and Hall parser against captured fixtures. Plan: docs/PLAN.md §12.")
slive=$(bd create --silent --type=task --parent="$DEF" --title="scraper-livewater" -l "track:deferred,complexity:high" -p 3 -d "Real Live Water Properties parser against captured fixtures. Plan: docs/PLAN.md §12.")
sacre=$(bd create --silent --type=task --parent="$DEF" --title="scraper-acretrader" -l "track:deferred,complexity:high" -p 3 -d "First auction scraper (AcreTrader) using the auction extension fields. Plan: docs/PLAN.md §12.")
slw=$(bd create --silent --type=task --parent="$DEF" --title="scraper-landwatch" -l "track:deferred,complexity:high" -p 3 -d "LandWatch scraper: conservative HTTP, escalate to chromedp if blocked, kill switch in source config. Plan: docs/PLAN.md §12, Part B Phase 3.")
sloa=$(bd create --silent --type=task --parent="$DEF" --title="scraper-landsofamerica" -l "track:deferred,complexity:high" -p 3 -d "Lands of America scraper; drop if redundant with LandWatch. Plan: docs/PLAN.md §12.")
pada=$(bd create --silent --type=task --parent="$DEF" --title="parcel-scraper-ada" -l "track:deferred,complexity:high" -p 3 -d "First automated parcel scraper for Ada County (Beacon template). Plan: docs/PLAN.md Part B Phase 4-5.")
sales=$(bd create --silent --type=task --parent="$DEF" --title="sales-idaho-beacon" -l "track:deferred,complexity:high" -p 3 -d "Recorded sales for Ada/Canyon/Kootenai/Bonner/Twin Falls via Beacon template. Plan: docs/PLAN.md Part B Phase 5.")
mtlist=$(bd create --silent --type=task --parent="$DEF" --title="montana-listings" -l "track:deferred,complexity:high" -p 3 -d "First Montana listing scrapers. Plan: docs/PLAN.md Part B Phase 5.")

echo "Wiring dependencies (bd dep add <id> <depends-on...>)..."
# Phase 0
bd dep add "$config" "$bootstrap"
bd dep add "$migrations" "$bootstrap"
bd dep add "$domain" "$bootstrap"
bd dep add "$parser" "$bootstrap"
bd dep add "$scraperiface" "$bootstrap"
bd dep add "$ratelimit" "$scraperiface"
bd dep add "$storage" "$migrations" "$domain"
bd dep add "$listingbus" "$domain"
bd dep add "$backfill" "$storage" "$listingbus"
bd dep add "$orch" "$ratelimit" "$listingbus" "$storage"
bd dep add "$fake" "$scraperiface"
bd dep add "$scraperd" "$orch" "$fake"
bd dep add "$scrapeonce" "$orch" "$fake"
bd dep add "$webshell" "$config"
bd dep add "$weblist" "$webshell" "$listingbus"
bd dep add "$e2e" "$scrapeonce" "$backfill" "$fake"
# Phase 1
bd dep add "$geocode" "$migrations" "$domain"
bd dep add "$normgeo" "$geocode" "$orch"
bd dep add "$searchf" "$listingbus" "$storage"
bd dep add "$searchmap" "$weblist" "$searchf"
bd dep add "$pricehist" "$weblist" "$listingbus"
bd dep add "$fixharness" "$parser" "$scraperiface"
# Phase 2
bd dep add "$savedsearch" "$searchf"
bd dep add "$websaved" "$searchmap" "$savedsearch"
bd dep add "$dedup" "$listingbus" "$geocode"
bd dep add "$webdup" "$weblist" "$dedup"
bd dep add "$auction" "$domain" "$migrations"
# Phase 3
bd dep add "$attrx" "$parser"
bd dep add "$attrwire" "$attrx" "$orch"
bd dep add "$fts" "$searchf" "$searchmap"
bd dep add "$health" "$webshell" "$listingbus"
bd dep add "$logging" "$webshell"
bd dep add "$adminsrc" "$health" "$backfill"
# Phase 4
bd dep add "$llm" "$config"
bd dep add "$llmfb" "$llm" "$attrx"
bd dep add "$attrui" "$searchmap" "$attrwire"
bd dep add "$parcels" "$migrations" "$domain"
bd dep add "$csv" "$parcels"
bd dep add "$match" "$parcels" "$geocode"
bd dep add "$parcelpanel" "$weblist" "$match"
# Phase 5
bd dep add "$compsq" "$parcels" "$match"
bd dep add "$compspanel" "$parcelpanel" "$compsq"
bd dep add "$mtimport" "$parcels"
# Deferred -> gates + build-track prereqs
bd dep add "$opsdeploy" "$GVPS"
bd dep add "$opsbackup" "$GVPS"
bd dep add "$secretscfg" "$GSEC" "$geocode" "$llm"
bd dep add "$sknipe" "$GFIX" "$fixharness"
bd dep add "$sicp" "$GFIX" "$fixharness"
bd dep add "$sfay" "$GFIX" "$fixharness"
bd dep add "$swhite" "$GFIX" "$fixharness"
bd dep add "$shall" "$GFIX" "$fixharness"
bd dep add "$slive" "$GFIX" "$fixharness"
bd dep add "$sacre" "$GFIX" "$auction"
bd dep add "$slw" "$GFIX"
bd dep add "$sloa" "$GFIX"
bd dep add "$pada" "$GFIX" "$parcels"
bd dep add "$sales" "$GFIX" "$parcels" "$compsq"
bd dep add "$mtlist" "$GFIX"

echo "P0_EPIC=$P0"
echo "P1_EPIC=$P1"
echo "DEFER_EPIC=$DEF"
echo "DONE"
