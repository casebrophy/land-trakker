# Handoff: land_trakker-n8m.3 (Phase 3 of 5)

**Title**: dedup-job
**Merge commit**: 6e6e36ae7e5412c0222f580cc1a636ad77e77891
**Worker exit code**: 0

## Files changed
M	business/domain/listing/listing.go
A	business/sdk/listingbus/dedup.go
A	business/sdk/listingbus/dedup_test.go
M	business/sdk/listingbus/listingbus_test.go
M	storage/listingdb/store.go
A	storage/migrations/0005_possible_duplicates.sql

## Public surface added
- listing.DedupReasonGeo
- listing.DedupReasonAcres
- listing.DedupReasonPrice
- listing.DedupReasonBroker
- listing.DedupReasonTitle
- listing.PossibleDuplicate
- listing.Storer.QueryListingsForDedup
- listing.Storer.UpsertPossibleDuplicate
- listing.Storer.QueryPossibleDuplicates
- listing.Storer.UpdateDuplicateDecision
- listingbus.DedupConfig
- listingbus.DefaultDedupConfig
- listingbus.ScorePair
- listingbus.Core.RunDedup

## Tests added
- business/sdk/listingbus/dedup_test.go
- business/sdk/listingbus/listingbus_test.go

## Deferred
(none)
