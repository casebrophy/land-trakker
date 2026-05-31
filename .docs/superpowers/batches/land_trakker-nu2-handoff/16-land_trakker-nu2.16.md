# Handoff: land_trakker-nu2.16 (Phase 16 of 17)

**Title**: web-listings
**Merge commit**: (work commits 3af725b, 53300b0)
**Worker exit code**: 0 (wiring_check: pass)

## Files changed
- A  foundation/web/listings.go, listings_test.go
- A  foundation/web/templates/{listings,listing_detail}.html
- M  cmd/api/{main,routes,api_test}.go (wired listings routes behind RequireAuth)
- M  business/sdk/listingbus/listingbus.go (+query methods), listingbus_test.go
- M  business/domain/listing/listing.go (+Storer.QueryListings)
- M  storage/listingdb/store.go (impl QueryListings)

## Public surface added
- `web.ListingsHandler`, `web.ListingDetailHandler`, `web.ListingsQuerier`
- `listingbus.Core.QueryListings`, `QuerySnapshotsByListing`, `QueryPriceChangesByListing`
- `listing.Storer.QueryListings`

## Tests added
- foundation/web/listings_test.go; listingbus_test.go extended

## Deferred
(none) — HTML template views (Go html/template), not a Vue SPA. List + detail w/ snapshot history & price changes.
