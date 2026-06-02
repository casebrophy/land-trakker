# Handoff: land_trakker-n8m.5 (Phase 5 of 5)

**Title**: auction-ext
**Merge commit**: 12347d870d188be9fe6ac6aba2f2368a238a56e3
**Worker exit code**: 0

## Files changed
M	business/domain/listing/listing.go
M	foundation/scraper/scraper.go
M	storage/db/listing.sql.go
M	storage/db/models.go
M	storage/listingdb/listingdb_integration_test.go
M	storage/listingdb/store.go
A	storage/migrations/0006_auction_extension.sql
M	storage/queries/listing.sql

## Public surface added
- listing.AuctionInfo
- listing.Listing.AuctionEndDate
- listing.Listing.AuctionCurrentBid
- listing.Listing.AuctionReserve
- listing.Listing.Auction
- scraper.ParsedListing.AuctionEndDate
- scraper.ParsedListing.AuctionCurrentBid
- scraper.ParsedListing.AuctionReserve

## Tests added
- storage/listingdb/listingdb_integration_test.go

## Deferred
(none)
