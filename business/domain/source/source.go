package source

import (
	"context"
	"time"
)

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
	ID                             string
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
	// MinResultRatioForInactivation is a value in [0,1]. A scrape run is
	// "healthy" for inactivation only if discovered_count >=
	// MinResultRatioForInactivation * prior_run.discovered_count.
	MinResultRatioForInactivation float64
	LastRunAt                     *time.Time
	NextRunAt                     *time.Time
	Notes                         *string
}

// ScrapeRun records a single execution of a source scraper.
type ScrapeRun struct {
	ID              int64
	SourceID        string
	StartedAt       time.Time
	FinishedAt      *time.Time
	Status          RunStatus
	DiscoveredCount *int
	FetchedCount    *int
	ParsedCount     *int
	ErrorCount      *int
	ErrorSample     *string
	Notes           *string
}

// RawFetch stores the raw HTTP response for a single listing URL.
type RawFetch struct {
	ID              int64
	SourceID        string
	SourceListingID string
	ScrapeRunID     *int64
	URL             string
	FetchedAt       time.Time
	StatusCode      int
	ContentType     *string
	Body            []byte
	BodySHA256      []byte
	HeadersJSON     []byte
}

// Storer defines the persistence contract for source-domain objects.
// Implementations live in storage/sourcedb.
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
