# Handoff: land_trakker-nu2.14 (Phase 15 of 17)

**Title**: cmd-scrape-once
**Merge commit**: (work commit fdf8a25)
**Worker exit code**: 0

## Files changed
- M  cmd/scrape-once/main.go (single-run CLI, -source flag)
- A  cmd/scrape-once/main_test.go

## Public surface added
(none — cmd binary). Runs scraper.Orchestrator once with FakeBroker; mirrors cmd/scraperd wiring.

## Tests added
- cmd/scrape-once/main_test.go

## Deferred
(none) — used by p0-e2e-tests (nu2.17).
