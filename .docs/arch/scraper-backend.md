# Scraper Backend Architecture

## Overview

The **scraper** domain (`foundation/scraper/`) orchestrates the full listing discovery→fetch→parse→normalize→upsert→snapshot pipeline for land broker sources. It plugs multiple concrete broker implementations (e.g., `FakeBroker`) and coordinates them with business-layer cores to ingest raw real estate data.

---

## Core Types

### Scraper Interface
```go
type Scraper interface {
    Source() Source
    ParserVersion() string
    Discover(ctx context.Context) ([]ListingRef, error)
    Fetch(ctx context.Context, ref ListingRef) (RawFetch, error)
    Parse(raw RawFetch) (ParsedListing, error)
}
```
- **Discover**: fetch listing IDs from broker (search results page)
- **Fetch**: retrieve full HTML/JSON for a single listing
- **Parse**: extract structured fields into `ParsedListing`

### Source & Listing Models
```go
type Source struct {
    ID, DisplayName, BaseURL, UserAgent string
    RateLimit time.Duration
    Concurrency int
    RespectRobots, Enabled bool
    AbsenceDaysBeforeStale, AbsenceDaysBeforeInactive int
    ConsecutiveMissedRunsThreshold int
    MinResultRatioForInactivation float64
}

type ListingRef struct {
    SourceListingID string
    URL string
    Summary map[string]any
}

type RawFetch struct {
    SourceID, SourceListingID, URL string
    FetchedAt time.Time
    StatusCode int
    ContentType string
    Body []byte
    Headers http.Header
}

type ParsedListing struct {
    SourceID, SourceListingID, URL, Title, Description string
    PriceCents *int64
    Acres *float64
    Address *Address
    County, State *string
    Photos []string
    Broker *Broker
    StructuredAttrs map[string]any
    PostedAt, UpdatedAt *time.Time
    SourceStatus string
    AuctionEndDate *time.Time     // optional auction end time
    AuctionCurrentBid *int64      // optional current bid (cents)
    AuctionReserve *int64         // optional reserve price (cents)
}
```

---

## Handler Layer: Orchestrator

### **RunOnce** Pipeline
```
Discover → DiffRefs → Fetch(TTL) → Parse → UpsertFromParsed → RecordParseAttempt
```

**Orchestrator** (`orchestrator.go`) wires `Scraper` with two business cores:
- **sourceCore**: manages `ScrapeRun`, `RawFetch`, source metadata
- **listingCore**: upserts `Listing`, `ListingSnapshot`, parse attempts

**Key responsibilities:**
1. **Discover** listings via `Scraper.Discover()`
2. **DiffRefs** (added/kept/removed) against `prevDiscovered` cache
3. **TTL check** per listing; skip fetch if fresh (`ttlExpired`)
4. **Fetch** added + expired listings via `Scraper.Fetch()`
5. **Parse** raw HTML/JSON via `Scraper.Parse()`
6. **Upsert** parsed data into listing via `listingCore.UpsertFromParsed()`
7. **Record parse attempts** (success/error) with snapshot link or error message
8. **Missed-run handler** for disappeared listings (stale/inactive logic)

**Run Lifecycle:**
- Call `sourceCore.StartRun()` → receive `ScrapeRun` with ID
- Update run counts: Discovered, Fetched, Parsed, Errors
- Determine status: OK (no errors) → Partial (some errors) → Failed (fatal error)
- Call `sourceCore.FinishRun()` in defer block

**Missed-Run Logic:**
- If `removed.len > 0` and `missedHandler != nil`: call handler for each disappeared listing
- Handler eligibility checked via `orchIsRunHealthy()` (mirrors `sourcebus.IsRunHealthy`)
  - Requires: prior run exists, prior discovered count ≥ current × `MinResultRatioForInactivation`

**Type Conversions:**
- `scraperRawToSource()`: adds SHA256 body hash, serializes headers to JSON

---

## Support Components

### RateLimiter (Wrapper Pattern)
```go
type RateLimiter struct {
    wrapped Scraper
    clock Clock
    lastFetchTime time.Time
}
```
- **Wraps** any `Scraper` to enforce rate limits on `Fetch()`
- **Jitter**: 0–500ms on top of `Source.RateLimit`
- **Exponential backoff retry**: up to 3 attempts on transient errors
  - Backoff: 100ms → 200ms → 400ms
- Delegates `Discover`, `Parse`, `Source` directly

### DiffRefs (Utility)
```go
func DiffRefs(prev, curr []string) (added, kept, removed []string)
```
- O(n) set-based diffing: identifies new, retained, or disappeared listing IDs
- Used to decide which listings to fetch (added always, kept only if TTL expired)

### FakeBroker (In-Repo Test Stub)
- 3 deterministic listings (Ada, Gem, Owyhee counties, ID)
- `Discover()`: returns 3 `ListingRef`
- `Fetch()`: returns hardcoded HTML bodies
- `Parse()`: extracts mock fields (title, price, address, broker)

### TestFixtures (Phase 6 Fixture Test Harness)
```go
type FixtureResult struct {
    Name    string  // fixture filename without extension
    Matched bool    // true if Parse output matches expected JSON
    Error   string  // error message if mismatch
}

func TestFixtures(t *testing.T, scraper Scraper) []FixtureResult
```
- **Load mechanism**: scans `testdata/<sourceID>/` (relative to test execution CWD) for paired `*.html` + `*.json` files (e.g., `foundation/scraper/testdata/fakebroker/fixture-1.html`, `fixture-1.json`)
- **Execution**: reads HTML as `RawFetch.Body`, loads JSON as expected `ParsedListing`, calls `Parse()`, compares via JSON marshal round-trip
- **Results**: returns `[]FixtureResult` in lexicographic order by fixture name; failures show detailed mismatches
- **Auto-discovery**: TestFixtures() auto-discovers all fixture pairs without explicit registration; fixtures are sorted and tested in order

---

## Dependencies

### Inbound (Called by Orchestrator)

**`business/sdk/sourcebus.Core`** (via `sourceCore` interface)
- `QuerySource(sourceID)` → `source.Source` (config, enabled flag)
- `QueryLatestRun(sourceID)` → prior `ScrapeRun` (for health check, stale thresholds)
- `StartRun(sourceID, now)` → new `ScrapeRun` with ID
- `FinishRun(run)` → persist counts, status, finished_at
- `CreateRawFetch(rf)` → store HTTP body + SHA256 + headers
- `QueryRawFetchesByListing(sourceID, sourceListingID)` → check TTL expiry

**`business/sdk/listingbus.Core`** (via `listingCore` interface)
- `UpsertFromParsed(pl, rawFetchID, now)` → create/update `Listing`, emit `ListingSnapshot`
- `RecordParseAttempt(pa)` → log parser version, outcome, error, snapshot link
- `QueryListingBySource(sourceID, sourceListingID)` → needed by missed-run handler

**`foundation/parser`** (Consumer use case, not a hard dep)
- `Scraper.Parse()` implementations may delegate to parser subdomain

### Outbound (Provided by Implementations)

- Concrete `Scraper`: custom `Discover()`, `Fetch()`, `Parse()` per broker
- `MissedRunHandler`: callback for disappeared listings (wired by application layer)

---

## Data Flow (Happy Path)

```
Discover
 ↓
 DiffRefs(prev, discovered) → added, kept, removed
 ↓
 For each (added + kept-with-expired-TTL):
   Fetch → RawFetch + CreateRawFetch → rawFetchID
   ↓
   Parse → ParsedListing
   ↓
   UpsertFromParsed + RecordParseAttempt(success, snapshotID)
 ↓
 For each removed (if healthy run):
   MissedHandler → mark stale/inactive
 ↓
 FinishRun(counts, status, finished_at)
```

---

## Error Handling

- **Discover fails**: return early, mark run as Failed
- **Fetch fails**: log warn, skip parse, increment error count
- **Parse fails**: record `ParseAttempt` with `OutcomeParserError`, skip upsert
- **Upsert fails**: log warn, record parse attempt without snapshot link
- **FinishRun fails**: log warn (run still persists, may be partial)
- **Missed-run handler fails**: log warn, continue to next

All errors are non-fatal; run completes with error counts and Partial/Failed status.

---

## Test Coverage

- **`scraper_test.go`**: Scraper interface, RateLimiter behavior (transient retry, jitter)
- **`orchestrator_test.go`**: RunOnce pipeline, diff logic, TTL, missed-run handler, parse errors
- **`rate_limiter_test.go`**: backoff timing, jitter bounds
- **`fixture_test.go`**: Phase 6 fixture test harness; tests TestFixtures() behavior, fixture discovery, JSON comparison (NEW)
- **`fakebroker_test.go`**: deterministic fixture behavior; may use TestFixtures() to validate against testdata/fakebroker/ fixtures

---

## Fixture Test Harness Impact

### Changing ParsedListing Fields
Any field added, removed, or renamed in `ParsedListing` requires:
1. **Fixture JSON updates**: every `testdata/<sourceID>/fixture-N.json` must include/exclude the field
2. **Parse() implementations**: each Scraper must extract the field (or omit if optional)
3. **TestFixtures() re-run**: JSON round-trip comparison will fail until fixture JSON matches new shape
4. **Fixture regeneration**: use a debug script or manual Parse() to generate corrected fixture JSON

Example: Renaming `PriceCents` → `PriceUSD` breaks all fixture tests; TestFixtures() emits mismatches showing expected (old) vs actual (new) JSON.

### Adding New Fixtures
1. Save raw HTML to `testdata/<sourceID>/fixture-N.html`
2. Run scraper's Parse() on that HTML, capture the resulting ParsedListing
3. Marshal to JSON and save as `testdata/<sourceID>/fixture-N.json`
4. TestFixtures() auto-discovers the pair and tests it on next run

### Fixture Ownership
- Fixtures live in `testdata/<sourceID>/` alongside test code
- Scraper implementation authors are responsible for maintaining fixture pairs
- Fixture JSON is the golden reference; Parse() implementation should match

---

## Phase 2: Auction Fields Impact (ParsedListing)

### ⚠ AuctionEndDate, AuctionCurrentBid, AuctionReserve
Added in Phase 2 (`foundation/scraper/scraper.go:80-83`); all three fields are optional pointers to support non-auction listings.

Changing or adding to these fields requires:
1. **Fixture JSON updates**: every `testdata/<sourceID>/fixture-N.json` must include/omit these fields (optional, all null if not present)
2. **Parse() implementations**: each Scraper.Parse() must extract auction dates/prices if broker exposes them (or set to nil)
3. **TestFixtures() re-run**: JSON round-trip will fail if fixture JSON shape drifts from Parse() output
4. **Downstream consumers**: `listingbus.UpsertFromParsed()` and `listingCore` handle these as optional nullable fields; storage layer must support NULL auction columns

---

## Known Drift / Future Work

1. **Circular dependency avoidance**: `orchIsRunHealthy()` duplicates `sourcebus.IsRunHealthy` logic to prevent import cycle
2. **Transient error classification**: `isTransientError()` currently treats all errors as transient; real implementation should check HTTP 5xx, 429, net.Error timeouts
3. **Concurrency**: `Source.Concurrency` field defined but not enforced in Orchestrator; RateLimiter is single-threaded
4. **Parser integration**: Orchestrator hard-depends on `Scraper.Parse()` returning `ParsedListing`; no pluggable parser delegation yet
5. **Content-Type validation**: `RawFetch.ContentType` stored but not validated before Parse
6. **Fixture testdata location**: currently `testdata/<sourceID>/` at repo root; may move under `foundation/scraper/` or per-source packages in future
