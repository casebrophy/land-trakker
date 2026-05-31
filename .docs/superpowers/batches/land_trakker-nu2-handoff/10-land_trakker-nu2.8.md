# Handoff: land_trakker-nu2.8 (Phase 10 of 17)

**Title**: storage-impls
**Merge commit**: (work commits 4aa3dd7, af3751e, 0ae1018)
**Worker exit code**: 0

## Files changed
- A  storage/db/{db,models,listing.sql,source.sql}.go  (sqlc-generated, now committed)
- A  storage/queries/{listing,source}.sql  (sqlc query source)
- A  storage/sourcedb/store.go, sourcedb_integration_test.go
- A  storage/listingdb/store.go, listingdb_integration_test.go
- M  sqlc.yaml, .gitignore (un-ignored storage/db/ so generated code is committed)

## Public surface added
- `sourcedb.Store`, `sourcedb.NewStore` — implements `source.Storer`
- `listingdb.Store`, `listingdb.NewStore` — implements `listing.Storer`
- compile-time interface assertions added (var _ source.Storer = ...)

## Tests added
- storage/sourcedb/sourcedb_integration_test.go, storage/listingdb/listingdb_integration_test.go (integration — likely Docker/`//go:build integration` gated)

## Deferred
(none) — Stores are consumed by listingbus (nu2.9), backfill (nu2.10), orchestrator (nu2.11).
To regenerate sqlc: `make sqlc-generate`. Query source lives in storage/queries/.
