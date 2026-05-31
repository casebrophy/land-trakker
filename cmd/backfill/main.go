package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/source"
	"github.com/cbrophy/land_trakker/foundation/scraper"
)

// eligibilityQuerier is the narrow interface backfill needs from the listing store.
type eligibilityQuerier interface {
	QueryEligibleRawFetchIDs(ctx context.Context, sourceID string, parserVersion string) ([]int64, error)
}

// rawFetchLoader retrieves a single raw HTTP fetch by its database ID.
type rawFetchLoader interface {
	QueryRawFetchByID(ctx context.Context, id int64) (source.RawFetch, error)
}

// backfillListingCore is the listing operations needed during backfill.
type backfillListingCore interface {
	UpsertFromParsed(ctx context.Context, pl scraper.ParsedListing, rawFetchID int64, now time.Time) (listing.Listing, listing.ListingSnapshot, error)
	RecordParseAttempt(ctx context.Context, pa listing.ParseAttempt) (listing.ParseAttempt, error)
}

// BackfillResult summarizes a single backfill run.
type BackfillResult struct {
	Eligible int
	Parsed   int
	Errors   int
}

// runBackfill processes all eligible raw fetches for the given scraper's source/version.
func runBackfill(ctx context.Context, q eligibilityQuerier, loader rawFetchLoader, lc backfillListingCore, s scraper.Scraper, now time.Time, out *slog.Logger) (BackfillResult, error) {
	ids, err := q.QueryEligibleRawFetchIDs(ctx, s.Source().ID, s.ParserVersion())
	if err != nil {
		return BackfillResult{}, fmt.Errorf("querying eligible raw fetches: %w", err)
	}
	result := BackfillResult{Eligible: len(ids)}
	for _, id := range ids {
		raw, loadErr := loader.QueryRawFetchByID(ctx, id)
		if loadErr != nil {
			out.Warn("load raw fetch", "id", id, "err", loadErr)
			result.Errors++
			continue
		}
		pl, parseErr := s.Parse(sourceRawToScraperRaw(raw))
		if parseErr != nil {
			errMsg := parseErr.Error()
			if _, paErr := lc.RecordParseAttempt(ctx, listing.ParseAttempt{
				RawFetchID:    id,
				ParserVersion: s.ParserVersion(),
				AttemptedAt:   now,
				Outcome:       listing.OutcomeParserError,
				ErrorMessage:  &errMsg,
			}); paErr != nil {
				out.Warn("record parse attempt (parse error)", "err", paErr)
			}
			result.Errors++
			continue
		}
		_, snap, upsertErr := lc.UpsertFromParsed(ctx, pl, id, now)
		if upsertErr != nil {
			out.Warn("upsert listing", "id", id, "err", upsertErr)
			result.Errors++
			continue
		}
		result.Parsed++
		snapID := snap.ID
		if _, paErr := lc.RecordParseAttempt(ctx, listing.ParseAttempt{
			RawFetchID:    id,
			ParserVersion: s.ParserVersion(),
			AttemptedAt:   now,
			Outcome:       listing.OutcomeSuccess,
			SnapshotID:    &snapID,
		}); paErr != nil {
			out.Warn("record parse attempt (success)", "err", paErr)
		}
	}
	return result, nil
}

// sourceRawToScraperRaw converts a stored source.RawFetch to a scraper.RawFetch for parsing.
func sourceRawToScraperRaw(raw source.RawFetch) scraper.RawFetch {
	ct := ""
	if raw.ContentType != nil {
		ct = *raw.ContentType
	}
	return scraper.RawFetch{
		SourceID:        raw.SourceID,
		SourceListingID: raw.SourceListingID,
		URL:             raw.URL,
		FetchedAt:       raw.FetchedAt,
		StatusCode:      raw.StatusCode,
		ContentType:     ct,
		Body:            raw.Body,
		Headers:         make(http.Header),
	}
}

// dryRun lists eligible raw fetch IDs for the given source without modifying any data.
func dryRun(ctx context.Context, q eligibilityQuerier, sourceID, parserVersion string, out *slog.Logger) (int, error) {
	ids, err := q.QueryEligibleRawFetchIDs(ctx, sourceID, parserVersion)
	if err != nil {
		return 0, fmt.Errorf("querying eligible raw fetches: %w", err)
	}
	out.Info("dry-run eligible fetches", "source", sourceID, "parser_version", parserVersion, "count", len(ids))
	for _, id := range ids {
		fmt.Printf("raw_fetch_id=%d\n", id)
	}
	return len(ids), nil
}

func main() {
	sourceFlag := flag.String("source", "", "source ID to backfill (required unless --all)")
	allFlag := flag.Bool("all", false, "backfill all enabled sources")
	dryRunFlag := flag.Bool("dry-run", false, "list eligible fetches without parsing")
	_ = flag.String("since", "", "only include raw_fetches fetched on or after this date (RFC3339)")
	_ = flag.Bool("force-unparseable", false, "include raw_fetches whose latest attempt is unparseable")
	flag.Parse()

	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	if !*allFlag && *sourceFlag == "" {
		log.Error("--source or --all is required")
		os.Exit(1)
	}
	if !*dryRunFlag {
		log.Error("full backfill not yet implemented; use --dry-run")
		os.Exit(1)
	}

	// TODO(nu2.11): wire real DB connection and stores
	log.Warn("backfill skeleton: DB wiring not yet connected",
		"dry_run", *dryRunFlag, "source", *sourceFlag)
	_ = listing.OutcomeSuccess // ensure import is used
}
