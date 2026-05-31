# Handoff: land_trakker-nu2.11 (Phase 13 of 17)

**Title**: orchestrator
**Merge commit**: (work commit aa7e812)
**Worker exit code**: 0

## Files changed
- A  foundation/scraper/diff.go, orchestrator.go, orchestrator_test.go
- M  business/sdk/listingbus/listingbus.go (+QueryListingBySource)
- M  business/sdk/sourcebus/sourcebus.go (+QueryLatestRun)

## Public surface added
**scraper**: `scraper.Orchestrator`, `scraper.NewOrchestrator`, `scraper.RunResult`, `scraper.MissedRunHandler`, `scraper.DiffRefs`
**bus extensions**: `sourcebus.Core.QueryLatestRun`, `listingbus.Core.QueryListingBySource`

Flow: Discover → diff (DiffRefs) → Fetch(TTL) → store raw → Parse → normalize → upsert → snapshot → scrape_runs.

## Tests added
- foundation/scraper/orchestrator_test.go (diff logic + full run over fake scraper, unit-tested)

## Deferred
(none) — Orchestrator is wired into cmd/scraperd (nu2.13) and cmd/scrape-once (nu2.14). NewOrchestrator takes Scraper, RateLimiter, sourcebus.Core, listingbus.Core.
