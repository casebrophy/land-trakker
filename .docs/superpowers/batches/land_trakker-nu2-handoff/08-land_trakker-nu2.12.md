# Handoff: land_trakker-nu2.12 (Phase 8 of 17)

**Title**: fakebroker
**Merge commit**: (work commit f1e0149)
**Worker exit code**: 0

## Files changed
- A  foundation/scraper/fakebroker.go, fakebroker_test.go

## Public surface added (foundation/scraper)
- `scraper.FakeBroker` — in-repo stub implementing `scraper.Scraper` (Discover/Fetch/Parse/ParserVersion), 3 deterministic fixtures
- `scraper.NewFakeBroker`

## Tests added
- foundation/scraper/fakebroker_test.go

## Deferred
(none) — FakeBroker is the Scraper used by orchestrator (nu2.11), cmd-scraperd (nu2.13), cmd-scrape-once (nu2.14), and the p0 e2e tests (nu2.17). ParserVersion is bumpable for reparse tests.
