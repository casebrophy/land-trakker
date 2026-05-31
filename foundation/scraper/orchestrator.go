package scraper

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/source"
)

// sourceCore is the subset of sourcebus.Core used by the orchestrator.
// *sourcebus.Core satisfies this interface.
type sourceCore interface {
	QuerySource(ctx context.Context, id string) (source.Source, error)
	QueryLatestRun(ctx context.Context, sourceID string) (*source.ScrapeRun, error)
	StartRun(ctx context.Context, sourceID string, now time.Time) (source.ScrapeRun, error)
	FinishRun(ctx context.Context, run source.ScrapeRun) error
	CreateRawFetch(ctx context.Context, rf source.RawFetch) (source.RawFetch, error)
	QueryRawFetchesByListing(ctx context.Context, sourceID, sourceListingID string) ([]source.RawFetch, error)
}

// listingCore is the subset of listingbus.Core used by the orchestrator.
// *listingbus.Core satisfies this interface.
type listingCore interface {
	UpsertFromParsed(ctx context.Context, pl ParsedListing, rawFetchID int64, now time.Time) (listing.Listing, listing.ListingSnapshot, error)
	RecordParseAttempt(ctx context.Context, pa listing.ParseAttempt) (listing.ParseAttempt, error)
	QueryListingBySource(ctx context.Context, sourceID, sourceListingID string) (listing.Listing, error)
}

// MissedRunHandler is called for each source listing ID that was present in
// the previous run but absent from the current Discover() result.  It is
// provided by the wiring layer and typically calls
// listingbus.Core.ApplyMissedRun after a QueryListingBySource lookup.
// A nil handler silently skips missed-run processing.
type MissedRunHandler func(ctx context.Context, sourceID, sourceListingID string, srcCfg source.Source, runHealthy bool, now time.Time) error

// RunResult summarises a single orchestrator cycle for one source.
type RunResult struct {
	RunID      int64
	Discovered int
	Fetched    int
	Parsed     int
	Errors     int
	Duration   time.Duration
}

// Orchestrator wires together a Scraper with the source/listing buses to
// execute the full discover→diff→fetch(TTL)→parse→upsert→snapshot pipeline.
type Orchestrator struct {
	sc            sourceCore
	lc            listingCore
	missedHandler MissedRunHandler
	log           *slog.Logger
	// prevDiscovered holds the source listing IDs returned by the last
	// successful Discover() call, keyed by source ID.
	prevDiscovered map[string][]string
}

// NewOrchestrator constructs an Orchestrator.  missedHandler may be nil.
func NewOrchestrator(sc sourceCore, lc listingCore, missedHandler MissedRunHandler, log *slog.Logger) *Orchestrator {
	return &Orchestrator{
		sc:             sc,
		lc:             lc,
		missedHandler:  missedHandler,
		log:            log,
		prevDiscovered: make(map[string][]string),
	}
}

// RunOnce executes a full scrape cycle for the given Scraper.
// fetchTTL determines how long a stored raw fetch is considered "fresh";
// pass 0 to always re-fetch every discovered listing.
func (o *Orchestrator) RunOnce(ctx context.Context, s Scraper, fetchTTL time.Duration) (result RunResult, retErr error) {
	srcCfg, err := o.sc.QuerySource(ctx, s.Source().ID)
	if err != nil {
		return RunResult{}, fmt.Errorf("query source %q: %w", s.Source().ID, err)
	}
	if !srcCfg.Enabled {
		return RunResult{}, nil
	}

	start := time.Now()
	priorRun, _ := o.sc.QueryLatestRun(ctx, srcCfg.ID)

	run, err := o.sc.StartRun(ctx, srcCfg.ID, start)
	if err != nil {
		return RunResult{}, fmt.Errorf("start run: %w", err)
	}
	result.RunID = run.ID

	defer func() {
		finishedAt := time.Now()
		result.Duration = finishedAt.Sub(start)
		status := source.RunStatusOK
		if result.Errors > 0 {
			status = source.RunStatusPartial
		}
		if retErr != nil {
			status = source.RunStatusFailed
		}
		run.Status = status
		run.DiscoveredCount = intPtr(result.Discovered)
		run.FetchedCount = intPtr(result.Fetched)
		run.ParsedCount = intPtr(result.Parsed)
		run.ErrorCount = intPtr(result.Errors)
		run.FinishedAt = &finishedAt
		if ferr := o.sc.FinishRun(ctx, run); ferr != nil {
			o.log.Warn("finish run", "source_id", srcCfg.ID, "err", ferr)
		}
	}()

	refs, err := s.Discover(ctx)
	if err != nil {
		return result, fmt.Errorf("discover: %w", err)
	}
	result.Discovered = len(refs)

	// Index refs by source listing ID for fast lookup.
	refByID := make(map[string]ListingRef, len(refs))
	discoveredIDs := make([]string, len(refs))
	for i, ref := range refs {
		refByID[ref.SourceListingID] = ref
		discoveredIDs[i] = ref.SourceListingID
	}

	prev := o.prevDiscovered[srcCfg.ID]
	added, kept, removed := DiffRefs(prev, discoveredIDs)

	// Collect refs to fetch: added always, kept only when TTL has expired.
	toFetch := make([]ListingRef, 0, len(added)+len(kept))
	for _, id := range added {
		toFetch = append(toFetch, refByID[id])
	}
	for _, id := range kept {
		if o.ttlExpired(ctx, srcCfg.ID, id, start, fetchTTL) {
			toFetch = append(toFetch, refByID[id])
		}
	}

	for _, ref := range toFetch {
		raw, fetchErr := s.Fetch(ctx, ref)
		if fetchErr != nil {
			o.log.Warn("fetch error", "source_id", srcCfg.ID, "listing_id", ref.SourceListingID, "err", fetchErr)
			result.Errors++
			continue
		}

		storedRaw, storeErr := o.sc.CreateRawFetch(ctx, scraperRawToSource(raw, run.ID))
		if storeErr != nil {
			o.log.Warn("store raw fetch", "source_id", srcCfg.ID, "listing_id", ref.SourceListingID, "err", storeErr)
			result.Errors++
			continue
		}
		result.Fetched++

		pl, parseErr := s.Parse(raw)
		if parseErr != nil {
			errMsg := parseErr.Error()
			if _, paErr := o.lc.RecordParseAttempt(ctx, listing.ParseAttempt{
				RawFetchID:    storedRaw.ID,
				ParserVersion: s.ParserVersion(),
				AttemptedAt:   start,
				Outcome:       listing.OutcomeParserError,
				ErrorMessage:  &errMsg,
			}); paErr != nil {
				o.log.Warn("record parse attempt", "err", paErr)
			}
			result.Errors++
			continue
		}

		_, snap, upsertErr := o.lc.UpsertFromParsed(ctx, pl, storedRaw.ID, start)
		if upsertErr != nil {
			o.log.Warn("upsert listing", "source_id", srcCfg.ID, "listing_id", ref.SourceListingID, "err", upsertErr)
			if _, paErr := o.lc.RecordParseAttempt(ctx, listing.ParseAttempt{
				RawFetchID:    storedRaw.ID,
				ParserVersion: s.ParserVersion(),
				AttemptedAt:   start,
				Outcome:       listing.OutcomeParserError,
			}); paErr != nil {
				o.log.Warn("record parse attempt (upsert fail)", "err", paErr)
			}
			result.Errors++
			continue
		}
		result.Parsed++

		snapID := snap.ID
		if _, paErr := o.lc.RecordParseAttempt(ctx, listing.ParseAttempt{
			RawFetchID:    storedRaw.ID,
			ParserVersion: s.ParserVersion(),
			AttemptedAt:   start,
			Outcome:       listing.OutcomeSuccess,
			SnapshotID:    &snapID,
		}); paErr != nil {
			o.log.Warn("record parse attempt (success)", "err", paErr)
		}
	}

	// Apply missed-run logic for disappeared listings.
	if len(removed) > 0 && o.missedHandler != nil {
		partialRun := run
		partialRun.DiscoveredCount = intPtr(result.Discovered)
		healthy := orchIsRunHealthy(partialRun, srcCfg, priorRun)
		for _, id := range removed {
			if hmErr := o.missedHandler(ctx, srcCfg.ID, id, srcCfg, healthy, start); hmErr != nil {
				o.log.Warn("missed run handler", "source_id", srcCfg.ID, "listing_id", id, "err", hmErr)
			}
		}
	}

	o.prevDiscovered[srcCfg.ID] = discoveredIDs
	return result, nil
}

// ttlExpired reports whether the most recent raw fetch for the given listing
// predates now by more than fetchTTL.  Returns true if no prior fetch exists
// or fetchTTL == 0 (always re-fetch).
func (o *Orchestrator) ttlExpired(ctx context.Context, sourceID, sourceListingID string, now time.Time, fetchTTL time.Duration) bool {
	if fetchTTL == 0 {
		return true
	}
	fetches, err := o.sc.QueryRawFetchesByListing(ctx, sourceID, sourceListingID)
	if err != nil || len(fetches) == 0 {
		return true
	}
	var newest time.Time
	for _, f := range fetches {
		if f.FetchedAt.After(newest) {
			newest = f.FetchedAt
		}
	}
	return now.Sub(newest) > fetchTTL
}

// scraperRawToSource converts a scraper.RawFetch to a source.RawFetch for
// storage, computing the body SHA-256 and serialising HTTP headers to JSON.
func scraperRawToSource(raw RawFetch, runID int64) source.RawFetch {
	sum := sha256.Sum256(raw.Body)
	headersJSON, _ := json.Marshal(raw.Headers)
	ct := raw.ContentType
	return source.RawFetch{
		SourceID:        raw.SourceID,
		SourceListingID: raw.SourceListingID,
		ScrapeRunID:     &runID,
		URL:             raw.URL,
		FetchedAt:       raw.FetchedAt,
		StatusCode:      raw.StatusCode,
		ContentType:     &ct,
		Body:            raw.Body,
		BodySHA256:      sum[:],
		HeadersJSON:     headersJSON,
	}
}

// orchIsRunHealthy mirrors sourcebus.IsRunHealthy without importing that
// package (which would create a circular dependency).
func orchIsRunHealthy(run source.ScrapeRun, src source.Source, prior *source.ScrapeRun) bool {
	if prior == nil || prior.DiscoveredCount == nil || *prior.DiscoveredCount == 0 {
		return true
	}
	if run.DiscoveredCount == nil {
		return false
	}
	threshold := src.MinResultRatioForInactivation * float64(*prior.DiscoveredCount)
	return float64(*run.DiscoveredCount) >= threshold
}

func intPtr(n int) *int {
	return &n
}
