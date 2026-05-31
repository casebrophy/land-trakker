package listingbus_test

import (
	"context"
	"log/slog"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/sdk/listingbus"
	"github.com/cbrophy/land_trakker/foundation/scraper"
	"github.com/jackc/pgx/v5"
)

// =============================================================================
// fakeStore — in-memory implementation of listing.Storer
// =============================================================================

type fakeStore struct {
	listings      map[string]listing.Listing          // id → listing
	bySource      map[string]string                    // "sourceID:sourceListingID" → id
	snapshots     map[string][]listing.ListingSnapshot // listingID → snaps (DESC)
	priceChanges  []listing.PriceChange
	parseAttempts []listing.ParseAttempt
	nextSnapID    int64
	nextListSeq   int
	nextPAID      int64
	eligibleIDs   []int64
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		listings:   make(map[string]listing.Listing),
		bySource:   make(map[string]string),
		snapshots:  make(map[string][]listing.ListingSnapshot),
		nextSnapID: 1,
	}
}

func (f *fakeStore) sourceKey(sourceID, sourceListingID string) string {
	return sourceID + ":" + sourceListingID
}

func (f *fakeStore) CreateListing(_ context.Context, l listing.Listing) (listing.Listing, error) {
	f.nextListSeq++
	l.ID = "listing-" + itoa(f.nextListSeq)
	f.listings[l.ID] = l
	f.bySource[f.sourceKey(l.SourceID, l.SourceListingID)] = l.ID
	return l, nil
}

func (f *fakeStore) UpdateListing(_ context.Context, l listing.Listing) error {
	f.listings[l.ID] = l
	return nil
}

func (f *fakeStore) QueryListingByID(_ context.Context, id string) (listing.Listing, error) {
	l, ok := f.listings[id]
	if !ok {
		return listing.Listing{}, pgx.ErrNoRows
	}
	return l, nil
}

func (f *fakeStore) QueryListingBySource(_ context.Context, sourceID, sourceListingID string) (listing.Listing, error) {
	id, ok := f.bySource[f.sourceKey(sourceID, sourceListingID)]
	if !ok {
		return listing.Listing{}, pgx.ErrNoRows
	}
	l, ok := f.listings[id]
	if !ok {
		return listing.Listing{}, pgx.ErrNoRows
	}
	return l, nil
}

func (f *fakeStore) CreateSnapshot(_ context.Context, snap listing.ListingSnapshot) (listing.ListingSnapshot, error) {
	snap.ID = f.nextSnapID
	f.nextSnapID++
	snaps := f.snapshots[snap.ListingID]
	snaps = append([]listing.ListingSnapshot{snap}, snaps...) // prepend → DESC order
	f.snapshots[snap.ListingID] = snaps
	return snap, nil
}

func (f *fakeStore) QuerySnapshotsByListing(_ context.Context, listingID string) ([]listing.ListingSnapshot, error) {
	snaps := f.snapshots[listingID]
	// ensure DESC by captured_at
	sorted := make([]listing.ListingSnapshot, len(snaps))
	copy(sorted, snaps)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CapturedAt.After(sorted[j].CapturedAt)
	})
	return sorted, nil
}

func (f *fakeStore) CreatePriceChange(_ context.Context, pc listing.PriceChange) (listing.PriceChange, error) {
	f.priceChanges = append(f.priceChanges, pc)
	return pc, nil
}

func (f *fakeStore) QueryPriceChangesByListing(_ context.Context, listingID string) ([]listing.PriceChange, error) {
	var out []listing.PriceChange
	for _, pc := range f.priceChanges {
		if pc.ListingID == listingID {
			out = append(out, pc)
		}
	}
	return out, nil
}

func (f *fakeStore) CreateParseAttempt(_ context.Context, pa listing.ParseAttempt) (listing.ParseAttempt, error) {
	f.nextPAID++
	pa.ID = f.nextPAID
	f.parseAttempts = append(f.parseAttempts, pa)
	return pa, nil
}

func (f *fakeStore) QueryEligibleRawFetchIDs(_ context.Context, _, _ string) ([]int64, error) {
	return f.eligibleIDs, nil
}

func itoa(n int) string {
	return string(rune('0' + n)) // simple for tests (works up to 9)
}

// =============================================================================
// helpers
// =============================================================================

func newCore(store listing.Storer) *listingbus.Core {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return listingbus.NewCore(store, log)
}

func baseParsed() scraper.ParsedListing {
	price := int64(50000)
	acres := 10.5
	return scraper.ParsedListing{
		SourceID:        "src1",
		SourceListingID: "list1",
		URL:             "https://example.com/list1",
		Title:           "Nice Land",
		Description:     "Great plot",
		PriceCents:      &price,
		Acres:           &acres,
	}
}

var defaultMissedCfg = listingbus.MissedRunConfig{
	AbsenceDaysBeforeStale:         14,
	AbsenceDaysBeforeInactive:      30,
	ConsecutiveMissedRunsThreshold: 3,
}

// =============================================================================
// UpsertFromParsed tests
// =============================================================================

func TestUpsertFromParsed_NewListing(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	now := time.Now()
	pl := baseParsed()

	got, snap, err := core.UpsertFromParsed(context.Background(), pl, 1, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Status != listing.StatusActive {
		t.Errorf("want StatusActive, got %q", got.Status)
	}
	if got.ConsecutiveMisses != 0 {
		t.Errorf("want ConsecutiveMisses=0, got %d", got.ConsecutiveMisses)
	}
	if len(snap.Diff) != 0 {
		t.Errorf("want empty diff, got %v", snap.Diff)
	}
}

func TestUpsertFromParsed_Reappearance(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	now := time.Now()
	pl := baseParsed()

	// Seed an existing listing with presumed_inactive
	existing := listing.Listing{
		ID:              "listing-1",
		SourceID:        pl.SourceID,
		SourceListingID: pl.SourceListingID,
		URL:             pl.URL,
		Status:          listing.StatusPresumedInactive,
		ConsecutiveMisses: 5,
		FirstSeenAt:     now.Add(-30 * 24 * time.Hour),
		LastSeenAt:      now.Add(-20 * 24 * time.Hour),
	}
	store.listings[existing.ID] = existing
	store.bySource[store.sourceKey(existing.SourceID, existing.SourceListingID)] = existing.ID

	got, _, err := core.UpsertFromParsed(context.Background(), pl, 2, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != listing.StatusActive {
		t.Errorf("want StatusActive on reappearance, got %q", got.Status)
	}
	if got.ConsecutiveMisses != 0 {
		t.Errorf("want ConsecutiveMisses reset to 0, got %d", got.ConsecutiveMisses)
	}
}

func TestUpsertFromParsed_PriceChange(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	now := time.Now()
	pl := baseParsed()

	// First upsert to create listing + initial snapshot (price=50000)
	_, _, err := core.UpsertFromParsed(context.Background(), pl, 1, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Second upsert with new price=48000
	newPrice := int64(48000)
	pl.PriceCents = &newPrice
	_, _, err = core.UpsertFromParsed(context.Background(), pl, 2, now)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if len(store.priceChanges) != 1 {
		t.Fatalf("want 1 price change, got %d", len(store.priceChanges))
	}
	pc := store.priceChanges[0]
	if pc.NewPriceCents != 48000 {
		t.Errorf("want new price 48000, got %d", pc.NewPriceCents)
	}
}

func TestUpsertFromParsed_NoPriceChangeIfSamePrice(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	now := time.Now()
	pl := baseParsed()

	_, _, err := core.UpsertFromParsed(context.Background(), pl, 1, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	// Same price
	_, _, err = core.UpsertFromParsed(context.Background(), pl, 2, now)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	if len(store.priceChanges) != 0 {
		t.Errorf("want no price changes, got %d", len(store.priceChanges))
	}
}

func TestUpsertFromParsed_SourceStatusSold(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	now := time.Now()
	pl := baseParsed()

	// Create active listing first
	_, _, err := core.UpsertFromParsed(context.Background(), pl, 1, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	pl.SourceStatus = "sold"
	got, _, err := core.UpsertFromParsed(context.Background(), pl, 2, now)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if got.Status != listing.StatusConfirmedSold {
		t.Errorf("want StatusConfirmedSold, got %q", got.Status)
	}
}

func TestUpsertFromParsed_SourceStatusWithdrawn(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	now := time.Now()
	pl := baseParsed()

	_, _, err := core.UpsertFromParsed(context.Background(), pl, 1, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("first upsert: %v", err)
	}

	pl.SourceStatus = "withdrawn"
	got, _, err := core.UpsertFromParsed(context.Background(), pl, 2, now)
	if err != nil {
		t.Fatalf("second upsert: %v", err)
	}
	if got.Status != listing.StatusWithdrawn {
		t.Errorf("want StatusWithdrawn, got %q", got.Status)
	}
}

// =============================================================================
// ApplyMissedRun tests
// =============================================================================

func seedListing(store *fakeStore, id string, status listing.ListingStatus, misses int, lastSeen time.Time) {
	l := listing.Listing{
		ID:                id,
		SourceID:          "src1",
		SourceListingID:   id,
		Status:            status,
		ConsecutiveMisses: misses,
		FirstSeenAt:       lastSeen.Add(-30 * 24 * time.Hour),
		LastSeenAt:        lastSeen,
	}
	store.listings[id] = l
	store.bySource[store.sourceKey(l.SourceID, l.SourceListingID)] = id
}

func TestApplyMissedRun_BelowThreshold(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	now := time.Now()
	seedListing(store, "l1", listing.StatusActive, 2, now.Add(-5*24*time.Hour))

	cfg := listingbus.MissedRunConfig{
		AbsenceDaysBeforeStale:         14,
		AbsenceDaysBeforeInactive:      30,
		ConsecutiveMissedRunsThreshold: 3,
	}
	got, err := core.ApplyMissedRun(context.Background(), "l1", cfg, true, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != listing.StatusActive {
		t.Errorf("want StatusActive, got %q", got.Status)
	}
	if got.ConsecutiveMisses != 3 {
		t.Errorf("want ConsecutiveMisses=3, got %d", got.ConsecutiveMisses)
	}
}

func TestApplyMissedRun_ActiveToStale(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	now := time.Now()
	lastSeen := now.Add(-20 * 24 * time.Hour)
	seedListing(store, "l1", listing.StatusActive, 2, lastSeen)

	cfg := listingbus.MissedRunConfig{
		AbsenceDaysBeforeStale:         14,
		AbsenceDaysBeforeInactive:      30,
		ConsecutiveMissedRunsThreshold: 3,
	}
	got, err := core.ApplyMissedRun(context.Background(), "l1", cfg, true, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != listing.StatusStale {
		t.Errorf("want StatusStale, got %q", got.Status)
	}
}

func TestApplyMissedRun_ActiveToPresumedInactive(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	now := time.Now()
	lastSeen := now.Add(-35 * 24 * time.Hour)
	seedListing(store, "l1", listing.StatusActive, 2, lastSeen)

	cfg := listingbus.MissedRunConfig{
		AbsenceDaysBeforeStale:         14,
		AbsenceDaysBeforeInactive:      30,
		ConsecutiveMissedRunsThreshold: 3,
	}
	got, err := core.ApplyMissedRun(context.Background(), "l1", cfg, true, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != listing.StatusPresumedInactive {
		t.Errorf("want StatusPresumedInactive, got %q", got.Status)
	}
}

func TestApplyMissedRun_UnhealthyRunNoTransition(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	now := time.Now()
	lastSeen := now.Add(-40 * 24 * time.Hour)
	seedListing(store, "l1", listing.StatusActive, 4, lastSeen)

	cfg := listingbus.MissedRunConfig{
		AbsenceDaysBeforeStale:         14,
		AbsenceDaysBeforeInactive:      30,
		ConsecutiveMissedRunsThreshold: 3,
	}
	got, err := core.ApplyMissedRun(context.Background(), "l1", cfg, false, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != listing.StatusActive {
		t.Errorf("want StatusActive (unhealthy run), got %q", got.Status)
	}
}

func TestApplyMissedRun_TerminalStateUnchanged(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	now := time.Now()
	seedListing(store, "l1", listing.StatusConfirmedSold, 0, now.Add(-5*24*time.Hour))

	got, err := core.ApplyMissedRun(context.Background(), "l1", defaultMissedCfg, true, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Status != listing.StatusConfirmedSold {
		t.Errorf("want StatusConfirmedSold unchanged, got %q", got.Status)
	}
}

// =============================================================================
// RecordParseAttempt tests
// =============================================================================

func TestRecordParseAttempt(t *testing.T) {
	fs := newFakeStore()
	core := newCore(fs)
	ctx := context.Background()

	pa := listing.ParseAttempt{
		RawFetchID:    42,
		ParserVersion: "landwatch.v1",
		AttemptedAt:   time.Now().UTC(),
		Outcome:       listing.OutcomeSuccess,
	}
	got, err := core.RecordParseAttempt(ctx, pa)
	if err != nil {
		t.Fatalf("RecordParseAttempt: %v", err)
	}
	if got.ID == 0 {
		t.Error("expected non-zero ID")
	}
	if got.RawFetchID != 42 {
		t.Errorf("RawFetchID = %d, want 42", got.RawFetchID)
	}
	if len(fs.parseAttempts) != 1 {
		t.Errorf("expected 1 stored parse attempt, got %d", len(fs.parseAttempts))
	}
}
