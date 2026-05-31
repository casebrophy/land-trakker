package scraper

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/source"
)

// =============================================================================
// DiffRefs unit tests
// =============================================================================

func TestDiffRefs_AllNew(t *testing.T) {
	added, kept, removed := DiffRefs(nil, []string{"a", "b", "c"})
	if len(removed) != 0 {
		t.Errorf("want 0 removed, got %d", len(removed))
	}
	if len(kept) != 0 {
		t.Errorf("want 0 kept, got %d", len(kept))
	}
	if len(added) != 3 {
		t.Errorf("want 3 added, got %d", len(added))
	}
}

func TestDiffRefs_AllKept(t *testing.T) {
	ids := []string{"a", "b", "c"}
	added, kept, removed := DiffRefs(ids, ids)
	if len(added) != 0 {
		t.Errorf("want 0 added, got %d", len(added))
	}
	if len(removed) != 0 {
		t.Errorf("want 0 removed, got %d", len(removed))
	}
	if len(kept) != 3 {
		t.Errorf("want 3 kept, got %d", len(kept))
	}
}

func TestDiffRefs_AllRemoved(t *testing.T) {
	added, kept, removed := DiffRefs([]string{"a", "b"}, nil)
	if len(added) != 0 {
		t.Errorf("want 0 added, got %d", len(added))
	}
	if len(kept) != 0 {
		t.Errorf("want 0 kept, got %d", len(kept))
	}
	if len(removed) != 2 {
		t.Errorf("want 2 removed, got %d", len(removed))
	}
}

func TestDiffRefs_Mixed(t *testing.T) {
	prev := []string{"a", "b", "c"}
	curr := []string{"b", "c", "d"}
	added, kept, removed := DiffRefs(prev, curr)

	sortedAdded := sorted(added)
	sortedKept := sorted(kept)
	sortedRemoved := sorted(removed)

	if len(sortedAdded) != 1 || sortedAdded[0] != "d" {
		t.Errorf("want added=[d], got %v", sortedAdded)
	}
	if len(sortedKept) != 2 || sortedKept[0] != "b" || sortedKept[1] != "c" {
		t.Errorf("want kept=[b,c], got %v", sortedKept)
	}
	if len(sortedRemoved) != 1 || sortedRemoved[0] != "a" {
		t.Errorf("want removed=[a], got %v", sortedRemoved)
	}
}

func TestDiffRefs_EmptyBoth(t *testing.T) {
	added, kept, removed := DiffRefs(nil, nil)
	if len(added)+len(kept)+len(removed) != 0 {
		t.Error("want all empty slices")
	}
}

func sorted(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	sort.Strings(out)
	return out
}

// =============================================================================
// In-memory fakes for orchestrator integration tests
// =============================================================================

type fakeSourceCore struct {
	src         source.Source
	priorRun    *source.ScrapeRun
	runs        []source.ScrapeRun
	rawFetches  []source.RawFetch
	nextRunID   int64
	nextFetchID int64
}

func newFakeSourceCore(src source.Source) *fakeSourceCore {
	return &fakeSourceCore{src: src}
}

func (f *fakeSourceCore) QuerySource(_ context.Context, _ string) (source.Source, error) {
	return f.src, nil
}

func (f *fakeSourceCore) QueryLatestRun(_ context.Context, _ string) (*source.ScrapeRun, error) {
	return f.priorRun, nil
}

func (f *fakeSourceCore) StartRun(_ context.Context, sourceID string, now time.Time) (source.ScrapeRun, error) {
	f.nextRunID++
	run := source.ScrapeRun{ID: f.nextRunID, SourceID: sourceID, StartedAt: now, Status: source.RunStatusRunning}
	f.runs = append(f.runs, run)
	return run, nil
}

func (f *fakeSourceCore) FinishRun(_ context.Context, run source.ScrapeRun) error {
	for i := range f.runs {
		if f.runs[i].ID == run.ID {
			f.runs[i] = run
			return nil
		}
	}
	return errors.New("run not found")
}

func (f *fakeSourceCore) CreateRawFetch(_ context.Context, rf source.RawFetch) (source.RawFetch, error) {
	f.nextFetchID++
	rf.ID = f.nextFetchID
	f.rawFetches = append(f.rawFetches, rf)
	return rf, nil
}

func (f *fakeSourceCore) QueryRawFetchesByListing(_ context.Context, sourceID, sourceListingID string) ([]source.RawFetch, error) {
	var out []source.RawFetch
	for _, rf := range f.rawFetches {
		if rf.SourceID == sourceID && rf.SourceListingID == sourceListingID {
			out = append(out, rf)
		}
	}
	return out, nil
}

// fakeListingCore records calls for assertion in tests.
type fakeListingCore struct {
	upsertCalls   []upsertCall
	parseAttempts []listing.ParseAttempt
	snapIDSeq     int64
}

type upsertCall struct {
	pl         ParsedListing
	rawFetchID int64
}

var errNotFound = errors.New("not found")

func (f *fakeListingCore) UpsertFromParsed(_ context.Context, pl ParsedListing, rawFetchID int64, _ time.Time) (listing.Listing, listing.ListingSnapshot, error) {
	f.upsertCalls = append(f.upsertCalls, upsertCall{pl, rawFetchID})
	l := listing.Listing{ID: pl.SourceListingID + "-id", SourceID: pl.SourceID, SourceListingID: pl.SourceListingID}
	f.snapIDSeq++
	snap := listing.ListingSnapshot{ID: f.snapIDSeq, ListingID: l.ID, RawFetchID: rawFetchID}
	return l, snap, nil
}

func (f *fakeListingCore) RecordParseAttempt(_ context.Context, pa listing.ParseAttempt) (listing.ParseAttempt, error) {
	f.parseAttempts = append(f.parseAttempts, pa)
	return pa, nil
}

func (f *fakeListingCore) QueryListingBySource(_ context.Context, _, _ string) (listing.Listing, error) {
	return listing.Listing{}, errNotFound
}

// =============================================================================
// Orchestrator integration tests
// =============================================================================

func newTestOrchestrator(sc *fakeSourceCore, lc *fakeListingCore) *Orchestrator {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return NewOrchestrator(sc, lc, nil, log)
}

func TestOrchestratorRunOnce_BasicPipeline(t *testing.T) {
	src := source.Source{
		ID:      "fakebroker",
		Enabled: true,
	}
	sc := newFakeSourceCore(src)
	lc := &fakeListingCore{}
	orch := newTestOrchestrator(sc, lc)

	result, err := orch.RunOnce(context.Background(), NewFakeBroker(), 0)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}

	if result.Discovered != 3 {
		t.Errorf("Discovered = %d, want 3", result.Discovered)
	}
	if result.Fetched != 3 {
		t.Errorf("Fetched = %d, want 3", result.Fetched)
	}
	if result.Parsed != 3 {
		t.Errorf("Parsed = %d, want 3", result.Parsed)
	}
	if result.Errors != 0 {
		t.Errorf("Errors = %d, want 0", result.Errors)
	}
	if result.RunID == 0 {
		t.Error("RunID should be non-zero")
	}

	// Verify storage side-effects.
	if len(sc.rawFetches) != 3 {
		t.Errorf("raw fetches stored = %d, want 3", len(sc.rawFetches))
	}
	if len(sc.runs) != 1 {
		t.Fatalf("runs stored = %d, want 1", len(sc.runs))
	}
	run := sc.runs[0]
	if run.Status != source.RunStatusOK {
		t.Errorf("run.Status = %q, want %q", run.Status, source.RunStatusOK)
	}
	if run.FinishedAt == nil {
		t.Error("run.FinishedAt should be set")
	}

	if len(lc.upsertCalls) != 3 {
		t.Errorf("upsert calls = %d, want 3", len(lc.upsertCalls))
	}
	if len(lc.parseAttempts) != 3 {
		t.Errorf("parse attempts = %d, want 3", len(lc.parseAttempts))
	}
	for _, pa := range lc.parseAttempts {
		if pa.Outcome != listing.OutcomeSuccess {
			t.Errorf("parse attempt outcome = %q, want success", pa.Outcome)
		}
		if pa.SnapshotID == nil {
			t.Error("parse attempt SnapshotID should be set on success")
		}
	}
}

func TestOrchestratorRunOnce_TTLSkipsFreshListings(t *testing.T) {
	src := source.Source{
		ID:      "fakebroker",
		Enabled: true,
	}
	sc := newFakeSourceCore(src)
	lc := &fakeListingCore{}
	orch := newTestOrchestrator(sc, lc)

	// First run with fetchTTL=0 (always fetch) → populates raw fetches.
	if _, err := orch.RunOnce(context.Background(), NewFakeBroker(), 0); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// Second run with 24-hour TTL — raw fetches from the first run are fresh.
	result2, err := orch.RunOnce(context.Background(), NewFakeBroker(), 24*time.Hour)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if result2.Fetched != 0 {
		t.Errorf("second run Fetched = %d, want 0 (TTL not expired)", result2.Fetched)
	}
	if result2.Discovered != 3 {
		t.Errorf("second run Discovered = %d, want 3", result2.Discovered)
	}
}

func TestOrchestratorRunOnce_DisabledSource(t *testing.T) {
	src := source.Source{
		ID:      "fakebroker",
		Enabled: false,
	}
	sc := newFakeSourceCore(src)
	lc := &fakeListingCore{}
	orch := newTestOrchestrator(sc, lc)

	result, err := orch.RunOnce(context.Background(), NewFakeBroker(), 0)
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if result.RunID != 0 {
		t.Error("disabled source should produce no run")
	}
	if len(sc.runs) != 0 {
		t.Error("no runs should be started for disabled source")
	}
	if len(sc.rawFetches) != 0 {
		t.Error("no raw fetches should be stored for disabled source")
	}
}

func TestOrchestratorRunOnce_DiffTracksRemovals(t *testing.T) {
	src := source.Source{
		ID:      "fakebroker",
		Enabled: true,
	}
	sc := newFakeSourceCore(src)

	var missedIDs []string
	missedHandler := func(_ context.Context, _, sourceListingID string, _ source.Source, _ bool, _ time.Time) error {
		missedIDs = append(missedIDs, sourceListingID)
		return nil
	}

	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	orch := NewOrchestrator(sc, &fakeListingCore{}, missedHandler, log)

	// First run: discover all 3 fixtures.
	if _, err := orch.RunOnce(context.Background(), NewFakeBroker(), 0); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if len(missedIDs) != 0 {
		t.Errorf("first run should have no missed IDs, got %v", missedIDs)
	}

	// Seed prevDiscovered with an extra ID that won't appear in Discover().
	orch.prevDiscovered["fakebroker"] = append(orch.prevDiscovered["fakebroker"], "ghost-listing")

	// Second run: "ghost-listing" is absent → missedHandler should be called once.
	if _, err := orch.RunOnce(context.Background(), NewFakeBroker(), 0); err != nil {
		t.Fatalf("second run: %v", err)
	}
	if len(missedIDs) != 1 || missedIDs[0] != "ghost-listing" {
		t.Errorf("want missedIDs=[ghost-listing], got %v", missedIDs)
	}
}

func TestOrchestratorRunOnce_SHA256Stored(t *testing.T) {
	src := source.Source{ID: "fakebroker", Enabled: true}
	sc := newFakeSourceCore(src)
	lc := &fakeListingCore{}
	orch := newTestOrchestrator(sc, lc)

	if _, err := orch.RunOnce(context.Background(), NewFakeBroker(), 0); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	for _, rf := range sc.rawFetches {
		if len(rf.BodySHA256) != 32 {
			t.Errorf("listing %s: BodySHA256 len = %d, want 32", rf.SourceListingID, len(rf.BodySHA256))
		}
	}
}
