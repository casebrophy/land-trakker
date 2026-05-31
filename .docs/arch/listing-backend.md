# Listing Backend Architecture

## Domain Layer: `business/domain/listing/`

### Core Types

**Listing** — canonical land listing record with UUID, source identity, state machine fields, parsed fields (title, price, acres, address, broker), and computed fields (price_per_acre_cents).

**ListingStatus** — enum: `active`, `stale`, `presumed_inactive`, `confirmed_sold`, `withdrawn`. State transitions in Core.ApplyMissedRun and UpsertFromParsed.

**ListingSnapshot** — point-in-time capture (RawFetchID, PriceCents, Acres, Title, StructuredAttrs). Includes Diff map vs previous snapshot (5-field diffs: price_cents, acres, title, description, status).

**PriceChange** — detected price movement (OldPriceCents, NewPriceCents, SnapshotID). Created in UpsertFromParsed when price differs between snapshots.

**ParseAttempt** — single parse attempt record (RawFetchID, ParserVersion, Outcome, ErrorMessage, SnapshotID).

**Storer interface** — persistence contract:
- Listing CRUD: CreateListing, UpdateListing, QueryListingByID, QueryListingBySource, QueryListings
- Snapshot: CreateSnapshot, QuerySnapshotsByListing  
- PriceChange: CreatePriceChange, QueryPriceChangesByListing
- ParseAttempt: CreateParseAttempt, QueryEligibleRawFetchIDs

**Implementation**: `storage/listingdb.Store` (PostgreSQL with pgx).

---

## Service Layer: `business/sdk/listingbus/`

### Core

**Dependency**: Storer interface + slog.Logger.

**Key Methods**:
- **UpsertFromParsed**(ParsedListing, rawFetchID, now) → creates/updates Listing, snapshots with diffs, detects price changes
- **ApplyMissedRun**(listingID, MissedRunConfig, runHealthy, now) → health-gated status transitions (active→stale→presumed_inactive) based on consecutive misses & absence days
- **QueryListings, QueryListingByID, QueryListingBySource** → Listing lookups with error wrapping
- **QuerySnapshotsByListing, QueryPriceChangesByListing** → history queries
- **RecordParseAttempt** → writes parse_attempts rows

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
