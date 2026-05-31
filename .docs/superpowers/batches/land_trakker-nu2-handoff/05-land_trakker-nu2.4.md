# Handoff: land_trakker-nu2.4 (Phase 5 of 17)

**Title**: domain-types
**Merge commit**: (work commit eac259c)
**Worker exit code**: 0

## Files changed
- A  business/domain/listing/listing.go, listing_test.go
- A  business/domain/source/source.go

## Public surface added
**source pkg**: `source.Source`, `source.ScrapeRun`, `source.RawFetch`, `source.RunStatus`, `source.Storer` (interface)
**listing pkg**: `listing.Listing`, `listing.ListingSnapshot`, `listing.PriceChange`, `listing.Point`, `listing.Storer` (interface)
**listing status enum**: `listing.ListingStatus`, `listing.StatusActive`, `listing.StatusStale`, `listing.StatusPresumedInactive`, `listing.StatusConfirmedSold`, `listing.StatusWithdrawn`, `listing.AllStatuses`

## Tests added
- business/domain/listing/listing_test.go

## Deferred
(none) — `source.Storer` and `listing.Storer` interfaces are implemented by storage-impls (nu2.8) and consumed by listingbus (nu2.9).
