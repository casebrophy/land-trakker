package main

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/source"
	"github.com/cbrophy/land_trakker/foundation/scraper"
)

// TestSmoke_FakeBrokerSingleRun verifies that scrape-once can run FakeBroker once and report counts.
func TestSmoke_FakeBrokerSingleRun(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Create in-memory test fakes for the buses.
	sc := newTestSourceCore(source.Source{
		ID:          "fakebroker",
		DisplayName: "Fake Broker (Test Fixtures)",
		Enabled:     true,
	})
	lc := newTestListingCore()

	// Create the orchestrator.
	orch := scraper.NewOrchestrator(sc, lc, nil, log)

	// Create the FakeBroker scraper.
	broker := scraper.NewFakeBroker()

	// Run one tick with a 24-hour TTL.
	result, err := orch.RunOnce(ctx, broker, 24*time.Hour)
	if err != nil {
		t.Fatalf("RunOnce failed: %v", err)
	}

	// Verify that FakeBroker's 3 fixtures were discovered.
	if result.Discovered != 3 {
		t.Errorf("expected 3 discovered, got %d", result.Discovered)
	}

	// Verify that all were fetched and parsed.
	if result.Fetched != 3 {
		t.Errorf("expected 3 fetched, got %d", result.Fetched)
	}
	if result.Parsed != 3 {
		t.Errorf("expected 3 parsed, got %d", result.Parsed)
	}
	if result.Errors != 0 {
		t.Errorf("expected 0 errors, got %d", result.Errors)
	}

	// Verify that a run was recorded.
	if result.RunID == 0 {
		t.Error("expected non-zero RunID")
	}

	// Verify that parse attempts were recorded.
	if len(lc.parseAttempts) != 3 {
		t.Errorf("expected 3 parse attempts, got %d", len(lc.parseAttempts))
	}

	// Verify that upserts were called.
	if len(lc.upsertCalls) != 3 {
		t.Errorf("expected 3 upserts, got %d", len(lc.upsertCalls))
	}
}

// =============================================================================
// In-memory test fakes for the buses
// =============================================================================

type testSourceCore struct {
	src         source.Source
	runs        []source.ScrapeRun
	rawFetches  []source.RawFetch
	nextRunID   int64
	nextFetchID int64
}

func newTestSourceCore(src source.Source) *testSourceCore {
	return &testSourceCore{src: src}
}

func (f *testSourceCore) QuerySource(_ context.Context, _ string) (source.Source, error) {
	return f.src, nil
}

func (f *testSourceCore) QueryLatestRun(_ context.Context, _ string) (*source.ScrapeRun, error) {
	return nil, nil // No prior runs
}

func (f *testSourceCore) StartRun(_ context.Context, sourceID string, now time.Time) (source.ScrapeRun, error) {
	f.nextRunID++
	run := source.ScrapeRun{ID: f.nextRunID, SourceID: sourceID, StartedAt: now, Status: source.RunStatusRunning}
	f.runs = append(f.runs, run)
	return run, nil
}

func (f *testSourceCore) FinishRun(_ context.Context, run source.ScrapeRun) error {
	for i := range f.runs {
		if f.runs[i].ID == run.ID {
			f.runs[i] = run
			return nil
		}
	}
	return nil
}

func (f *testSourceCore) CreateRawFetch(_ context.Context, rf source.RawFetch) (source.RawFetch, error) {
	f.nextFetchID++
	rf.ID = f.nextFetchID
	f.rawFetches = append(f.rawFetches, rf)
	return rf, nil
}

func (f *testSourceCore) QueryRawFetchesByListing(_ context.Context, sourceID, sourceListingID string) ([]source.RawFetch, error) {
	var out []source.RawFetch
	for _, rf := range f.rawFetches {
		if rf.SourceID == sourceID && rf.SourceListingID == sourceListingID {
			out = append(out, rf)
		}
	}
	return out, nil
}

type testListingCore struct {
	upsertCalls   []upsertCall
	parseAttempts []listing.ParseAttempt
	snapIDSeq     int64
}

type upsertCall struct {
	pl         scraper.ParsedListing
	rawFetchID int64
}

func newTestListingCore() *testListingCore {
	return &testListingCore{}
}

func (f *testListingCore) UpsertFromParsed(_ context.Context, pl scraper.ParsedListing, rawFetchID int64, _ time.Time) (listing.Listing, listing.ListingSnapshot, error) {
	f.upsertCalls = append(f.upsertCalls, upsertCall{pl, rawFetchID})
	l := listing.Listing{ID: pl.SourceListingID + "-id", SourceID: pl.SourceID, SourceListingID: pl.SourceListingID}
	f.snapIDSeq++
	snap := listing.ListingSnapshot{ID: f.snapIDSeq, ListingID: l.ID, RawFetchID: rawFetchID}
	return l, snap, nil
}

func (f *testListingCore) RecordParseAttempt(_ context.Context, pa listing.ParseAttempt) (listing.ParseAttempt, error) {
	f.parseAttempts = append(f.parseAttempts, pa)
	return pa, nil
}

func (f *testListingCore) QueryListingBySource(_ context.Context, _, _ string) (listing.Listing, error) {
	return listing.Listing{}, nil
}
