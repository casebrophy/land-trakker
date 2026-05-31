package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/source"
	"github.com/cbrophy/land_trakker/foundation/scraper"
)

type fakeQuerier struct {
	ids []int64
}

func (f *fakeQuerier) QueryEligibleRawFetchIDs(_ context.Context, _, _ string) ([]int64, error) {
	return f.ids, nil
}

func TestDryRunReturnsCount(t *testing.T) {
	q := &fakeQuerier{ids: []int64{10, 20, 30}}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	n, err := dryRun(context.Background(), q, "landwatch", "landwatch.v1", log)
	if err != nil {
		t.Fatalf("dryRun: %v", err)
	}
	if n != 3 {
		t.Errorf("count = %d, want 3", n)
	}
}

func TestDryRunEmptySource(t *testing.T) {
	q := &fakeQuerier{ids: nil}
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	n, err := dryRun(context.Background(), q, "unused-source", "v1", log)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0", n)
	}
}

// e2eSource implements scraper's sourceCore interface AND rawFetchLoader.
// It records raw fetches with sequential IDs so tests can look them up by ID.
type e2eSource struct {
	src         source.Source
	runs        []source.ScrapeRun
	rawFetches  []source.RawFetch
	nextRunID   int64
	nextFetchID int64
}

func newE2ESource(src source.Source) *e2eSource {
	return &e2eSource{src: src}
}

func (f *e2eSource) QuerySource(_ context.Context, _ string) (source.Source, error) {
	return f.src, nil
}

func (f *e2eSource) QueryLatestRun(_ context.Context, _ string) (*source.ScrapeRun, error) {
	return nil, nil
}

func (f *e2eSource) StartRun(_ context.Context, sourceID string, now time.Time) (source.ScrapeRun, error) {
	f.nextRunID++
	run := source.ScrapeRun{ID: f.nextRunID, SourceID: sourceID, StartedAt: now, Status: source.RunStatusRunning}
	f.runs = append(f.runs, run)
	return run, nil
}

func (f *e2eSource) FinishRun(_ context.Context, run source.ScrapeRun) error {
	for i := range f.runs {
		if f.runs[i].ID == run.ID {
			f.runs[i] = run
			return nil
		}
	}
	return nil
}

func (f *e2eSource) CreateRawFetch(_ context.Context, rf source.RawFetch) (source.RawFetch, error) {
	f.nextFetchID++
	rf.ID = f.nextFetchID
	f.rawFetches = append(f.rawFetches, rf)
	return rf, nil
}

func (f *e2eSource) QueryRawFetchesByListing(_ context.Context, sourceID, sourceListingID string) ([]source.RawFetch, error) {
	var out []source.RawFetch
	for _, rf := range f.rawFetches {
		if rf.SourceID == sourceID && rf.SourceListingID == sourceListingID {
			out = append(out, rf)
		}
	}
	return out, nil
}

// QueryRawFetchByID implements rawFetchLoader.
func (f *e2eSource) QueryRawFetchByID(_ context.Context, id int64) (source.RawFetch, error) {
	for _, rf := range f.rawFetches {
		if rf.ID == id {
			return rf, nil
		}
	}
	return source.RawFetch{}, fmt.Errorf("raw fetch %d not found", id)
}

// e2eListing implements scraper's listingCore AND backfillListingCore.
type e2eUpsertCall struct {
	pl         scraper.ParsedListing
	rawFetchID int64
}

type e2eListing struct {
	upsertCalls   []e2eUpsertCall
	parseAttempts []listing.ParseAttempt
	snapIDSeq     int64
}

func (f *e2eListing) UpsertFromParsed(_ context.Context, pl scraper.ParsedListing, rawFetchID int64, _ time.Time) (listing.Listing, listing.ListingSnapshot, error) {
	f.upsertCalls = append(f.upsertCalls, e2eUpsertCall{pl, rawFetchID})
	l := listing.Listing{ID: pl.SourceListingID + "-id", SourceID: pl.SourceID, SourceListingID: pl.SourceListingID}
	f.snapIDSeq++
	snap := listing.ListingSnapshot{ID: f.snapIDSeq, ListingID: l.ID, RawFetchID: rawFetchID}
	return l, snap, nil
}

func (f *e2eListing) RecordParseAttempt(_ context.Context, pa listing.ParseAttempt) (listing.ParseAttempt, error) {
	f.parseAttempts = append(f.parseAttempts, pa)
	return pa, nil
}

func (f *e2eListing) QueryListingBySource(_ context.Context, _, _ string) (listing.Listing, error) {
	return listing.Listing{}, nil
}

// e2eEligibilityQuerier returns a fixed set of IDs (simulating the eligibility query after a version bump).
type e2eEligibilityQuerier struct {
	ids []int64
}

func (q *e2eEligibilityQuerier) QueryEligibleRawFetchIDs(_ context.Context, _, _ string) ([]int64, error) {
	return q.ids, nil
}

// fakeBrokerV2 wraps FakeBroker with a bumped parser version.
type fakeBrokerV2 struct {
	inner *scraper.FakeBroker
}

func newFakeBrokerV2() *fakeBrokerV2 {
	return &fakeBrokerV2{inner: scraper.NewFakeBroker()}
}

func (b *fakeBrokerV2) Source() scraper.Source        { return b.inner.Source() }
func (b *fakeBrokerV2) ParserVersion() string         { return "fakebroker.v2" }
func (b *fakeBrokerV2) Discover(ctx context.Context) ([]scraper.ListingRef, error) {
	return b.inner.Discover(ctx)
}
func (b *fakeBrokerV2) Fetch(ctx context.Context, ref scraper.ListingRef) (scraper.RawFetch, error) {
	return b.inner.Fetch(ctx, ref)
}
func (b *fakeBrokerV2) Parse(raw scraper.RawFetch) (scraper.ParsedListing, error) {
	return b.inner.Parse(raw)
}

// TestE2E_BackfillProducesFreshSnapshots verifies the full Phase 0 DoD backfill scenario:
// 1. FakeBroker v1 scrape via orchestrator → 3 raw_fetches + 3 snapshots
// 2. Version bumped to v2 → eligibility query returns original 3 raw_fetch IDs
// 3. runBackfill re-parses and produces 3 new snapshots referencing the original raw_fetch IDs
func TestE2E_BackfillProducesFreshSnapshots(t *testing.T) {
	ctx := context.Background()
	now := time.Now()
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// --- Phase 1: initial scrape with FakeBroker v1 ---
	src := source.Source{ID: "fakebroker", Enabled: true}
	fakeSource := newE2ESource(src)
	fakeListing := &e2eListing{}

	orch := scraper.NewOrchestrator(fakeSource, fakeListing, nil, log)
	result, err := orch.RunOnce(ctx, scraper.NewFakeBroker(), 0)
	if err != nil {
		t.Fatalf("initial scrape: %v", err)
	}
	if result.Parsed != 3 {
		t.Fatalf("initial scrape: expected 3 parsed, got %d", result.Parsed)
	}

	// Collect raw_fetch IDs created during the initial scrape.
	if len(fakeSource.rawFetches) != 3 {
		t.Fatalf("expected 3 raw_fetches after scrape, got %d", len(fakeSource.rawFetches))
	}
	originalIDs := make([]int64, len(fakeSource.rawFetches))
	for i, rf := range fakeSource.rawFetches {
		originalIDs[i] = rf.ID
	}

	// All initial parse attempts should use v1.
	for _, pa := range fakeListing.parseAttempts {
		if pa.ParserVersion != "fakebroker.v1" {
			t.Errorf("initial parse attempt used version %q, want fakebroker.v1", pa.ParserVersion)
		}
	}

	// --- Phase 2: bump version + run backfill ---
	// Eligibility querier returns the original raw_fetch IDs (simulating the DB query
	// that finds raw_fetches never parsed with fakebroker.v2).
	eligQ := &e2eEligibilityQuerier{ids: originalIDs}
	brokerV2 := newFakeBrokerV2()

	upsertsBefore := len(fakeListing.upsertCalls) // = 3

	bResult, err := runBackfill(ctx, eligQ, fakeSource, fakeListing, brokerV2, now, log)
	if err != nil {
		t.Fatalf("runBackfill: %v", err)
	}

	if bResult.Eligible != 3 {
		t.Errorf("Eligible = %d, want 3", bResult.Eligible)
	}
	if bResult.Parsed != 3 {
		t.Errorf("Parsed = %d, want 3", bResult.Parsed)
	}
	if bResult.Errors != 0 {
		t.Errorf("Errors = %d, want 0", bResult.Errors)
	}

	// --- Phase 3: verify new snapshots reference original raw_fetch IDs ---
	newUpserts := fakeListing.upsertCalls[upsertsBefore:]
	if len(newUpserts) != 3 {
		t.Fatalf("expected 3 new upserts after backfill, got %d", len(newUpserts))
	}

	newRawFetchIDs := make(map[int64]bool)
	for _, uc := range newUpserts {
		newRawFetchIDs[uc.rawFetchID] = true
	}
	for _, id := range originalIDs {
		if !newRawFetchIDs[id] {
			t.Errorf("backfill did not upsert with original raw_fetch_id=%d", id)
		}
	}

	// Verify backfill parse attempts used v2 and reference original raw_fetch IDs.
	var v2Attempts []listing.ParseAttempt
	for _, pa := range fakeListing.parseAttempts {
		if pa.ParserVersion == "fakebroker.v2" {
			v2Attempts = append(v2Attempts, pa)
		}
	}
	if len(v2Attempts) != 3 {
		t.Errorf("expected 3 v2 parse attempts, got %d", len(v2Attempts))
	}
	v2AttemptRawIDs := make(map[int64]bool)
	for _, pa := range v2Attempts {
		v2AttemptRawIDs[pa.RawFetchID] = true
	}
	for _, id := range originalIDs {
		if !v2AttemptRawIDs[id] {
			t.Errorf("v2 parse attempt missing original raw_fetch_id=%d", id)
		}
	}
}
