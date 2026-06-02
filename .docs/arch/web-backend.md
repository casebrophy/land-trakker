# Web Backend Architecture

## Foundation Layer: `foundation/web/`

### Auth & Session Management

**Session** (`session.go`) — HMAC-SHA256 signed token in HttpOnly cookie (name: `lt_session`). Signature uses a shared secret; no session store.

**IsAuthenticated(r, secret)** — validates request's session cookie against `signedToken(secret)`. Returns bool.

**SetSession(w, secret)** → creates signed cookie with SameSite=Lax, HttpOnly=true. No expiry (browser session).

**ClearSession(w)** → sets MaxAge=-1 to delete cookie.

**RequireAuth(secret)** — chi middleware that redirects unauthenticated requests to /login with 303 SeeOther.

### Health & System Monitoring

**HealthQuerier interface** (`health_dashboard.go`) — minimal contract: `QuerySources(ctx) []source.Source`, `QueryRecentRuns(ctx, sourceID, limit) []source.ScrapeRun`. Used by HealthDashboardHandler to fetch per-source metrics.

**HealthDashboardHandler(hq)** → GET /health (public). Queries hq for sources and recent 30 runs per source. Builds sourcePanelData (Name, LastRunAt, LastStatus, Sparkline of 30 days as status indicators, ErrorRate %). Reads runtime.MemStats for system metrics (Goroutines, AllocMB, SysMB, NumGC). Renders health.html. Returns partial data if hq is nil (graceful degradation).

**LogCapture** (`logs.go`) — circular buffer (capacity default 100) implementing slog.Handler. Handle() stores formatted log records (Message + Attrs); Recent(n) returns last N in reverse chronological order; Filter(query, n) returns matching entries (case-insensitive substring search, max 500 entries).

**LogsHandler(lc)** → GET /health/logs (public). Parses query param q (search query, case-insensitive) and limit (default 50, max 500). If q present, calls lc.Filter(q, limit); else lc.Recent(limit). Renders logs.html with results and search form. Returns empty list if lc is nil.

### Login Handlers

**LoginHandler(passwordHash, secret)** → GET renders embedded login.html template; POST validates bcrypt password hash against form field, calls SetSession on success with 303 redirect to /, or returns 401 + form on invalid password.

**LogoutHandler()** → calls ClearSession and redirects to /login with 303.

### Listing Handlers

**ListingsQuerier interface** — minimal contract: `QueryListings(ctx, limit, offset) []Listing`, `QueryListingByID(ctx, id) Listing`, `QuerySnapshotsByListing(ctx, id) []ListingSnapshot`, `QueryListingsFilter(ctx, f listing.ListingFilter, limit, offset) []Listing`. Callers inject via dependency.

**ListingsHandler(q)** → GET only. Supports HTMX partial requests (HX-Request header). Parses:
  - Pagination: limit (default 50, max 200), offset query params
  - Filter form: q (full-text search), acres_min/acres_max (float), price_min/price_max (int, converted to cents), counties (comma-separated), property_type, attr_water/attr_off_grid/attr_power/attr_well/attr_septic (checkboxes)
  
  Queries paginated listings with parseFilter (empty filter → QueryListings; non-empty → QueryListingsFilter). Builds listingRow objects (ID, Title, Status, PricePerAcre formatted as $X/acre, Acres, Location as City+State, FirstSeenDate formatted YYYY-MM-DD) and mapMarker objects from Geom (Lat, Lng, Title, ID) for Leaflet. Renders listings.html (full page) on initial load or listings_results.html (partial template "results_content") on HTMX requests. Returns 503 if q is nil, 500 on query error.

**ListingDetailHandler(q)** → GET with `{id}` chi URL param. Queries single listing by ID (404 if "no rows"), queries snapshots by ID (500 on error). Builds timelineData from snapshots (date, price, acres) for Chart.js dual-axis timeline. Maps listing fields to HTML (Title, URL, Status, Price formatted $X, Acres, Address as line+city+state). Maps snapshots to rows (CapturedAt formatted YYYY-MM-DD HH:MM, Status, Price, Acres, Diff as comma-joined keys from s.Diff map). Renders listing_detail.html with timeline JSON (serialized as template.JS). Returns 500 on query errors.

### Searches Handlers (Phase 2)

**SearchCore interface** — minimal contract for search operations:
- `QuerySavedSearches(ctx) []SavedSearch` — list all saved searches
- `QuerySavedSearchByID(ctx, id) SavedSearch` — fetch one search by ID
- `CreateSavedSearch(ctx, ss) SavedSearch` — create new saved search (returns created entity with ID)
- `UpdateSavedSearch(ctx, ss) SavedSearch` — update existing search
- `DeleteSavedSearch(ctx, id) error` — delete by ID
- `QueryUnseen(ctx, limit) []SearchHit` — fetch unseen search hits for digest
- `MarkHitsSeen(ctx, ids) error` — batch mark hits as seen

**SearchesHandler(sc)** → GET /searches. Lists all saved searches. Queries sc, builds savedSearchRow objects (ID, Name, Enabled, CreatedAt formatted YYYY-MM-DD, FilterSummary as compact filter description), renders searches.html full page. Returns 503 if sc is nil.

**SearchesNewHandler()** → GET /searches/new. Renders search_form.html with empty form (IsEdit=false, ActionURL="/searches", default values). No service dependency.

**SearchesCreateHandler(sc)** → POST /searches. Parses form: name (required), enabled checkbox, filter fields (acres_min/max, price_min/max in dollars, counties, property_type, attr_water/attr_off_grid/attr_power/attr_well/attr_septic). Converts filter to ListingFilter (price in cents), calls sc.CreateSavedSearch(), redirects to /searches on success. Returns 422 if name missing.

**SearchesEditHandler(sc)** → GET /searches/{id}. Queries search by ID, builds filterForm from ListingFilter (reverse of parseFilterFromForm), renders search_form.html with IsEdit=true, ActionURL="/searches/{id}". Returns 404 if not found.

**SearchesUpdateHandler(sc)** → POST /searches/{id}. Parses form (same fields as create), calls sc.UpdateSavedSearch(), redirects to /searches on success.

**SearchesDeleteHandler(sc)** → POST /searches/{id}/delete. Calls sc.DeleteSavedSearch(id), redirects to /searches.

### Digest Handler (Phase 2)

**DigestHandler(sc, lq)** → GET /digest. Shows unseen search hits from today. Queries sc.QueryUnseen(200). For each hit, looks up SavedSearch name; if lq available, enriches with listing details (title, price formatted cents, acres). Builds digestHitRow (HitID, ListingID, ListingTitle, ListingPrice, ListingAcres, ListingURL, SearchName, HitAt formatted YYYY-MM-DD, Reason as human label from HitReason enum). Renders digest.html with hit rows, comma-separated hit IDs for batch mark-seen form.

**DigestMarkSeenHandler(sc)** → POST /digest/mark-seen. Parses form field hit_ids (comma-separated int64 list), calls sc.MarkHitsSeen(ids), redirects to /digest.

### Duplicates Handlers (Phase 2)

**DuplicatesQuerier interface** — minimal contract:
- `QueryPossibleDuplicates(ctx, decision *string) []PossibleDuplicate` — query possible duplicate pairs (decision=nil filters to undecided)
- `UpdateDuplicateDecision(ctx, aID, bID, decision) error` — record reviewer decision (same/different/dismiss)
- `QueryListingByID(ctx, id) Listing` — fetch listing details for enrichment

**DuplicatesHandler(dq)** → GET /duplicates. Queries dq.QueryPossibleDuplicates(nil) to show only undecided pairs. For each pair, queries listing A and B details, builds duplicatePairRow (ListingAID, ListingATitle, ListingAPrice formatted cents, ListingAURL, ListingAAddr; same for B; ScorePercent as 0-100 float, Reasons as comma-joined dedup reason labels). Renders duplicates.html with pairs table. Returns 503 if dq is nil.

**DuplicatesUpdateHandler(dq)** → POST /duplicates/decision. Parses form: action (same/different/dismiss), a_id, b_id. Validates IDs present, maps action to decision string, calls dq.UpdateDuplicateDecision(aID, bID, decision), redirects to /duplicates.

### Admin: Sources & Backfill

**AdminSourcesQuerier interface** (`admin_sources.go`) — minimal contract: `QuerySources(ctx) []source.Source`, `QueryRecentRuns(ctx, sourceID, limit) []source.ScrapeRun`, `CountBackfillEligible(ctx, sourceID) int`.

**AdminSourcesUpdater interface** (`admin_sources.go`) — minimal contract: `QuerySourceByID(ctx, id) source.Source`, `UpdateSource(ctx, src) error`.

**BackfillTrigger interface** (`admin_sources.go`) — minimal contract: `TriggerBackfill(sourceID)`.

**AdminSourcesHandler(asq)** → GET /admin/sources (authenticated). Queries asq for all sources and recent 5 runs per source; counts backfill-eligible listings per source. Builds adminSourcePanel (ID, DisplayName, BaseURL, Enabled, RateLimitMS, Concurrency, AbsenceDaysBeforeStale, AbsenceDaysBeforeInactive, ConsecutiveMissedRunsThreshold, MinResultRatioForInactivation formatted "%.3f", LastRunAt as timeAgo, RecentRuns table rows, EligibleCount, EligibleAvailable flag). Renders admin_sources.html with flash query param (e.g., "saved", "backfill_started"). Returns 503 if asq is nil.

**AdminSourcesUpdateHandler(asu)** → POST /admin/sources/{id} (authenticated). Parses form: absence_days_stale, absence_days_inactive, consecutive_missed, min_ratio, rate_limit_ms, concurrency (all int or float), enabled (checkbox). Queries source by ID, updates fields, calls asu.UpdateSource(), redirects to /admin/sources?flash=saved. Returns 404 if source not found, 500 on update error.

**AdminSourcesBackfillHandler(bt)** → POST /admin/sources/{id}/backfill (authenticated). Parses URL param {id}, calls bt.TriggerBackfill(id) if bt is non-nil, redirects to /admin/sources?flash=backfill_started.

### Helpers

**formatCents(cents)** → converts int64 cents to string "$X,XXX" format (dollars with commas, no cents precision).

**addCommas(s)** — utility to insert thousands separators in digit strings.

**filterSummary(f)** → builds compact filter description (e.g., "acres≥20; price≤500000; counties: Ada, Valley") from ListingFilter.

**filterToFormData(f)** → reverses filter to filterForm struct for pre-populating edit forms (converts cents to dollars, pointers to strings).

**parseFilterFromForm(r)** → parses ListingFilter from POST form values (converts dollars to cents, comma-separated counties to slice).

**hitReasonLabel(r)** → maps search.HitReason enum (ReasonNew, ReasonPriceDrop, ReasonAttributeAdded) to human-readable labels.

**formatReasons(reasons)** → converts dedup reason codes (listing.DedupReasonGeo, DedupReasonAcres, etc.) to comma-joined labels.

**formatAddress(l)** → builds formatted address string from listing address line, city, county.

### Templates

Embedded via `//go:embed templates/`. All templates parsed at package init:
- `login.html` — form with password input, displays {{.Error}} on POST failure
- `listings.html` — full page: search filter form (acres range, price range, counties, property type, 5 boolean attributes) with HTMX; Leaflet map (OSM tile layer, center 44.5/-114.0 zoom 6); results div (swapped by HTMX)
- `listings_results.html` — define "results_content": table of {{.Rows}} with HTMX pagination links; embedded script calls updateMapMarkers({{.Markers}}) to refresh map after swap
- `listing_detail.html` — detail view with {{.ID}}, {{.Title}}, {{.URL}}, {{.Status}}, {{.Price}}, {{.Acres}}, {{.Address}}; Chart.js dual-axis timeline ({{.Timeline}} JSON: points array with date, price, acres); snapshot history table (CapturedAt, Status, Price, Acres, Diff)
- `searches.html` (Phase 2) — lists saved searches in table ({{.Searches}} rows: ID, Name, Enabled, CreatedAt, FilterSummary); links to /searches/new, /searches/{id}/edit, /searches/{id}/delete
- `search_form.html` (Phase 2) — reusable form for create/edit ({{.IsEdit}}, {{.ActionURL}}, {{.ID}}, {{.Name}}, {{.Enabled}}, {{.Filter}} struct with filter fields); submit creates/updates via SearchesCreateHandler or SearchesUpdateHandler
- `digest.html` (Phase 2) — daily digest page showing unseen search hits table ({{.Hits}} rows: HitID, ListingID, ListingTitle, ListingPrice, ListingAcres, ListingURL, SearchName, HitAt, Reason); form to POST /digest/mark-seen with comma-separated hit IDs
- `duplicates.html` (Phase 2) — duplicate review queue table ({{.Pairs}} rows: ListingA/B ID, Title, Price, URL, Address; ScorePercent, Reasons); decision form (same/different/dismiss) per pair POSTs to /duplicates/decision

---

## API Layer: `cmd/api/`

### Server Setup (`main.go`)

**Config** — loaded from $CONFIG_PATH or land_trakker.toml (defaults: listen ":8080"). Contains Server.Listen, Server.AdminPasswordHash, Server.SessionSecret.

**HTTP Server** — chi router from `newRouter(cfg, q)` with middleware: RealIP, Recoverer. Timeouts: read 15s, write 30s, idle 60s. Graceful shutdown waits 10s on SIGINT/SIGTERM.

**Dependency Injection** — `newRouter` takes cfg and q (ListingsQuerier). q is nil in main.go (intent: injected upstream by harness).

### Route Wiring (`routes.go`)

```
GET  /health                     → HealthDashboardHandler(hq) [public]
GET  /health/logs                → LogsHandler(lc) [public]
GET  /login, POST /login         → LoginHandler(cfg.Server.AdminPasswordHash, cfg.Server.SessionSecret)
GET  /logout                     → LogoutHandler()

Group {
  Middleware: RequireAuth(cfg.Server.SessionSecret)
  GET  /                         → ListingsHandler(q)
  GET  /listings/{id}            → ListingDetailHandler(q)

  GET  /searches                 → SearchesHandler(sc)
  GET  /searches/new             → SearchesNewHandler()
  POST /searches                 → SearchesCreateHandler(sc)
  GET  /searches/{id}/edit       → SearchesEditHandler(sc)
  POST /searches/{id}            → SearchesUpdateHandler(sc)
  POST /searches/{id}/delete     → SearchesDeleteHandler(sc)

  GET  /digest                   → DigestHandler(sc, q)
  POST /digest/mark-seen         → DigestMarkSeenHandler(sc)

  GET  /duplicates               → DuplicatesHandler(dq)
  POST /duplicates/decision      → DuplicatesUpdateHandler(dq)

  GET  /admin/sources            → AdminSourcesHandler(asq)
  POST /admin/sources/{id}       → AdminSourcesUpdateHandler(asu)
  POST /admin/sources/{id}/backfill → AdminSourcesBackfillHandler(bt)
}
```

Public: /health, /health/logs, /login routes. Authenticated group protects: listings, searches, digest, duplicates, admin/sources. newRouter signature: `newRouter(cfg, q, sc, dq, hq, lc, asq, asu, bt)` injects ListingsQuerier, SearchCore, DuplicatesQuerier, HealthQuerier, LogCapture, AdminSourcesQuerier, AdminSourcesUpdater, BackfillTrigger as dependencies.

---

## Data Flow

1. **GET /health** (public, no auth) → HealthDashboardHandler queries HealthQuerier for sources and recent runs; renders health.html with source panels (sparkline, error rate) and system stats.
2. **GET /health/logs** (public, no auth) → LogsHandler parses query param q and limit; searches LogCapture (or returns recent entries); renders logs.html with results.
3. **Unauthenticated request** → RequireAuth redirects to /login
4. **Login POST** → bcrypt compare; on success, SetSession + redirect to /
5. **GET /** (initial load) → ListingsHandler detects no HTMX header; queries listings (empty filter); builds rows + markers; renders full listings.html (map + filter form + results div)
6. **GET /** (HTMX filter/paginate) → ListingsHandler detects HX-Request header; queries with parseFilter(r) including optional q param for full-text search; builds rows + markers; renders listings_results.html partial "results_content"; browser runs embedded updateMapMarkers(markers) script via htmx:afterSettle event
7. **GET /listings/{id}** → ListingDetailHandler queries Listing + ListingSnapshot history; builds timelineData (points array: date, price, acres); renders listing_detail.html; Chart.js initializes on page load with dual-axis line chart (price on left Y, acres on right Y)
8. **GET /searches** (Phase 2) → SearchesHandler queries sc.QuerySavedSearches(); builds savedSearchRow list (filterSummary computed); renders searches.html full page
9. **GET /searches/new** (Phase 2) → SearchesNewHandler renders search_form.html with empty form
10. **POST /searches** (Phase 2) → SearchesCreateHandler parses form (name, enabled, filter), converts filter to ListingFilter (cents), calls sc.CreateSavedSearch(), redirects to /searches
11. **GET /searches/{id}/edit** (Phase 2) → SearchesEditHandler queries sc.QuerySavedSearchByID(id), builds filterForm from ListingFilter (reverse conversion: cents to dollars), renders search_form.html with IsEdit=true for pre-population
12. **POST /searches/{id}** (Phase 2) → SearchesUpdateHandler parses form, calls sc.UpdateSavedSearch(id, ...), redirects to /searches
13. **POST /searches/{id}/delete** (Phase 2) → SearchesDeleteHandler calls sc.DeleteSavedSearch(id), redirects to /searches
14. **GET /digest** (Phase 2) → DigestHandler queries sc.QueryUnseen(200); builds searchNames map; for each hit, if lq available, enriches with listing details; renders digest.html with digestHitRow objects and comma-separated hit IDs
15. **POST /digest/mark-seen** (Phase 2) → DigestMarkSeenHandler parses hit_ids form field (comma-split, trim), calls sc.MarkHitsSeen(ids), redirects to /digest
16. **GET /duplicates** (Phase 2) → DuplicatesHandler queries dq.QueryPossibleDuplicates(nil); for each pair, queries listing A + B details (price, title, address), builds duplicatePairRow; renders duplicates.html with pair table
17. **POST /duplicates/decision** (Phase 2) → DuplicatesUpdateHandler parses form (action: same/different/dismiss, a_id, b_id), maps action to decision string, calls dq.UpdateDuplicateDecision(aID, bID, decision), redirects to /duplicates
18. **GET /admin/sources** (Phase 3) → AdminSourcesHandler queries asq for all sources and recent 5 runs, counts backfill-eligible per source, renders admin_sources.html with config panels and run history.
19. **POST /admin/sources/{id}** (Phase 3) → AdminSourcesUpdateHandler parses form (rate_limit_ms, concurrency, absence thresholds, min_ratio, enabled flag), updates source, redirects to /admin/sources?flash=saved.
20. **POST /admin/sources/{id}/backfill** (Phase 3) → AdminSourcesBackfillHandler extracts URL param {id}, calls bt.TriggerBackfill(id), redirects to /admin/sources?flash=backfill_started.
21. **Logout** → ClearSession + redirect to /login

---

## Key Behaviors

1. **Session** is stateless signed token (no DB lookup per request). Secret is shared credential.
2. **Password** only checked at POST /login; subsequent requests validated by cookie signature alone.
3. **Pagination** defaults to 50, max 200 per query param. HTMX requests preserve filter state in URLs (hx-push-url="true").
4. **Filtering** — parseFilter converts URL query params to listing.ListingFilter struct (price in dollars, converted to cents for DB query; acres as float; counties as comma-separated string parsed to []string; booleans set only if "true"). isFilterEmpty checks all fields to decide QueryListings vs. QueryListingsFilter.
5. **Map** — Leaflet with OSM tiles; markers from listing Geom fields (Lat, Lng); HTMX swap event triggers updateMapMarkers(data) to refresh pins and fitBounds around visible listings.
6. **Timeline** — Chart.js dual-axis: price (left Y, blue line) and acres (right Y, green line) over snapshot dates. Price values formatted as $M/$K in ticks; null/zero prices filtered out per snapshot.
7. **Formatting** — prices in cents stored in domain, rendered as "$X,XXX" in UI; acres as float 2 decimals; dates as "2006-01-02" or "2006-01-02 15:04".
8. **Snapshots** show field-level Diff map (e.g., `{"price_cents": {"old": 500000, "new": 490000}}`); UI renders keys as comma-joined string.
9. **Nil checks** on optional fields (Title, City, State, Price, Acres, Address, Geom) before use; safe fallbacks ("n/a", "(untitled)").
10. **HTMX integration** — filter form uses hx-get + hx-target="#results" + hx-push-url="true"; pagination links also HTMX-enabled; htmx:afterSettle event handler runs embedded script to update map markers.
11. **SearchCore interface** (Phase 2) — injected dependency for saves searches and digest. All CRUD + QueryUnseen/MarkHitsSeen. Handlers check nil and return 503 if missing.
12. **Saved searches** (Phase 2) — store a ListingFilter (acts as a saved query). FilterSummary built on-the-fly from filter (compact label). Edit form reverses filter fields to populate form. Price stored in dollars on form submission but converted to cents for ListingFilter.
13. **Digest** (Phase 2) — shows unseen SearchHit records from QueryUnseen(200). Hit enrichment depends on lq availability; if nil, shows only hit metadata. SearchName lookup deduplicates by cached query. MarkHitsSeen batch-updates, then redirects back to /digest.
14. **DuplicatesQuerier interface** (Phase 2) — injected dependency for dedup review. QueryPossibleDuplicates(nil) filters to undecided pairs. UpdateDuplicateDecision accepts action string (same/different/dismiss) mapped to decision column.
15. **Duplicates decision** (Phase 2) — form submits action + a_id + b_id. Handler validates IDs present, maps action, updates both listings' duplicate_decision column. On reload, QueryPossibleDuplicates(nil) excludes decided pairs.
16. **HealthQuerier interface** (Phase 3) — injected dependency for system health dashboard. QuerySources returns all configured sources; QueryRecentRuns returns recent N runs per source. HealthDashboardHandler gracefully degrades if nil.
17. **LogCapture** (Phase 3) — circular slog.Handler buffer implementing Enabled/Handle/WithAttrs/WithGroup. LogsHandler calls Recent(n) or Filter(query, n) and renders logs.html. Supports full-text search on captured logs.
18. **AdminSourcesQuerier/Updater interfaces & BackfillTrigger** (Phase 3) — injected dependencies for source config. AdminSourcesHandler displays source configuration and recent run history. AdminSourcesUpdateHandler modifies per-source thresholds (absence days, consecutive misses, result ratio, rate limits, concurrency). AdminSourcesBackfillHandler triggers background backfill via BackfillTrigger.TriggerBackfill(id).
19. **Full-text search** (Phase 3) — ListingFilter.FullText field (optional string pointer). parseFilter extracts query param "q" and sets filter.FullText. isFilterEmpty checks for nil. ListingsHandler passes FullText to QueryListingsFilter for full-text matching in listings.
