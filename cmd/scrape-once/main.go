package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/source"
	"github.com/cbrophy/land_trakker/foundation/scraper"
)

func main() {
	sourceFlag := flag.String("source", "", "Scraper source to run (e.g., 'fakebroker')")
	flag.Parse()

	if *sourceFlag == "" {
		flag.PrintDefaults()
		os.Exit(1)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create in-memory test fakes for the buses.
	src := source.Source{
		ID:          *sourceFlag,
		DisplayName: "Scraper (Single Run)",
		Enabled:     true,
	}
	sc := newMainSourceCore(src)
	lc := newMainListingCore()

	// Create the orchestrator.
	orch := scraper.NewOrchestrator(sc, lc, nil, log)

	var s scraper.Scraper
	switch *sourceFlag {
	case "fakebroker":
		s = scraper.NewFakeBroker()
	default:
		log.Error("unknown source", "source", *sourceFlag)
		os.Exit(1)
	}

	// Run one tick with a 24-hour TTL.
	result, err := orch.RunOnce(ctx, s, 24*time.Hour)
	if err != nil {
		log.Error("orchestrator run failed", "error", err)
		os.Exit(1)
	}

	log.Info("orchestrator tick completed",
		"run_id", result.RunID,
		"discovered", result.Discovered,
		"fetched", result.Fetched,
		"parsed", result.Parsed,
		"errors", result.Errors,
		"duration_ms", result.Duration.Milliseconds(),
	)

	if result.Errors > 0 {
		os.Exit(1)
	}
}

// Minimal in-memory stubs for orchestrator wiring (same as test fakes).

type mainSourceCore struct {
	src         source.Source
	runs        []source.ScrapeRun
	rawFetches  []source.RawFetch
	nextRunID   int64
	nextFetchID int64
}

func newMainSourceCore(src source.Source) *mainSourceCore {
	return &mainSourceCore{src: src}
}

func (f *mainSourceCore) QuerySource(_ context.Context, _ string) (source.Source, error) {
	return f.src, nil
}

func (f *mainSourceCore) QueryLatestRun(_ context.Context, _ string) (*source.ScrapeRun, error) {
	return nil, nil
}

func (f *mainSourceCore) StartRun(_ context.Context, sourceID string, now time.Time) (source.ScrapeRun, error) {
	f.nextRunID++
	run := source.ScrapeRun{ID: f.nextRunID, SourceID: sourceID, StartedAt: now, Status: source.RunStatusRunning}
	f.runs = append(f.runs, run)
	return run, nil
}

func (f *mainSourceCore) FinishRun(_ context.Context, run source.ScrapeRun) error {
	for i := range f.runs {
		if f.runs[i].ID == run.ID {
			f.runs[i] = run
			return nil
		}
	}
	return nil
}

func (f *mainSourceCore) CreateRawFetch(_ context.Context, rf source.RawFetch) (source.RawFetch, error) {
	f.nextFetchID++
	rf.ID = f.nextFetchID
	f.rawFetches = append(f.rawFetches, rf)
	return rf, nil
}

func (f *mainSourceCore) QueryRawFetchesByListing(_ context.Context, sourceID, sourceListingID string) ([]source.RawFetch, error) {
	var out []source.RawFetch
	for _, rf := range f.rawFetches {
		if rf.SourceID == sourceID && rf.SourceListingID == sourceListingID {
			out = append(out, rf)
		}
	}
	return out, nil
}

type mainListingCore struct {
	snapIDSeq int64
}

func newMainListingCore() *mainListingCore {
	return &mainListingCore{}
}

func (f *mainListingCore) UpsertFromParsed(_ context.Context, pl scraper.ParsedListing, rawFetchID int64, _ time.Time) (listing.Listing, listing.ListingSnapshot, error) {
	f.snapIDSeq++
	l := listing.Listing{ID: pl.SourceListingID + "-id", SourceID: pl.SourceID, SourceListingID: pl.SourceListingID}
	snap := listing.ListingSnapshot{ID: f.snapIDSeq, ListingID: l.ID, RawFetchID: rawFetchID}
	return l, snap, nil
}

func (f *mainListingCore) RecordParseAttempt(_ context.Context, pa listing.ParseAttempt) (listing.ParseAttempt, error) {
	return pa, nil
}

func (f *mainListingCore) QueryListingBySource(_ context.Context, _, _ string) (listing.Listing, error) {
	return listing.Listing{}, nil
}
