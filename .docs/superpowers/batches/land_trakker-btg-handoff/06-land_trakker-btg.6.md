# Handoff: land_trakker-btg.6 (Phase 6 of 6)

**Title**: web-admin-sources
**Merge commit**: fd52cb565cdec93d46683a96c86b708cf2e3a0d8
**Worker exit code**: 0 (result file missing — recovered from branch commit)

## Files changed
M	cmd/api/api_test.go
M	cmd/api/main.go
M	cmd/api/routes.go
A	foundation/web/admin_sources.go
A	foundation/web/admin_sources_test.go
A	foundation/web/templates/admin_sources.html

## Public surface added
- web.AdminSourcesHandler
- web.AdminSourcesUpdateHandler
- web.AdminSourcesBackfillHandler
- web.AdminSourcesQuerier
- web.AdminSourcesUpdater
- web.BackfillTrigger

## Deferred
(none)
