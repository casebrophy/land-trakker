package sourcebus

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/source"
)

// Core provides business logic for the source domain.
type Core struct {
	storer source.Storer
	log    *slog.Logger
}

// NewCore constructs a Core with the given storer and logger.
func NewCore(storer source.Storer, log *slog.Logger) *Core {
	return &Core{storer: storer, log: log}
}

// QuerySource retrieves a source by its ID.
func (c *Core) QuerySource(ctx context.Context, id string) (source.Source, error) {
	src, err := c.storer.QuerySourceByID(ctx, id)
	if err != nil {
		return source.Source{}, fmt.Errorf("querying source by id: %w", err)
	}
	return src, nil
}

// QuerySources retrieves all sources.
func (c *Core) QuerySources(ctx context.Context) ([]source.Source, error) {
	srcs, err := c.storer.QuerySources(ctx)
	if err != nil {
		return nil, fmt.Errorf("querying sources: %w", err)
	}
	return srcs, nil
}

// CreateSource persists a new source.
func (c *Core) CreateSource(ctx context.Context, src source.Source) (source.Source, error) {
	created, err := c.storer.CreateSource(ctx, src)
	if err != nil {
		return source.Source{}, fmt.Errorf("creating source: %w", err)
	}
	return created, nil
}

// UpdateSource persists changes to an existing source.
func (c *Core) UpdateSource(ctx context.Context, src source.Source) error {
	if err := c.storer.UpdateSource(ctx, src); err != nil {
		return fmt.Errorf("updating source: %w", err)
	}
	return nil
}

// StartRun creates a new scrape run in running state.
func (c *Core) StartRun(ctx context.Context, sourceID string, now time.Time) (source.ScrapeRun, error) {
	run := source.ScrapeRun{
		SourceID:  sourceID,
		StartedAt: now,
		Status:    source.RunStatusRunning,
	}
	created, err := c.storer.CreateRun(ctx, run)
	if err != nil {
		return source.ScrapeRun{}, fmt.Errorf("creating scrape run: %w", err)
	}
	return created, nil
}

// FinishRun persists the final state of a completed scrape run.
func (c *Core) FinishRun(ctx context.Context, run source.ScrapeRun) error {
	if err := c.storer.UpdateRun(ctx, run); err != nil {
		return fmt.Errorf("updating scrape run: %w", err)
	}
	return nil
}

// CreateRawFetch persists a raw HTTP fetch record.
func (c *Core) CreateRawFetch(ctx context.Context, rf source.RawFetch) (source.RawFetch, error) {
	created, err := c.storer.CreateRawFetch(ctx, rf)
	if err != nil {
		return source.RawFetch{}, fmt.Errorf("creating raw fetch: %w", err)
	}
	return created, nil
}

// QueryLatestRun retrieves the most recent scrape run for a source, or nil if
// no runs exist.
func (c *Core) QueryLatestRun(ctx context.Context, sourceID string) (*source.ScrapeRun, error) {
	runs, err := c.storer.QueryRunsBySource(ctx, sourceID, 1)
	if err != nil {
		return nil, fmt.Errorf("querying latest run: %w", err)
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return &runs[0], nil
}

// QueryRawFetchesByListing retrieves all raw fetches for a given source listing.
func (c *Core) QueryRawFetchesByListing(ctx context.Context, sourceID, sourceListingID string) ([]source.RawFetch, error) {
	fetches, err := c.storer.QueryRawFetchesByListing(ctx, sourceID, sourceListingID)
	if err != nil {
		return nil, fmt.Errorf("querying raw fetches by listing: %w", err)
	}
	return fetches, nil
}

// IsRunHealthy reports whether a scrape run is "healthy" for inactivation purposes.
// Returns true if discoveredCount >= src.MinResultRatioForInactivation * priorRun.DiscoveredCount.
// If priorRun is nil or priorRun.DiscoveredCount is nil/0, returns true.
func IsRunHealthy(run source.ScrapeRun, src source.Source, priorRun *source.ScrapeRun) bool {
	if priorRun == nil {
		return true
	}
	if priorRun.DiscoveredCount == nil || *priorRun.DiscoveredCount == 0 {
		return true
	}
	if run.DiscoveredCount == nil {
		return false
	}

	threshold := src.MinResultRatioForInactivation * float64(*priorRun.DiscoveredCount)
	return float64(*run.DiscoveredCount) >= threshold
}
