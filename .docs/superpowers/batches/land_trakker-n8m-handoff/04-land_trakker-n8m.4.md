# Handoff: land_trakker-n8m.4 (Phase 4 of 5)

**Title**: web-duplicates
**Merge commit**: 44e73b4ec0e424d4adeefbc784ddcc40fae0f443
**Worker exit code**: 0

## Files changed
M	cmd/api/api_test.go
M	cmd/api/main.go
M	cmd/api/routes.go
A	foundation/web/duplicates.go
A	foundation/web/duplicates_test.go
A	foundation/web/templates/duplicates.html

## Public surface added
- web.DuplicatesQuerier
- web.DuplicatesHandler
- web.DuplicatesUpdateHandler

## Tests added
- cmd/api/api_test.go
- foundation/web/duplicates_test.go

## Deferred
(none)
