# Source Backend System

The source domain manages web scraper configuration, execution, and raw HTTP fetch storage. It maintains sources (scraper targets), scrape runs (execution records), and raw fetches (cached HTTP responses). Sources track health via result ratios; runs record discovery/parsing metrics; raw fetches preserve response bodies for offline parsing.

## Core Types

```go
// RunStatus represents the outcome of a scrape run.
type RunStatus string

const (
	RunStatusRunning RunStatus = "running"
	RunStatusOK      RunStatus = "ok"
	RunStatusPartial RunStatus = "partial"
	RunStatusFailed  RunStatus = "failed"
)

// Source holds per-source configuration for the scraper.
type Source struct {
	ID                             string     // unique identifier
	DisplayName                    string
	BaseURL                        string
	Enabled                        bool
	RateLimitMS                    int
	Concurrency                    int
	UserAgent                      string
	RespectRobots                  bool
	AbsenceDaysBeforeStale         int
	AbsenceDaysBeforeInactive      int
	ConsecutiveMissedRunsThreshold int
	MinResultRatioForInactivation  float64    // [0,1]; health threshold for inactivation
	LastRunAt                      *time.Time
	NextRunAt                      *time.Time
	Notes                          *string
}

// ScrapeRun records a single execution of a source scraper.
type ScrapeRun struct {
	ID              int64
	SourceID        string
	StartedAt       time.Time
	FinishedAt      *time.Time
	Status          RunStatus
	DiscoveredCount *int     // total URLs discovered
	FetchedCount    *int     // URLs fetched
	ParsedCount     *int     // URLs successfully parsed
	ErrorCount      *int
	ErrorSample     *string
	Notes           *string
}

// RawFetch stores the raw HTTP response for a single listing URL.
type RawFetch struct {
	ID              int64
	SourceID        string
	SourceListingID string      // correlation to listing entity
	ScrapeRunID     *int64
	URL             string
	FetchedAt       time.Time
	StatusCode      int
	ContentType     *string
	Body            []byte      // raw HTTP body
	BodySHA256      []byte      // content hash
	HeadersJSON     []byte      // HTTP headers as JSON
}

// Storer defines the persistence contract for source-domain objects.
type Storer interface {
	// Source operations
	CreateSource(ctx context.Context, src Source) (Source, error)
	UpdateSource(ctx context.Context, src Source) error
	QuerySourceByID(ctx context.Context, id string) (Source, error)
	QuerySources(ctx context.Context) ([]Source, error)

	// ScrapeRun operations
	CreateRun(ctx context.Context, run ScrapeRun) (ScrapeRun, error)
	UpdateRun(ctx context.Context, run ScrapeRun) error
	QueryRunByID(ctx context.Context, id int64) (ScrapeRun, error)
	QueryRunsBySource(ctx context.Context, sourceID string, limit int) ([]ScrapeRun, error)

	// RawFetch operations
	CreateRawFetch(ctx context.Context, rf RawFetch) (RawFetch, error)
	QueryRawFetchByID(ctx context.Context, id int64) (RawFetch, error)
	QueryRawFetchesByListing(ctx context.Context, sourceID, sourceListingID string) ([]RawFetch, error)
}
```

## File Map

### Models
- `business/domain/source/source.go` — Source, ScrapeRun, RawFetch structs; RunStatus enum; Storer interface

### Core
- `business/sdk/sourcebus/sourcebus.go` — Core struct; methods QuerySource, QuerySources, CreateSource, UpdateSource, StartRun, FinishRun, CreateRawFetch, QueryLatestRun, QueryRawFetchesByListing, IsRunHealthy; wraps Storer with logging and error wrapping

### Store
- `storage/sourcedb/store.go` — Store implements Storer; methods CreateSource, UpdateSource, QuerySourceByID, QuerySources, CreateRun, UpdateRun, QueryRunByID, QueryRunsBySource, CreateRawFetch, QueryRawFetchByID, QueryRawFetchesByListing; handles pgtype conversion (numeric, timestamptz, int4, int8, text)
- `storage/db/source.sql.go` — sqlc-generated database access layer

## Impact Callouts

### ⚠ Source ({business/domain/source/source.go})
Changing this struct affects:
- `storage/sourcedb/store.go:rowToSource()` — conversion from DB row; all fields must map to CreateSourceParams/UpdateSourceParams
- `storage/db/source.sql.go` — sqlc-generated params; column names must match SQL schema
- MinResultRatioForInactivation field used in `sourcebus.IsRunHealthy()` calculation (business/sdk/sourcebus/sourcebus.go:125)

### ⚠ ScrapeRun ({business/domain/source/source.go})
Changing this struct affects:
- `storage/sourcedb/store.go:rowToRun()` — conversion from DB ScrapeRun row; all fields must map to CreateScrapeRunParams/UpdateScrapeRunParams
- `sourcebus.IsRunHealthy()` — depends on DiscoveredCount field for health ratio calculation
- Conversion helpers handle nullable fields (DiscoveredCount, FetchedCount, ParsedCount, ErrorCount as pgtype.Int4)

### ⚠ RawFetch ({business/domain/source/source.go})
Changing this struct affects:
- `storage/sourcedb/store.go:rowToRawFetch()` — conversion from DB RawFetch; Body, BodySHA256, HeadersJSON are byte arrays
- SourceListingID field used to correlate raw fetches to listing entities for cross-domain queries

### ⚠ Storer interface ({business/domain/source/source.go})
Adding/changing a method affects:
- `storage/sourcedb/store.go` — must implement every Storer method; _ source.Storer = (*Store)(nil) compile-time check at line 350
- `business/sdk/sourcebus/sourcebus.go:Core` — all public methods delegate to storer methods; adding a Storer method requires corresponding Core wrapper

### ⚠ IsRunHealthy() ({business/sdk/sourcebus/sourcebus.go:111})
Changing this health calculation logic affects:
- Source inactivation workflows — affects whether a source is considered healthy enough to remain active based on discovered result ratio
- Threshold: floor(MinResultRatioForInactivation * priorRun.DiscoveredCount); run is healthy if current.DiscoveredCount >= threshold
- Returns true if priorRun is nil or prior discovered count is 0/nil (bootstrap case)

## Cross-Domain Dependencies

- **Outbound:** None — source domain is isolation; no calls to other buses
- **Inbound:** Likely called by scraper/orchestration domain to manage source configs and record run results; listing domain may query raw fetches by SourceListingID to cross-reference

## Storage Schema

- Table: `sources` — ID (PK), DisplayName, BaseUrl, Enabled, RateLimitMs, Concurrency, UserAgent, RespectRobots, AbsenceDaysBeforeStale, AbsenceDaysBeforeInactive, ConsecutiveMissedRunsThreshold, MinResultRatioForInactivation (numeric), LastRunAt, NextRunAt, Notes (text)
- Table: `scrape_runs` — ID (PK), SourceID (FK), StartedAt (timestamptz), FinishedAt (timestamptz, nullable), Status (text), DiscoveredCount (int4, nullable), FetchedCount (int4, nullable), ParsedCount (int4, nullable), ErrorCount (int4, nullable), ErrorSample (text, nullable), Notes (text, nullable)
- Table: `raw_fetches` — ID (PK), SourceID (FK), SourceListingID, ScrapeRunID (int8, nullable), URL, FetchedAt (timestamptz), StatusCode (int4), ContentType (text, nullable), Body (bytea), BodySha256 (bytea), HeadersJson (bytea)
