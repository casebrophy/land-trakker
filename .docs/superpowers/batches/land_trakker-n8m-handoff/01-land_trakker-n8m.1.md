# Handoff: land_trakker-n8m.1 (Phase 1 of 5)

**Title**: saved-searches
**Merge commit**: a8f3dec92152aae3d2a86c58d25a0fa3b91e9637
**Worker exit code**: 0

## Files changed
A	business/domain/search/search.go
A	business/sdk/searchbus/searchbus.go
A	business/sdk/searchbus/searchbus_test.go
A	storage/migrations/0004_saved_searches.sql
A	storage/searchdb/searchdb_integration_test.go
A	storage/searchdb/store.go

## Public surface added
- search.HitReason
- search.ReasonNew
- search.ReasonPriceDrop
- search.ReasonAttributeAdded
- search.SavedSearch
- search.SearchHit
- search.Storer
- searchbus.Core
- searchbus.NewCore
- searchbus.Core.EvaluateAll
- searchbus.Core.CreateSavedSearch
- searchbus.Core.UpdateSavedSearch
- searchbus.Core.DeleteSavedSearch
- searchbus.Core.QuerySavedSearches
- searchbus.Core.QuerySavedSearchByID
- searchbus.Core.QueryUnseen
- searchbus.Core.MarkHitsSeen

## Tests added
- business/sdk/searchbus/searchbus_test.go
- storage/searchdb/searchdb_integration_test.go

## Deferred
(none)
