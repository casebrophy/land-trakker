package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/cbrophy/land_trakker/business/domain/listing"
)

// eligibilityQuerier is the narrow interface backfill needs from the listing store.
type eligibilityQuerier interface {
	QueryEligibleRawFetchIDs(ctx context.Context, sourceID string, parserVersion string) ([]int64, error)
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
