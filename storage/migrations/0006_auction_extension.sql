-- +goose Up
CREATE TABLE auction_extension (
    id              bigserial PRIMARY KEY,
    listing_id      uuid NOT NULL UNIQUE REFERENCES listings(id) ON DELETE CASCADE,
    auction_end_date timestamptz,
    auction_current_bid bigint,
    auction_reserve bigint
);
CREATE INDEX ON auction_extension (listing_id);

-- +goose Down
DROP TABLE IF EXISTS auction_extension;
