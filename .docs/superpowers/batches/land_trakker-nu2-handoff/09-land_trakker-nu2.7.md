# Handoff: land_trakker-nu2.7 (Phase 9 of 17)

**Title**: rate-limiter
**Merge commit**: (work commit 64ff665)
**Worker exit code**: 0

## Files changed
- A  foundation/scraper/rate_limiter.go, rate_limiter_test.go

## Public surface added (foundation/scraper)
- `scraper.RateLimiter` — per-source rate limiting with jitter + exponential backoff retry
- `scraper.NewRateLimiter`
- `scraper.NewRateLimiterWithClock` (injectable clock for tests)
- `scraper.Clock` (interface)

## Tests added
- foundation/scraper/rate_limiter_test.go (uses injectable Clock — no wall-clock flakiness)

## Deferred
(none) — consumed by orchestrator (nu2.11) to throttle Fetch calls.
