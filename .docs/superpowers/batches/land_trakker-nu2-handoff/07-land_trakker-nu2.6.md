# Handoff: land_trakker-nu2.6 (Phase 7 of 17)

**Title**: scraper-iface
**Merge commit**: (work commit 36e6710)
**Worker exit code**: 0

## Files changed
- A  foundation/scraper/scraper.go, scraper_test.go

## Public surface added (foundation/scraper)
- `scraper.Scraper` (interface — Discover/Fetch/Parse/ParserVersion)
- `scraper.Source`
- `scraper.ListingRef`
- `scraper.RawFetch`
- `scraper.ParsedListing`
- `scraper.Address`
- `scraper.Broker`

## Tests added
- foundation/scraper/scraper_test.go

## Deferred
(none) — `scraper.Scraper` is implemented by fakebroker (nu2.12), wrapped by rate-limiter (nu2.7), driven by the orchestrator (nu2.11).
