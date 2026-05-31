-- name: CreateSource :one
INSERT INTO sources (
    id, display_name, base_url, enabled, rate_limit_ms, concurrency,
    user_agent, respect_robots, absence_days_before_stale,
    absence_days_before_inactive, consecutive_missed_runs_threshold,
    min_result_ratio_for_inactivation, last_run_at, next_run_at, notes
) VALUES (
    @id, @display_name, @base_url, @enabled, @rate_limit_ms, @concurrency,
    @user_agent, @respect_robots, @absence_days_before_stale,
    @absence_days_before_inactive, @consecutive_missed_runs_threshold,
    @min_result_ratio_for_inactivation, sqlc.narg('last_run_at'), sqlc.narg('next_run_at'), sqlc.narg('notes')
)
RETURNING *;

-- name: UpdateSource :exec
UPDATE sources SET
    display_name                      = @display_name,
    base_url                          = @base_url,
    enabled                           = @enabled,
    rate_limit_ms                     = @rate_limit_ms,
    concurrency                       = @concurrency,
    user_agent                        = @user_agent,
    respect_robots                    = @respect_robots,
    absence_days_before_stale         = @absence_days_before_stale,
    absence_days_before_inactive      = @absence_days_before_inactive,
    consecutive_missed_runs_threshold = @consecutive_missed_runs_threshold,
    min_result_ratio_for_inactivation = @min_result_ratio_for_inactivation,
    last_run_at                       = sqlc.narg('last_run_at'),
    next_run_at                       = sqlc.narg('next_run_at'),
    notes                             = sqlc.narg('notes')
WHERE id = @id;

-- name: GetSourceByID :one
SELECT * FROM sources WHERE id = @id;

-- name: ListSources :many
SELECT * FROM sources ORDER BY id;

-- name: CreateScrapeRun :one
INSERT INTO scrape_runs (
    source_id, started_at, finished_at, status,
    discovered_count, fetched_count, parsed_count, error_count,
    error_sample, notes
) VALUES (
    @source_id, @started_at, sqlc.narg('finished_at'), @status,
    sqlc.narg('discovered_count'), sqlc.narg('fetched_count'),
    sqlc.narg('parsed_count'), sqlc.narg('error_count'),
    sqlc.narg('error_sample'), sqlc.narg('notes')
)
RETURNING *;

-- name: UpdateScrapeRun :exec
UPDATE scrape_runs SET
    finished_at      = sqlc.narg('finished_at'),
    status           = @status,
    discovered_count = sqlc.narg('discovered_count'),
    fetched_count    = sqlc.narg('fetched_count'),
    parsed_count     = sqlc.narg('parsed_count'),
    error_count      = sqlc.narg('error_count'),
    error_sample     = sqlc.narg('error_sample'),
    notes            = sqlc.narg('notes')
WHERE id = @id;

-- name: GetScrapeRunByID :one
SELECT * FROM scrape_runs WHERE id = @id;

-- name: ListScrapeRunsBySource :many
SELECT * FROM scrape_runs
WHERE source_id = @source_id
ORDER BY started_at DESC
LIMIT @lim::int;

-- name: CreateRawFetch :one
INSERT INTO raw_fetches (
    source_id, source_listing_id, scrape_run_id, url,
    fetched_at, status_code, content_type,
    body, body_sha256, headers_json
) VALUES (
    @source_id, @source_listing_id, sqlc.narg('scrape_run_id'), @url,
    @fetched_at, @status_code, sqlc.narg('content_type'),
    @body, @body_sha256, sqlc.narg('headers_json')
)
RETURNING *;

-- name: GetRawFetchByID :one
SELECT * FROM raw_fetches WHERE id = @id;

-- name: ListRawFetchesByListing :many
SELECT * FROM raw_fetches
WHERE source_id = @source_id AND source_listing_id = @source_listing_id
ORDER BY fetched_at DESC;
