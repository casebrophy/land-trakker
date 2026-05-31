-- +goose Up
CREATE TABLE sources (
    id                                  text PRIMARY KEY,
    display_name                        text NOT NULL,
    base_url                            text NOT NULL,
    enabled                             boolean NOT NULL DEFAULT true,
    rate_limit_ms                       int  NOT NULL DEFAULT 1000,
    concurrency                         int  NOT NULL DEFAULT 1,
    user_agent                          text NOT NULL,
    respect_robots                      boolean NOT NULL DEFAULT true,
    absence_days_before_stale           int  NOT NULL DEFAULT 14,
    absence_days_before_inactive        int  NOT NULL DEFAULT 30,
    consecutive_missed_runs_threshold   int  NOT NULL DEFAULT 3,
    min_result_ratio_for_inactivation   numeric(4,3) NOT NULL DEFAULT 0.500,
    last_run_at                         timestamptz,
    next_run_at                         timestamptz,
    notes                               text
);

CREATE TABLE scrape_runs (
    id                  bigserial PRIMARY KEY,
    source_id           text NOT NULL REFERENCES sources(id),
    started_at          timestamptz NOT NULL,
    finished_at         timestamptz,
    status              text NOT NULL,
    discovered_count    int,
    fetched_count       int,
    parsed_count        int,
    error_count         int,
    error_sample        text,
    notes               text
);
CREATE INDEX ON scrape_runs (source_id, started_at DESC);

CREATE TABLE raw_fetches (
    id                  bigserial PRIMARY KEY,
    source_id           text NOT NULL REFERENCES sources(id),
    source_listing_id   text NOT NULL,
    scrape_run_id       bigint REFERENCES scrape_runs(id),
    url                 text NOT NULL,
    fetched_at          timestamptz NOT NULL,
    status_code         int  NOT NULL,
    content_type        text,
    body                bytea NOT NULL,
    body_sha256         bytea NOT NULL,
    headers_json        jsonb
);
CREATE INDEX ON raw_fetches (source_id, source_listing_id, fetched_at DESC);
CREATE UNIQUE INDEX ON raw_fetches (source_id, source_listing_id, body_sha256);

CREATE TABLE parse_attempts (
    id              bigserial PRIMARY KEY,
    raw_fetch_id    bigint NOT NULL REFERENCES raw_fetches(id) ON DELETE CASCADE,
    parser_version  text NOT NULL,
    attempted_at    timestamptz NOT NULL DEFAULT now(),
    outcome         text NOT NULL,
    error_message   text,
    snapshot_id     bigint  -- FK to listing_snapshots added in 0002
);
CREATE INDEX ON parse_attempts (raw_fetch_id, attempted_at DESC);
CREATE INDEX ON parse_attempts (parser_version, outcome);

-- +goose Down
DROP TABLE IF EXISTS parse_attempts;
DROP TABLE IF EXISTS raw_fetches;
DROP TABLE IF EXISTS scrape_runs;
DROP TABLE IF EXISTS sources;
