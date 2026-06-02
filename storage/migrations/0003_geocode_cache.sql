-- +goose Up
CREATE TABLE geocode_cache (
    id          bigserial PRIMARY KEY,
    address_key text NOT NULL UNIQUE,
    geom        geometry(Point, 4326),
    precision   text NOT NULL,
    provider    text NOT NULL,
    confidence  numeric(3,2),
    raw         jsonb,
    cached_at   timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS geocode_cache;
