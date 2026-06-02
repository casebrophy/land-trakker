-- +goose Up
CREATE TABLE saved_searches (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text NOT NULL,
    query       jsonb NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    enabled     boolean NOT NULL DEFAULT true
);

CREATE TABLE search_hits (
    id                  bigserial PRIMARY KEY,
    saved_search_id     uuid NOT NULL REFERENCES saved_searches(id) ON DELETE CASCADE,
    listing_id          uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    hit_at              timestamptz NOT NULL,
    reason              text NOT NULL,
    seen                boolean NOT NULL DEFAULT false
);

CREATE UNIQUE INDEX ON search_hits (saved_search_id, listing_id, reason, hit_at);
CREATE INDEX ON search_hits (hit_at DESC) WHERE seen = false;

-- +goose Down
DROP TABLE IF EXISTS search_hits;
DROP TABLE IF EXISTS saved_searches;
