package listingbus_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/sdk/listingbus"
	"github.com/cbrophy/land_trakker/foundation/geocode"
	"github.com/cbrophy/land_trakker/foundation/scraper"
	"github.com/jackc/pgx/v5"
)

// =============================================================================
// fakeStore — in-memory implementation of listing.Storer
// =============================================================================

type fakeStore struct {
	listings      map[string]listing.Listing                  // id → listing
	bySource      map[string]string                           // "sourceID:sourceListingID" → id
	snapshots     map[string][]listing.ListingSnapshot        // listingID → snaps (DESC)
	priceChanges  []listing.PriceChange
	parseAttempts []listing.ParseAttempt
	duplicates    map[string]listing.PossibleDuplicate        // "aID|bID" → PossibleDuplicate
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
		duplicates: make(map[string]listing.PossibleDuplicate),
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

func (f *fakeStore) QueryListingsForDedup(_ context.Context) ([]listing.Listing, error) {
	var out []listing.Listing
	for _, l := range f.listings {
		if l.Status == listing.StatusActive || l.Status == listing.StatusStale {
			out = append(out, l)
		}
	}
	return out, nil
}

func (f *fakeStore) UpsertPossibleDuplicate(_ context.Context, pd listing.PossibleDuplicate) error {
	key := pd.ListingAID + "|" + pd.ListingBID
	f.duplicates[key] = pd
	return nil
}

func (f *fakeStore) QueryPossibleDuplicates(_ context.Context, decision *string) ([]listing.PossibleDuplicate, error) {
	var out []listing.PossibleDuplicate
	for _, pd := range f.duplicates {
		if decision == nil {
			out = append(out, pd)
			continue
		}
		if pd.UserDecision != nil && *pd.UserDecision == *decision {
			out = append(out, pd)
		} else if pd.UserDecision == nil && *decision == "" {
			out = append(out, pd)
		}
	}
	return out, nil
}

func (f *fakeStore) UpdateDuplicateDecision(_ context.Context, aID, bID string, decision string) error {
	key := aID + "|" + bID
	pd, ok := f.duplicates[key]
	if !ok {
		return fmt.Errorf("not found")
	}
	pd.UserDecision = &decision
	f.duplicates[key] = pd
	return nil
}

func (f *fakeStore) QueryListings(_ context.Context, limit, offset int) ([]listing.Listing, error) {
	all := make([]listing.Listing, 0, len(f.listings))
	for _, l := range f.listings {
		all = append(all, l)
	}
	if offset >= len(all) {
		return nil, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end], nil
}

func (f *fakeStore) QueryListingsFilter(_ context.Context, filter listing.ListingFilter, limit, offset int) ([]listing.Listing, error) {
	var matched []listing.Listing
	for _, l := range f.listings {
		if filter.AcresMin != nil && (l.Acres == nil || *l.Acres < *filter.AcresMin) {
			continue
		}
		if filter.AcresMax != nil && (l.Acres == nil || *l.Acres > *filter.AcresMax) {
			continue
		}
		if filter.PriceMin != nil && (l.PriceCents == nil || *l.PriceCents < *filter.PriceMin) {
			continue
		}
		if filter.PriceMax != nil && (l.PriceCents == nil || *l.PriceCents > *filter.PriceMax) {
			continue
		}
		if len(filter.Counties) > 0 {
			found := false
			for _, c := range filter.Counties {
				if l.County != nil && *l.County == c {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if filter.PPAMin != nil && (l.PricePerAcreCents == nil || *l.PricePerAcreCents < *filter.PPAMin) {
			continue
		}
		if filter.PPAMax != nil && (l.PricePerAcreCents == nil || *l.PricePerAcreCents > *filter.PPAMax) {
			continue
		}
		if filter.PropertyType != nil && (l.AttrPropertyType == nil || *l.AttrPropertyType != *filter.PropertyType) {
			continue
		}
		if filter.AttrWaterFrontage != nil && (l.AttrWaterFrontage == nil || *l.AttrWaterFrontage != *filter.AttrWaterFrontage) {
			continue
		}
		if filter.AttrOffGrid != nil && (l.AttrOffGrid == nil || *l.AttrOffGrid != *filter.AttrOffGrid) {
			continue
		}
		if filter.AttrPower != nil && (l.AttrPower == nil || *l.AttrPower != *filter.AttrPower) {
			continue
		}
		if filter.AttrWell != nil && (l.AttrWell == nil || *l.AttrWell != *filter.AttrWell) {
			continue
		}
		if filter.AttrSeptic != nil && (l.AttrSeptic == nil || *l.AttrSeptic != *filter.AttrSeptic) {
			continue
		}
		matched = append(matched, l)
	}
	if offset >= len(matched) {
		return nil, nil
	}
	end := offset + limit
	if end > len(matched) {
		end = len(matched)
	}
	return matched[offset:end], nil
}

func itoa(n int) string {
	return string(rune('0' + n)) // simple for tests (works up to 9)
}

// =============================================================================
// mockGeocoder — test mock for geocode.Geocoder
// =============================================================================

// fakeSuccessGeocoder always returns a valid geocoding result.
type fakeSuccessGeocoder struct{}

func (fakeSuccessGeocoder) Geocode(ctx context.Context, address, city, county, state string) (geocode.Result, error) {
	return geocode.Result{
		Lat:        43.6150,
		Lng:        -116.2023,
		Precision:  geocode.PrecisionRooftop,
		Provider:   "fake",
		Confidence: 0.99,
	}, nil
}

// fakeDailyCapGeocoder returns ErrDailyLimitExceeded.
type fakeDailyCapGeocoder struct{}

func (fakeDailyCapGeocoder) Geocode(ctx context.Context, address, city, county, state string) (geocode.Result, error) {
	return geocode.Result{}, geocode.ErrDailyLimitExceeded
}

// fakeErrorGeocoder returns a non-cap error.
type fakeErrorGeocoder struct{}

func (fakeErrorGeocoder) Geocode(ctx context.Context, address, city, county, state string) (geocode.Result, error) {
	return geocode.Result{}, errors.New("fake geocode error")
}

// trackingGeocoder records calls for verification.
type trackingGeocoder struct {
	calls []geocodeCall
}

type geocodeCall struct {
	address string
	city    string
	county  string
	state   string
}

func (t *trackingGeocoder) Geocode(ctx context.Context, address, city, county, state string) (geocode.Result, error) {
	t.calls = append(t.calls, geocodeCall{address, city, county, state})
	return geocode.Result{
		Lat:        43.6150,
		Lng:        -116.2023,
		Precision:  geocode.PrecisionRooftop,
		Provider:   "fake",
		Confidence: 0.99,
	}, nil
}

// =============================================================================
// helpers
// =============================================================================

func newCore(store listing.Storer) *listingbus.Core {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return listingbus.NewCore(store, fakeSuccessGeocoder{}, log)
}

func newCoreWithGeocoder(store listing.Storer, geocoder geocode.Geocoder) *listingbus.Core {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return listingbus.NewCore(store, geocoder, log)
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

// =============================================================================
// Geocoding integration tests
// =============================================================================

func TestUpsertFromParsed_Geocoding_Success(t *testing.T) {
	store := newFakeStore()
	core := newCoreWithGeocoder(store, fakeSuccessGeocoder{})
	now := time.Now()
	pl := baseParsed()

	// Add address to parsed listing
	addressLine := "123 Main St"
	city := "Boise"
	county := "Ada"
	state := "ID"
	pl.Address = &scraper.Address{
		Street: addressLine,
		City:   city,
		County: county,
		State:  state,
		Zip:    "83702",
	}

	got, _, err := core.UpsertFromParsed(context.Background(), pl, 1, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Geom == nil {
		t.Error("expected Geom to be populated after successful geocoding")
	} else {
		if got.Geom.Lat != 43.6150 {
			t.Errorf("want Lat=43.6150, got %f", got.Geom.Lat)
		}
		if got.Geom.Lng != -116.2023 {
			t.Errorf("want Lng=-116.2023, got %f", got.Geom.Lng)
		}
	}
}

func TestUpsertFromParsed_Geocoding_DailyCapExceeded(t *testing.T) {
	store := newFakeStore()
	core := newCoreWithGeocoder(store, fakeDailyCapGeocoder{})
	now := time.Now()
	pl := baseParsed()

	// Add address
	pl.Address = &scraper.Address{
		Street: "123 Main St",
		City:   "Boise",
		County: "Ada",
		State:  "ID",
		Zip:    "83702",
	}

	got, _, err := core.UpsertFromParsed(context.Background(), pl, 1, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Geom != nil {
		t.Errorf("expected Geom to remain nil when daily cap exceeded, got %v", got.Geom)
	}
}

func TestUpsertFromParsed_Geocoding_NullAddress(t *testing.T) {
	store := newFakeStore()
	tracker := &trackingGeocoder{}
	core := newCoreWithGeocoder(store, tracker)
	now := time.Now()
	pl := baseParsed()

	// No address set
	pl.Address = nil

	got, _, err := core.UpsertFromParsed(context.Background(), pl, 1, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Geom != nil {
		t.Errorf("expected Geom to remain nil for null address, got %v", got.Geom)
	}

	if len(tracker.calls) != 0 {
		t.Errorf("expected no geocoding calls for null address, got %d calls", len(tracker.calls))
	}
}

func TestUpsertFromParsed_Geocoding_Error(t *testing.T) {
	store := newFakeStore()
	core := newCoreWithGeocoder(store, fakeErrorGeocoder{})
	now := time.Now()
	pl := baseParsed()

	// Add address
	pl.Address = &scraper.Address{
		Street: "123 Main St",
		City:   "Boise",
		County: "Ada",
		State:  "ID",
		Zip:    "83702",
	}

	got, _, err := core.UpsertFromParsed(context.Background(), pl, 1, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Geom != nil {
		t.Errorf("expected Geom to remain nil on geocoding error, got %v", got.Geom)
	}
	// Upsert should succeed despite geocoding error
	if got.ID == "" {
		t.Error("expected listing to be created despite geocoding error")
	}
}

// =============================================================================
// QueryListingsFilter tests
// =============================================================================

func ptrF(v float64) *float64 { return &v }
func ptrI(v int64) *int64     { return &v }
func ptrS(v string) *string   { return &v }
func ptrB(v bool) *bool       { return &v }

// seedFilterListing inserts a listing directly into the fakeStore.
func seedFilterListing(store *fakeStore, id string, acres *float64, priceCents *int64, county *string, propType *string, well *bool) {
	store.nextListSeq++
	l := listing.Listing{
		ID:               id,
		SourceID:         "src-filter",
		SourceListingID:  id,
		Status:           listing.StatusActive,
		FirstSeenAt:      time.Now(),
		LastSeenAt:       time.Now(),
		Acres:            acres,
		PriceCents:       priceCents,
		County:           county,
		AttrPropertyType: propType,
		AttrWell:         well,
	}
	store.listings[l.ID] = l
	store.bySource[store.sourceKey(l.SourceID, l.SourceListingID)] = id
}

func TestQueryListingsFilter_NoFilter(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	ctx := context.Background()

	seedFilterListing(store, "f1", ptrF(10.0), ptrI(100000), ptrS("Ada"), ptrS("ranch"), ptrB(true))
	seedFilterListing(store, "f2", ptrF(5.0), ptrI(50000), ptrS("Boise"), ptrS("farm"), ptrB(false))
	seedFilterListing(store, "f3", ptrF(20.0), ptrI(200000), ptrS("Canyon"), ptrS("residential"), nil)

	results, err := core.QueryListingsFilter(ctx, listing.ListingFilter{}, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Errorf("want 3 results, got %d", len(results))
	}
}

func TestQueryListingsFilter_AcresMinMax(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	ctx := context.Background()

	seedFilterListing(store, "f1", ptrF(5.0), ptrI(50000), ptrS("Ada"), nil, nil)
	seedFilterListing(store, "f2", ptrF(10.0), ptrI(100000), ptrS("Ada"), nil, nil)
	seedFilterListing(store, "f3", ptrF(20.0), ptrI(200000), ptrS("Ada"), nil, nil)

	results, err := core.QueryListingsFilter(ctx, listing.ListingFilter{
		AcresMin: ptrF(8.0),
		AcresMax: ptrF(15.0),
	}, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("want 1 result (10 acres), got %d", len(results))
	}
	if results[0].Acres == nil || *results[0].Acres != 10.0 {
		t.Errorf("expected 10 acres listing, got %v", results[0].Acres)
	}
}

func TestQueryListingsFilter_PriceMinMax(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	ctx := context.Background()

	seedFilterListing(store, "f1", ptrF(5.0), ptrI(30000), ptrS("Ada"), nil, nil)
	seedFilterListing(store, "f2", ptrF(10.0), ptrI(75000), ptrS("Ada"), nil, nil)
	seedFilterListing(store, "f3", ptrF(20.0), ptrI(150000), ptrS("Ada"), nil, nil)

	results, err := core.QueryListingsFilter(ctx, listing.ListingFilter{
		PriceMin: ptrI(50000),
		PriceMax: ptrI(100000),
	}, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("want 1 result (75000 cents), got %d", len(results))
	}
	if results[0].PriceCents == nil || *results[0].PriceCents != 75000 {
		t.Errorf("expected 75000 price listing, got %v", results[0].PriceCents)
	}
}

func TestQueryListingsFilter_County(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	ctx := context.Background()

	seedFilterListing(store, "f1", ptrF(5.0), ptrI(50000), ptrS("Ada"), nil, nil)
	seedFilterListing(store, "f2", ptrF(10.0), ptrI(100000), ptrS("Canyon"), nil, nil)
	seedFilterListing(store, "f3", ptrF(20.0), ptrI(200000), ptrS("Owyhee"), nil, nil)

	results, err := core.QueryListingsFilter(ctx, listing.ListingFilter{
		Counties: []string{"Ada", "Owyhee"},
	}, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("want 2 results (Ada, Owyhee), got %d", len(results))
	}
}

func TestQueryListingsFilter_PropertyType(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	ctx := context.Background()

	seedFilterListing(store, "f1", ptrF(5.0), ptrI(50000), ptrS("Ada"), ptrS("ranch"), nil)
	seedFilterListing(store, "f2", ptrF(10.0), ptrI(100000), ptrS("Canyon"), ptrS("farm"), nil)
	seedFilterListing(store, "f3", ptrF(20.0), ptrI(200000), ptrS("Owyhee"), ptrS("ranch"), nil)

	results, err := core.QueryListingsFilter(ctx, listing.ListingFilter{
		PropertyType: ptrS("ranch"),
	}, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("want 2 ranch results, got %d", len(results))
	}
}

func TestQueryListingsFilter_AttrWell(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	ctx := context.Background()

	seedFilterListing(store, "f1", ptrF(5.0), ptrI(50000), ptrS("Ada"), nil, ptrB(true))
	seedFilterListing(store, "f2", ptrF(10.0), ptrI(100000), ptrS("Canyon"), nil, ptrB(false))
	seedFilterListing(store, "f3", ptrF(20.0), ptrI(200000), ptrS("Owyhee"), nil, nil) // no well info

	results, err := core.QueryListingsFilter(ctx, listing.ListingFilter{
		AttrWell: ptrB(true),
	}, 100, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("want 1 result (well=true), got %d", len(results))
	}
	if results[0].AttrWell == nil || !*results[0].AttrWell {
		t.Errorf("expected well=true listing")
	}
}
