# Handoff: land_trakker-7sf.1 (Phase 1 of 6)

**Title**: geocode-client
**Merge commit**: 3aa22f9b76c42a0ce8c8f2bbbb64d025b114aee0
**Worker exit code**: 0

## Files changed
A	foundation/geocode/cache.go
A	foundation/geocode/county.go
A	foundation/geocode/fake.go
A	foundation/geocode/geocode.go
A	foundation/geocode/geocode_test.go
A	foundation/geocode/pgstore.go
A	storage/migrations/0003_geocode_cache.sql
M	storage/migrations/migrations_test.go

## Public surface added
- geocode.Geocoder
- geocode.Result
- geocode.Precision
- geocode.PrecisionRooftop
- geocode.PrecisionStreet
- geocode.PrecisionLocality
- geocode.PrecisionCountyCentroid
- geocode.ErrDailyLimitExceeded
- geocode.CacheStore
- geocode.CachingGeocoder
- geocode.NewCachingGeocoder
- geocode.MemStore
- geocode.NewMemStore
- geocode.FakeGeocoder
- geocode.CountyCentroid
- geocode.PGStore
- geocode.NewPGStore

## Tests added
- foundation/geocode/geocode_test.go
- storage/migrations/migrations_test.go

## Deferred
(none)
