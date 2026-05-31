# Handoff: land_trakker-nu2.10 (Phase 12 of 17)

**Title**: backfill-query
**Merge commit**: (work commit 7cf4476)
**Worker exit code**: 0

## Files changed
- M  business/domain/listing/listing.go (added ParseAttempt types + Storer methods)
- M  business/sdk/listingbus/listingbus.go (+RecordParseAttempt), listingbus_test.go
- A  cmd/backfill/backfill_test.go; M cmd/backfill/main.go (--dry-run skeleton)
- M  storage/listingdb/store.go, storage/db/listing.sql.go, storage/queries/listing.sql (eligibility query + parse_attempts writes)

## Public surface added
- `listing.ParseAttemptOutcome` enum: `OutcomeSuccess`, `OutcomePartial`, `OutcomeParserError`, `OutcomeUnparseable`
- `listing.ParseAttempt`
- `listing.Storer` extended: `CreateParseAttempt`, `QueryEligibleRawFetchIDs` (implemented in listingdb)
- `listingbus.Core.RecordParseAttempt`

## Tests added
- cmd/backfill/backfill_test.go; listingbus_test.go extended

## Deferred
(none) — eligibility query excludes unparseable, includes never-parsed/parser_error/old-version. cmd/backfill --dry-run lists eligible fetches. Consumed by p0-e2e-tests (nu2.17).
