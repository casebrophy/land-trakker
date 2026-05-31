# Handoff: land_trakker-nu2.9 (Phase 11 of 17)

**Title**: listingbus (also adds sourcebus)
**Merge commit**: (work commit 2aacac3)
**Worker exit code**: 0

## Files changed
- A  business/sdk/listingbus/listingbus.go, listingbus_test.go
- A  business/sdk/sourcebus/{doc,sourcebus,sourcebus_test}.go

## Public surface added
**listingbus**: `listingbus.Core`, `listingbus.NewCore`, `listingbus.MissedRunConfig`,
  methods `Core.UpsertFromParsed`, `Core.ApplyMissedRun` (snapshot diff, state machine, price-change)
**sourcebus**: `sourcebus.Core`, `sourcebus.NewCore`, `sourcebus.IsRunHealthy`,
  methods `QuerySource`, `QuerySources`, `CreateSource`, `UpdateSource`, `StartRun`, `FinishRun`, `CreateRawFetch`, `QueryRawFetchesByListing`

## Tests added
- business/sdk/listingbus/listingbus_test.go, business/sdk/sourcebus/sourcebus_test.go

## Deferred
(none) — both Cores take Storer interfaces (nu2.4) constructor-injected; orchestrator (nu2.11) wires concrete sourcedb/listingdb stores (nu2.10/nu2.8) into them. backfill (nu2.10) uses listingbus.
