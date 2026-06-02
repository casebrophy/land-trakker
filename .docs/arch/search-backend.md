# Search Backend Architecture

## Domain Layer: `business/domain/search/`

### Core Types

**SavedSearch** — a persisted search filter with metadata:
```go
type SavedSearch struct {
    ID        string
    Name      string
    Query     listing.ListingFilter  // serialized as JSONB
    CreatedAt time.Time
    Enabled   bool
}
```

**SearchHit** — a single match between a saved search and a listing:
```go
type SearchHit struct {
    ID            int64
    SavedSearchID string
    ListingID     string
    HitAt         time.Time
    Reason        HitReason
    Seen          bool
}
```

**HitReason** — enum: `"new"`, `"price_drop"`, `"attribute_added"`. Recorded in search_hits.reason column.

**Storer interface** — persistence contract:
- SavedSearch CRUD: CreateSavedSearch, UpdateSavedSearch, DeleteSavedSearch, QuerySavedSearchByID, QuerySavedSearches
- SearchHit: CreateHitIfAbsent (INSERT ... ON CONFLICT DO NOTHING), QueryUnseen, MarkHitsSeen

**Implementation**: `storage/searchdb.Store` (PostgreSQL with pgx).

---

## Service Layer: `business/sdk/searchbus/`

### Core

**Dependencies**: search.Storer, ListingStore (partial listing interface for QueryListingsFilter, QueryPriceChangesByListing, QuerySnapshotsByListing), slog.Logger.

**Key Methods**:
- **CreateSavedSearch, UpdateSavedSearch, DeleteSavedSearch, QuerySavedSearchByID, QuerySavedSearches** → SavedSearch CRUD wrappers with error wrapping
- **EvaluateAll**(ctx, now) → runs all enabled saved searches against current listings (page size 500, max 10k rows per search), records hits for new listings, price drops, and attribute changes; returns total hits created
- **QueryUnseen**(limit) → returns up to limit unseen hits
- **MarkHitsSeen**(ids) → marks hits as seen

### Internal Logic (EvaluateAll)

1. **Query enabled saved searches** via Storer
2. **Page through listings** matching each search's ListingFilter (QueryListingsFilter) in batches of 500, up to 10k rows per search
3. **Evaluate each listing** for three hit types:
   - **ReasonNew**: if FirstSeenAt is within the past 24h
   - **ReasonPriceDrop**: if QueryPriceChangesByListing shows a price drop (OldPriceCents != nil && NewPriceCents < OldPriceCents) within the window
   - **ReasonAttributeAdded**: if QuerySnapshotsByListing's latest snapshot has attribute keys (attr_*) with old=nil and new!=nil in its Diff
4. **Record hits** via CreateHitIfAbsent (idempotent per unique index on saved_search_id, listing_id, reason, hit_at)
5. **Logging**: failed searches/listings are logged as warnings and continue

Time windows: `since := now.Add(-24 * time.Hour)`, `hitAt := now.UTC().Truncate(24 * time.Hour)`.

---

## Database Schema: `storage/migrations/0004_saved_searches.sql`

```sql
CREATE TABLE saved_searches (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL,
    query       jsonb NOT NULL,           -- ListingFilter serialized
    created_at  timestamptz NOT NULL DEFAULT now(),
    enabled     boolean NOT NULL DEFAULT true
);

CREATE TABLE search_hits (
    id                  bigserial PRIMARY KEY,
    saved_search_id     uuid NOT NULL REFERENCES saved_searches(id) ON DELETE CASCADE,
    listing_id          uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    hit_at              timestamptz NOT NULL,
    reason              text NOT NULL,   -- 'new', 'price_drop', 'attribute_added'
    seen                boolean NOT NULL DEFAULT false
);

CREATE UNIQUE INDEX ON search_hits (saved_search_id, listing_id, reason, hit_at);
CREATE INDEX ON search_hits (hit_at DESC) WHERE seen = false;
```

---

## Store Implementation: `storage/searchdb/store.go`

All methods implement the search.Storer interface:

- **CreateSavedSearch** — marshals ListingFilter to JSONB, INSERTs row, RETURNs assigned UUID
- **UpdateSavedSearch** — UPDATEs name, query, enabled WHERE id = $1
- **DeleteSavedSearch** — DELETEs WHERE id = $1
- **QuerySavedSearchByID** — SELECTs WHERE id = $1
- **QuerySavedSearches** — SELECTs all WHERE enabled = true ORDER BY created_at ASC
- **CreateHitIfAbsent** — INSERTs search hit with ON CONFLICT DO NOTHING (unique index enforces idempotency on (saved_search_id, listing_id, reason, hit_at))
- **QueryUnseen** — SELECTs WHERE seen = false ORDER BY hit_at DESC LIMIT $1
- **MarkHitsSeen** — UPDATEs SET seen = true WHERE id = ANY($1)

Conversion helpers:
- **marshalFilter / unmarshalFilter** — JSON serde for listing.ListingFilter ↔ []byte
- **strToUUID / uuidToStr** — string ↔ pgtype.UUID conversions
- **timeToTZ** — time.Time → pgtype.Timestamptz

---

## Cross-Domain Dependencies

### Inbound
- **Outbound calls**: search depends on listing domain (ListingFilter struct, QueryListingsFilter, QueryPriceChangesByListing, QuerySnapshotsByListing)

### Outbound
- Typically called from a scheduled job or handler (not yet in arch scope for Phase 2; expected in later handlers)

---

## Impact Callouts

### ⚠ ListingFilter Type (business/domain/listing/listing.go)
Changing ListingFilter struct definition (field names, types, or nullability) requires:
- `business/domain/search/search.go:23` — SavedSearch.Query field definition
- `storage/searchdb/store.go:232–244` — marshalFilter and unmarshalFilter JSON serde
- `business/sdk/searchbus/searchbus.go:134` — EvaluateAll passes ss.Query to QueryListingsFilter call

Adding/removing filter constraints silently breaks existing saved_searches.query JSONB rows until they are manually updated or deleted.

### ⚠ SearchHit Unique Index (storage/migrations/0004_saved_searches.sql:19)
The unique index on (saved_search_id, listing_id, reason, hit_at) enforces:
- `storage/searchdb/store.go:106–122` — CreateHitIfAbsent relies on this for idempotency (ON CONFLICT DO NOTHING)
- `business/sdk/searchbus/searchbus.go:169–178, 188–197, 212–221` — EvaluateAll must call CreateHitIfAbsent for all three reason types; changing reason enum or logic affects hit deduplication

Removing or altering this index allows duplicate hits to be created.

### ⚠ HitReason Enum (business/domain/search/search.go:11–17)
Adding/removing reason types affects:
- `storage/migrations/0004_saved_searches.sql:15` — search_hits.reason column (text, any string valid)
- `storage/searchdb/store.go:193–212` — scanSearchHit converts reason string → HitReason type
- `business/sdk/searchbus/searchbus.go:164–227` — evaluateOne and evaluateListing must handle all reason types to populate their respective SearchHit.Reason fields

Changes to reason enum require updating EvaluateAll evaluation logic if new criteria are introduced.

### ⚠ EvaluateAll Hit Window (business/sdk/searchbus/searchbus.go:94–122)
The 24-hour evaluation window is hardcoded:
- `business/sdk/searchbus/searchbus.go:107` — since := now.Add(-24 * time.Hour)
- `business/sdk/searchbus/searchbus.go:108` — hitAt := now.UTC().Truncate(24 * time.Hour)

Changing this window affects:
- Which listings match ReasonNew (FirstSeenAt.After(since))
- Which price/attribute changes are detected (pc.ChangedAt.After(since), latest.CapturedAt.After(since))

Making this configurable requires adding a parameter to EvaluateAll and any scheduler calling it.

### ⚠ Page Size Limits (business/sdk/searchbus/searchbus.go:94–97)
Constants evalPageSize (500) and evalMaxRows (10000) limit evaluation:
- `business/sdk/searchbus/searchbus.go:126–159` — evaluateOne loop pagination and early exit
- Changing limits affects resource usage and hit coverage for large result sets

Making these configurable requires adding parameters to EvaluateAll or Core config.
