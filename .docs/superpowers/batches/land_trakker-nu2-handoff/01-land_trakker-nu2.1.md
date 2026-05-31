# Handoff: land_trakker-nu2.1 (Phase 1 of 17)

**Title**: bootstrap-module
**Merge commit**: 195e026 (work commit 7466b50)
**Worker exit code**: 0 (result JSON missing — recovered via branch commits; bd auto-closed)

## Files changed
- M  .gitignore
- A  Makefile  (build/test/test-integration/lint/sqlc-generate targets; Docker-detected integration)
- A  README.md
- A  business/domain/{listing,parcel,search,source}/doc.go
- A  business/sdk/{listingbus,parcelbus,searchbus}/doc.go
- A  cmd/{api,backfill,scrape-once,scraperd}/main.go  (stubs)
- A  foundation/{geocode,llm,parser,scraper,storage,web}/doc.go
- A  go.mod (module github.com/cbrophy/land_trakker, Go 1.26), go.sum
- A  sqlc.yaml  (points at storage/migrations + storage/queries)
- A  storage/{listingdb,parceldb,searchdb,sourcedb}/doc.go
- A  storage/migrations/.gitkeep, storage/queries/.gitkeep

## Public surface added
- Go module path: `github.com/cbrophy/land_trakker`
- goose v3.27.1 + sqlc v1.31.1 wired as `go -tool` dependencies
- Makefile targets: `build`, `test`, `test-integration`, `lint`, `sqlc-generate`
- Ardan Labs four-layer skeleton dirs (cmd/business/foundation/storage) with compiling doc.go/main.go stubs

## Tests added
(none — empty-but-compiling module)

## Deferred
(none)
