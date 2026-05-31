-- +goose Up
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE listings (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    source_id           text NOT NULL REFERENCES sources(id),
    source_listing_id   text NOT NULL,
    url                 text NOT NULL,
    first_seen_at       timestamptz NOT NULL,
    last_seen_at        timestamptz NOT NULL,
    status              text NOT NULL,
    consecutive_misses  int  NOT NULL DEFAULT 0,
    dismissed           boolean NOT NULL DEFAULT false,
    dismissed_reason    text,
    saved               boolean NOT NULL DEFAULT false,

    title               text,
    description         text,
    price_cents         bigint,
    acres               numeric(10,2),
    price_per_acre_cents bigint GENERATED ALWAYS AS (
        CASE WHEN acres > 0 AND price_cents IS NOT NULL
             THEN (price_cents / acres)::bigint END
    ) STORED,
    address_line        text,
    city                text,
    county              text,
    state               text,
    postal_code         text,
    geom                geometry(Point, 4326),
    photos              text[] NOT NULL DEFAULT '{}',
    broker_name         text,
    broker_phone        text,
    broker_email        text,
    posted_at           timestamptz,
    source_updated_at   timestamptz,

    attr_water_frontage boolean,
    attr_off_grid       boolean,
    attr_road_access    text,
    attr_power          boolean,
    attr_well           boolean,
    attr_septic         boolean,
    attr_property_type  text,
    attrs_extra         jsonb NOT NULL DEFAULT '{}',
    attrs_extraction    jsonb NOT NULL DEFAULT '{}'
);
CREATE UNIQUE INDEX ON listings (source_id, source_listing_id);
CREATE INDEX ON listings USING GIST (geom);
CREATE INDEX ON listings (state, county) WHERE status IN ('active','stale');
CREATE INDEX ON listings (price_per_acre_cents) WHERE status IN ('active','stale');
CREATE INDEX ON listings USING GIN (to_tsvector('english', coalesce(description,'') || ' ' || coalesce(title,'')));

CREATE TABLE listing_snapshots (
    id                  bigserial PRIMARY KEY,
    listing_id          uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    raw_fetch_id        bigint NOT NULL REFERENCES raw_fetches(id),
    captured_at         timestamptz NOT NULL,
    price_cents         bigint,
    acres               numeric(10,2),
    status              text,
    title               text,
    description         text,
    structured_attrs    jsonb,
    diff                jsonb
);
CREATE INDEX ON listing_snapshots (listing_id, captured_at DESC);

CREATE TABLE price_changes (
    id              bigserial PRIMARY KEY,
    listing_id      uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    changed_at      timestamptz NOT NULL,
    old_price_cents bigint,
    new_price_cents bigint NOT NULL,
    delta_cents     bigint GENERATED ALWAYS AS (new_price_cents - old_price_cents) STORED,
    snapshot_id     bigint REFERENCES listing_snapshots(id)
);
CREATE INDEX ON price_changes (listing_id, changed_at DESC);
CREATE INDEX ON price_changes (changed_at DESC) WHERE delta_cents < 0;

ALTER TABLE parse_attempts
    ADD CONSTRAINT fk_parse_attempts_snapshot
    FOREIGN KEY (snapshot_id) REFERENCES listing_snapshots(id);

-- +goose Down
ALTER TABLE parse_attempts DROP CONSTRAINT IF EXISTS fk_parse_attempts_snapshot;
DROP TABLE IF EXISTS price_changes;
DROP TABLE IF EXISTS listing_snapshots;
DROP TABLE IF EXISTS listings;
