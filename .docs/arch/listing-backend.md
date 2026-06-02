# Listing Backend Architecture

## Domain Layer: `business/domain/listing/`

### Core Types

**Listing** — canonical land listing record with UUID, source identity, state machine fields, parsed fields (title, price, acres, address, broker), and computed fields (price_per_acre_cents).

**ListingStatus** — enum: `active`, `stale`, `presumed_inactive`, `confirmed_sold`, `withdrawn`. State transitions in Core.ApplyMissedRun and UpsertFromParsed.

**ListingSnapshot** — point-in-time capture (RawFetchID, PriceCents, Acres, Title, StructuredAttrs). Includes Diff map vs previous snapshot (5-field diffs: price_cents, acres, title, description, status).

**PriceChange** — detected price movement (OldPriceCents, NewPriceCents, SnapshotID). Created in UpsertFromParsed when price differs between snapshots.

**ParseAttempt** — single parse attempt record (RawFetchID, ParserVersion, Outcome, ErrorMessage, SnapshotID).

**ListingFilter** — optional search constraints with nil-means-no-constraint semantics:
```
type ListingFilter struct {
  AcresMin, AcresMax *float64
  PriceMin, PriceMax *int64 (cents)
  Counties []string
  PPAMin, PPAMax *int64 (price_per_acre_cents)
  PropertyType *string
  AttrWaterFrontage, AttrOffGrid, AttrPower, AttrWell, AttrSeptic *bool
}
```

**Storer interface** — persistence contract:
- Listing CRUD: CreateListing, UpdateListing, QueryListingByID, QueryListingBySource, QueryListings, QueryListingsFilter
- Snapshot: CreateSnapshot, QuerySnapshotsByListing  
- PriceChange: CreatePriceChange, QueryPriceChangesByListing
- ParseAttempt: CreateParseAttempt, QueryEligibleRawFetchIDs

**Implementation**: `storage/listingdb.Store` (PostgreSQL with pgx).

---

## Service Layer: `business/sdk/listingbus/`

### Core

**Dependency**: Storer interface + slog.Logger.

**Key Methods**:
- **UpsertFromParsed**(ParsedListing, rawFetchID, now) → creates/updates Listing, snapshots with diffs, detects price changes; includes geocoding call via geocodeAndApply
- **ApplyMissedRun**(listingID, MissedRunConfig, runHealthy, now) → health-gated status transitions (active→stale→presumed_inactive) based on consecutive misses & absence days
- **QueryListings, QueryListingByID, QueryListingBySource** → Listing lookups with error wrapping
- **QueryListingsFilter**(ListingFilter, limit, offset) → filtered Listing lookups with ListingFilter constraints (Phase 1)
- **QuerySnapshotsByListing, QueryPriceChangesByListing** → history queries
- **RecordParseAttempt** → writes parse_attempts rows
- **geocodeAndApply**(ctx, listing) → calls geocoder with address components, handles daily limit + other errors gracefully (Phase 1)

### Internal Logic

**derivedStatus** — computes status for existing listing: sold/under-contract/withdrawn sources set terminal states; presumed_inactive/stale reappearances reset to active.

**diffSnapshots** — 5-field snapshot diff (price_cents, acres, title, description, status) in format `{"field": {"old": ..., "new": ...}}`.

**applyParsedFields** — copies ParsedListing fields (address, photos, broker, structured attrs) onto domain Listing.

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
