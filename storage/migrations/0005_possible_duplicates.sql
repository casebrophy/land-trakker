-- +goose Up
CREATE TABLE possible_duplicates (
    listing_a_id    uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    listing_b_id    uuid NOT NULL REFERENCES listings(id) ON DELETE CASCADE,
    score           numeric(3,2) NOT NULL,
    reasons         text[] NOT NULL,
    detected_at     timestamptz NOT NULL DEFAULT now(),
    user_decision   text,
    PRIMARY KEY (listing_a_id, listing_b_id),
    CHECK (listing_a_id < listing_b_id)
);

-- +goose Down
DROP TABLE IF EXISTS possible_duplicates;
