-- name: CreateListing :one
INSERT INTO listings (
    source_id, source_listing_id, url, first_seen_at, last_seen_at,
    status, consecutive_misses, dismissed, dismissed_reason, saved,
    title, description, price_cents, acres,
    address_line, city, county, state, postal_code,
    geom, photos,
    broker_name, broker_phone, broker_email,
    posted_at, source_updated_at,
    attr_water_frontage, attr_off_grid, attr_road_access,
    attr_power, attr_well, attr_septic, attr_property_type,
    attrs_extra, attrs_extraction
) VALUES (
    @source_id, @source_listing_id, @url, @first_seen_at, @last_seen_at,
    @status, @consecutive_misses, @dismissed, sqlc.narg('dismissed_reason'), @saved,
    sqlc.narg('title'), sqlc.narg('description'), sqlc.narg('price_cents'), sqlc.narg('acres'),
    sqlc.narg('address_line'), sqlc.narg('city'), sqlc.narg('county'), sqlc.narg('state'), sqlc.narg('postal_code'),
    ST_GeomFromText(sqlc.narg('geom_wkt')::text, 4326), @photos,
    sqlc.narg('broker_name'), sqlc.narg('broker_phone'), sqlc.narg('broker_email'),
    sqlc.narg('posted_at'), sqlc.narg('source_updated_at'),
    sqlc.narg('attr_water_frontage'), sqlc.narg('attr_off_grid'), sqlc.narg('attr_road_access'),
    sqlc.narg('attr_power'), sqlc.narg('attr_well'), sqlc.narg('attr_septic'), sqlc.narg('attr_property_type'),
    @attrs_extra, @attrs_extraction
)
RETURNING
    id, source_id, source_listing_id, url, first_seen_at, last_seen_at,
    status, consecutive_misses, dismissed, dismissed_reason, saved,
    title, description, price_cents, acres, price_per_acre_cents,
    address_line, city, county, state, postal_code,
    ST_AsText(geom) AS geom_wkt,
    photos, broker_name, broker_phone, broker_email, posted_at, source_updated_at,
    attr_water_frontage, attr_off_grid, attr_road_access, attr_power, attr_well, attr_septic,
    attr_property_type, attrs_extra, attrs_extraction;

-- name: UpdateListing :exec
UPDATE listings SET
    url                = @url,
    last_seen_at       = @last_seen_at,
    status             = @status,
    consecutive_misses = @consecutive_misses,
    dismissed          = @dismissed,
    dismissed_reason   = sqlc.narg('dismissed_reason'),
    saved              = @saved,
    title              = sqlc.narg('title'),
    description        = sqlc.narg('description'),
    price_cents        = sqlc.narg('price_cents'),
    acres              = sqlc.narg('acres'),
    address_line       = sqlc.narg('address_line'),
    city               = sqlc.narg('city'),
    county             = sqlc.narg('county'),
    state              = sqlc.narg('state'),
    postal_code        = sqlc.narg('postal_code'),
    geom               = ST_GeomFromText(sqlc.narg('geom_wkt')::text, 4326),
    photos             = @photos,
    broker_name        = sqlc.narg('broker_name'),
    broker_phone       = sqlc.narg('broker_phone'),
    broker_email       = sqlc.narg('broker_email'),
    posted_at          = sqlc.narg('posted_at'),
    source_updated_at  = sqlc.narg('source_updated_at'),
    attr_water_frontage = sqlc.narg('attr_water_frontage'),
    attr_off_grid      = sqlc.narg('attr_off_grid'),
    attr_road_access   = sqlc.narg('attr_road_access'),
    attr_power         = sqlc.narg('attr_power'),
    attr_well          = sqlc.narg('attr_well'),
    attr_septic        = sqlc.narg('attr_septic'),
    attr_property_type = sqlc.narg('attr_property_type'),
    attrs_extra        = @attrs_extra,
    attrs_extraction   = @attrs_extraction
WHERE id = @id::uuid;

-- name: GetListingByID :one
SELECT
    id, source_id, source_listing_id, url, first_seen_at, last_seen_at,
    status, consecutive_misses, dismissed, dismissed_reason, saved,
    title, description, price_cents, acres, price_per_acre_cents,
    address_line, city, county, state, postal_code,
    ST_AsText(geom) AS geom_wkt,
    photos, broker_name, broker_phone, broker_email, posted_at, source_updated_at,
    attr_water_frontage, attr_off_grid, attr_road_access, attr_power, attr_well, attr_septic,
    attr_property_type, attrs_extra, attrs_extraction
FROM listings
WHERE id = @id::uuid;

-- name: GetListingBySource :one
SELECT
    id, source_id, source_listing_id, url, first_seen_at, last_seen_at,
    status, consecutive_misses, dismissed, dismissed_reason, saved,
    title, description, price_cents, acres, price_per_acre_cents,
    address_line, city, county, state, postal_code,
    ST_AsText(geom) AS geom_wkt,
    photos, broker_name, broker_phone, broker_email, posted_at, source_updated_at,
    attr_water_frontage, attr_off_grid, attr_road_access, attr_power, attr_well, attr_septic,
    attr_property_type, attrs_extra, attrs_extraction
FROM listings
WHERE source_id = @source_id AND source_listing_id = @source_listing_id;

-- name: CreateListingSnapshot :one
INSERT INTO listing_snapshots (
    listing_id, raw_fetch_id, captured_at,
    price_cents, acres, status, title, description,
    structured_attrs, diff
) VALUES (
    @listing_id::uuid, @raw_fetch_id, @captured_at,
    sqlc.narg('price_cents'), sqlc.narg('acres'), sqlc.narg('status'),
    sqlc.narg('title'), sqlc.narg('description'),
    sqlc.narg('structured_attrs'), sqlc.narg('diff')
)
RETURNING *;

-- name: ListSnapshotsByListing :many
SELECT * FROM listing_snapshots
WHERE listing_id = @listing_id::uuid
ORDER BY captured_at DESC;

-- name: CreatePriceChange :one
INSERT INTO price_changes (
    listing_id, changed_at, old_price_cents, new_price_cents, snapshot_id
) VALUES (
    @listing_id::uuid, @changed_at, sqlc.narg('old_price_cents'), @new_price_cents, sqlc.narg('snapshot_id')
)
RETURNING id, listing_id, changed_at, old_price_cents, new_price_cents, delta_cents, snapshot_id;

-- name: ListPriceChangesByListing :many
SELECT id, listing_id, changed_at, old_price_cents, new_price_cents, delta_cents, snapshot_id
FROM price_changes
WHERE listing_id = @listing_id::uuid
ORDER BY changed_at DESC;

-- name: CreateParseAttempt :one
INSERT INTO parse_attempts (raw_fetch_id, parser_version, attempted_at, outcome, error_message, snapshot_id)
VALUES (@raw_fetch_id, @parser_version, @attempted_at, @outcome, sqlc.narg('error_message'), sqlc.narg('snapshot_id'))
RETURNING *;

-- name: CreateAuctionExt :exec
INSERT INTO auction_extension (listing_id, auction_end_date, auction_current_bid, auction_reserve)
VALUES (@listing_id::uuid, sqlc.narg('auction_end_date'), sqlc.narg('auction_current_bid'), sqlc.narg('auction_reserve'));

-- name: GetAuctionExt :one
SELECT id, listing_id, auction_end_date, auction_current_bid, auction_reserve
FROM auction_extension
WHERE listing_id = @listing_id::uuid;

-- name: UpdateAuctionExt :exec
UPDATE auction_extension
SET auction_end_date = sqlc.narg('auction_end_date'),
    auction_current_bid = sqlc.narg('auction_current_bid'),
    auction_reserve = sqlc.narg('auction_reserve')
WHERE listing_id = @listing_id::uuid;
