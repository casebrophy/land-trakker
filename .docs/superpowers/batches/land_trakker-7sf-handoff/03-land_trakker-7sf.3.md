# Handoff: land_trakker-7sf.3 (Phase 3 of 6)

**Title**: search-filters
**Merge commit**: 5be8e63c02f8cd954360010724a552a6eaf62731
**Worker exit code**: 0

## Files changed
M	business/domain/listing/listing.go
M	business/sdk/listingbus/listingbus.go
M	business/sdk/listingbus/listingbus_test.go
M	storage/listingdb/listingdb_integration_test.go
M	storage/listingdb/store.go

## Public surface added
- listing.ListingFilter
- listing.Storer.QueryListingsFilter
- listingbus.Core.QueryListingsFilter

## Tests added
- business/sdk/listingbus/listingbus_test.go
- storage/listingdb/listingdb_integration_test.go

## Deferred
(none)
