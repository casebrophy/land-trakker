# Listing Backend Architecture

## Domain Layer: `business/domain/listing/`

### Core Types

**Listing** — canonical land listing record with UUID, source identity, state machine fields, parsed fields (title, price, acres, address, broker), structured attributes (attr_* fields + attrs_extraction map for detailed extraction results), and computed fields (price_per_acre_cents).

**ListingStatus** — enum: `active`, `stale`, `presumed_inactive`, `confirmed_sold`, `withdrawn`. State transitions in Core.ApplyMissedRun and UpsertFromParsed.

**ListingSnapshot** — point-in-time capture (RawFetchID, PriceCents, Acres, Title, StructuredAttrs). Includes Diff map vs previous snapshot (5-field diffs: price_cents, acres, title, description, status).

**PriceChange** — detected price movement (OldPriceCents, NewPriceCents, SnapshotID). Created in UpsertFromParsed when price differs between snapshots.

**ParseAttempt** — single parse attempt record (RawFetchID, ParserVersion, Outcome, ErrorMessage, SnapshotID).

**ListingFilter** — optional search constraints with nil-means-no-constraint semantics (Phase 1 + Phase 3):
```
type ListingFilter struct {
  AcresMin, AcresMax *float64
  PriceMin, PriceMax *int64 (cents)
  Counties []string
  PPAMin, PPAMax *int64 (price_per_acre_cents)
  PropertyType *string
  FullText *string                               // Phase 3: full-text search query on title+description
  AttrWaterFrontage, AttrOffGrid, AttrPower, AttrWell, AttrSeptic *bool
}
```

**AuctionInfo** — optional auction details:
```
type AuctionInfo struct {
  EndDate    *time.Time
  CurrentBid *int64  // cents
  Reserve    *int64  // cents
}
```

**DedupConfig** — thresholds for duplicate detection:
```
type DedupConfig struct {
  GeoMaxKM       float64
  AcresMaxDelta  float64
  PriceMaxDelta  float64
  ScoreThreshold float64
}
```
**DefaultDedupConfig()** returns defaults: GeoMaxKM=10km, AcresMaxDelta/PriceMaxDelta=0.20, ScoreThreshold=0.40.

**ScorePair** — scoring function `(a, b Listing, cfg DedupConfig) → (float64, []string)` computes similarity score (0.0–1.0) and matching reason strings from {geo, acres, price, broker, title}.

**PossibleDuplicate** — represents a scored duplicate pair:
```
type PossibleDuplicate struct {
  ListingAID   string     // UUID
  ListingBID   string     // UUID (always ListingAID < ListingBID)
  Score        float64    // 0.0–1.0
  Reasons      []string   // from {DedupReasonGeo, DedupReasonAcres, DedupReasonPrice, DedupReasonBroker, DedupReasonTitle}
  DetectedAt   time.Time
  UserDecision *string    // user override: "duplicate" | "false_positive" | nil
}
```

**Dedup reason constants** — declared in listing package:
```
const (
  DedupReasonGeo    = "geo"
  DedupReasonAcres  = "acres"
  DedupReasonPrice  = "price"
  DedupReasonBroker = "broker"
  DedupReasonTitle  = "title"
)
```

**Storer interface** — persistence contract:
- Listing CRUD: CreateListing, UpdateListing, QueryListingByID, QueryListingBySource, QueryListings, QueryListingsFilter
- Snapshot: CreateSnapshot, QuerySnapshotsByListing  
- PriceChange: CreatePriceChange, QueryPriceChangesByListing
- ParseAttempt: CreateParseAttempt, QueryEligibleRawFetchIDs
- Dedup: QueryListingsForDedup, UpsertPossibleDuplicate, QueryPossibleDuplicates, UpdateDuplicateDecision

**Implementation**: `storage/listingdb.Store` (PostgreSQL with pgx).

---

## Service Layer: `business/sdk/listingbus/`

### Core

**Dependency**: Storer interface + slog.Logger.

**Key Methods**:
- **UpsertFromParsed**(ParsedListing, rawFetchID, now) → creates/updates Listing, snapshots with diffs, detects price changes; includes geocoding call via geocodeAndApply and attribute extraction via applyAttributeExtraction (Phase 3)
- **ApplyMissedRun**(listingID, MissedRunConfig, runHealthy, now) → health-gated status transitions (active→stale→presumed_inactive) based on consecutive misses & absence days
- **QueryListings, QueryListingByID, QueryListingBySource** → Listing lookups with error wrapping
- **QueryListingsFilter**(ListingFilter, limit, offset) → filtered Listing lookups with ListingFilter constraints (Phase 1 + Phase 3 FullText full-text search on title+description using PostgreSQL to_tsvector/plainto_tsquery)
- **QuerySnapshotsByListing, QueryPriceChangesByListing** → history queries
- **RecordParseAttempt** → writes parse_attempts rows
- **geocodeAndApply**(ctx, listing) → calls geocoder with address components, handles daily limit + other errors gracefully (Phase 1)
- **RunDedup**(ctx, DedupConfig, now) → fetches active/stale listings grouped by county, pairs them via ScorePair, upsets PossibleDuplicates above score threshold (Phase 2)

### Internal Logic

**derivedStatus** — computes status for existing listing: sold/under-contract/withdrawn sources set terminal states; presumed_inactive/stale reappearances reset to active.

**diffSnapshots** — 5-field snapshot diff (price_cents, acres, title, description, status) in format `{"field": {"old": ..., "new": ...}}`.

**applyParsedFields** — copies ParsedListing fields (address, photos, broker, structured attrs) onto domain Listing.

**applyAttributeExtraction** (Phase 3) — runs deterministic attribute extractors on listing title+description using foundation/attrs package; populates attr_* boolean/string fields (AttrWaterFrontage, AttrOffGrid, AttrRoadAccess, AttrPower, AttrWell, AttrSeptic, AttrPropertyType) and stores detailed extraction results in AttrsExtraction map for audit/debugging.

**MissedRunConfig** — source-level thresholds: AbsenceDaysBeforeStale (14), AbsenceDaysBeforeInactive (30), ConsecutiveMissedRunsThreshold (3).

---

## State Machine

```
[Active] ──── threshold misses + N days absence ──→ [Stale]
   ↑          ──── threshold misses + 30+ days absence ──→ [PresumedInactive]
   │
   └─────────────────── reappears ─────────────────────┘

Active/Stale ──── source="sold"/"under-contract" ──→ [ConfirmedSold] (terminal)
Active/Stale ──── source="withdrawn"/"cancelled" ──→ [Withdrawn] (terminal)
```

Terminal states (ConfirmedSold, Withdrawn) skip ApplyMissedRun transitions.

---

## Key Behaviors

1. **Upsert creates snapshot with diff vs previous**; no diff for first upsert.
2. **Price change detection**: triggers only if price differs AND previous snapshot exists.
3. **Run health gates transitions**: runHealthy=false skips status mutations.
4. **Parse attempt outcome** (success, partial, parser_error, unparseable) recorded for parse audit.
5. **Geocoding is best-effort**: daily limit → Geom=nil (no error); other errors → logged warning, upsert continues. Applied in UpsertFromParsed for all new and existing listings with address components (Phase 1).

## Impact Callouts (Phase 1)

### ⚠ ListingFilter & QueryListingsFilter
Adding ListingFilter type and QueryListingsFilter method affects:
- `business/domain/listing/listing.go:152–168` — ListingFilter struct definition
- `business/domain/listing/listing.go:179` — Storer.QueryListingsFilter signature
- `storage/listingdb/store.go:312` — Store.QueryListingsFilter implementation with SQL WHERE clause builder
- `business/sdk/listingbus/listingbus.go:164–170` — Core.QueryListingsFilter wrapper with error handling

Removing ListingFilter field or renaming query constraints requires updating ListingFilter struct + all Store implementations.

### ⚠ Geocoding Integration (Phase 1)
Adding geocoder dependency and geocodeAndApply() affects:
- `business/sdk/listingbus/listingbus.go:19` — geocoder field in Core struct
- `business/sdk/listingbus/listingbus.go:24` — NewCore requires geocode.Geocoder parameter
- `business/sdk/listingbus/listingbus.go:58, 92` — geocodeAndApply called in UpsertFromParsed (new + existing paths)
- `business/sdk/listingbus/listingbus.go:255–292` — geocodeAndApply implementation with daily-limit error handling
- `business/domain/listing/listing.go:76` — Point struct and Listing.Geom field (nullable)

Removing or changing geocoder interface requires refactoring Core.NewCore() call sites and handling address-to-Point conversion logic in both create and update paths.

## Impact Callouts (Phase 2)

### ⚠ Dedup System (Phase 2)
Adding deduplication logic (Core.RunDedup, ScorePair, DedupConfig, PossibleDuplicate) affects:
- `business/domain/listing/listing.go:182–189` — DedupReason constants (geo, acres, price, broker, title)
- `business/domain/listing/listing.go:191–199` — PossibleDuplicate struct with ListingAID/ListingBID/Score/Reasons/DetectedAt/UserDecision fields
- `business/domain/listing/listing.go:236–240` — Storer interface new methods: QueryListingsForDedup, UpsertPossibleDuplicate, QueryPossibleDuplicates, UpdateDuplicateDecision
- `business/sdk/listingbus/dedup.go:14–29` — DedupConfig struct (GeoMaxKM, AcresMaxDelta, PriceMaxDelta, ScoreThreshold) and DefaultDedupConfig()
- `business/sdk/listingbus/dedup.go:31–75` — ScorePair(a, b, cfg) computes score using geo (haversine distance ≤ GeoMaxKM), acres (% delta ≤ AcresMaxDelta), price (% delta ≤ PriceMaxDelta), broker (token overlap ≥ 0.5), title (Jaccard ≥ 0.30); returns score=len(reasons)/5.0
- `business/sdk/listingbus/dedup.go:79–128` — Core.RunDedup(ctx, cfg, now) fetches all active/stale listings, groups by county, pairs them, calls ScorePair, upserts matches above cfg.ScoreThreshold (logs warnings on upsert failure, enforces canonical ordering ListingAID < ListingBID)
- `storage/migrations/0005_possible_duplicates.sql` — possible_duplicates table (listing_a_id, listing_b_id, score, reasons, detected_at, user_decision; PK on (a_id, b_id); CHECK a_id < b_id)
- `storage/listingdb/store.go` — must implement 4 new Storer methods for dedup operations

Changing score computation (haversine, token Jaccard, threshold logic) requires updating test data and regenerating possible_duplicates. Adding/removing dedup reason constants requires migration and ScorePair logic updates. Changing canonical ID ordering (a < b) will break upsert uniqueness.

### ⚠ Auction Extension Fields (Phase 2)
Adding optional auction details (AuctionInfo type, Listing.AuctionEndDate/AuctionCurrentBid/AuctionReserve) affects:
- `business/domain/listing/listing.go:44–49` — AuctionInfo struct (EndDate, CurrentBid, Reserve fields)
- `business/domain/listing/listing.go:106–109` — Listing struct new fields: AuctionEndDate, AuctionCurrentBid, AuctionReserve (all nullable)
- `business/domain/listing/listing.go:201–211` — Listing.Auction() method returns AuctionInfo if any auction field is set, else nil
- `storage/migrations/0006_auction_extension.sql` — auction_extension table (id, listing_id, auction_end_date, auction_current_bid, auction_reserve) with unique constraint on listing_id
- `storage/listingdb/store.go` — CreateListing/UpdateListing must read/write auction_extension rows; QueryListingByID must join auction_extension

Changing auction field types (e.g., int64 → decimal for precision) requires migration and store method updates. Adding auction fields to ParsedListing requires parser integration. Removing the auction_extension table will orphan in-flight auction data.

## Impact Callouts (Phase 3)

### ⚠ Structured Attribute Extraction (Phase 3)
Adding attr_* boolean/string fields and attrs_extraction map to Listing affects:
- `business/domain/listing/listing.go:95–104` — Listing struct new fields: AttrWaterFrontage, AttrOffGrid, AttrRoadAccess, AttrPower, AttrWell, AttrSeptic, AttrPropertyType (all nullable); AttrsExtra (schema-supplied unstructured attrs); AttrsExtraction (extraction audit map)
- `business/sdk/listingbus/listingbus.go:40–62, 88–96` — UpsertFromParsed calls applyAttributeExtraction for both new and existing listings
- `business/sdk/listingbus/listingbus.go:361–432` — applyAttributeExtraction implementation: runs attrs.ExtractAll on combined title+description, populates attr_* fields from extraction results, stores full extraction map in AttrsExtraction for audit

Removing any attr_* field requires updating the extraction logic, store queries, and frontend filter UI. Changing attr field types (e.g., AttrPropertyType from string to enum) requires both extraction and storage layer updates. Disabling attribute extraction will null-populate attr_* fields on subsequent upserts.

### ⚠ Full-Text Search Query (Phase 3)
Adding FullText field to ListingFilter and full-text search implementation affects:
- `business/domain/listing/listing.go:173` — ListingFilter.FullText field (*string for SQL plainto_tsquery)
- `business/domain/listing/listing.go:223` — Storer.QueryListingsFilter must support FullText constraint
- `storage/migrations/0002_listings.sql:95` — GIN index on `to_tsvector('english', coalesce(description,'') || ' ' || coalesce(title,''))` for full-text queries
- `storage/listingdb/store.go` — QueryListingsFilter implementation adds WHERE clause `to_tsvector(...) @@ plainto_tsquery('english', $N)` when f.FullText is non-nil
- `business/sdk/listingbus/listingbus.go:168–173` — QueryListingsFilter wrapper calls storer with ListingFilter

Removing full-text support requires dropping the GIN index and FullText filter field. Changing the tokenizer language ('english' → 'german', etc.) requires index rebuild. Changing the search columns (title+description → include broker_name, address) requires index and query logic updates.
