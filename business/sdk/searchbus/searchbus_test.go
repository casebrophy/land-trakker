package searchbus_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/search"
	"github.com/cbrophy/land_trakker/business/sdk/searchbus"
)

// =============================================================================
// fakeSearchStore — in-memory search.Storer
// =============================================================================

type fakeSearchStore struct {
	searches  map[string]search.SavedSearch
	hits      []search.SearchHit
	nextID    int
	nextHitID int64
}

func newFakeSearchStore() *fakeSearchStore {
	return &fakeSearchStore{
		searches:  make(map[string]search.SavedSearch),
		nextID:    1,
		nextHitID: 1,
	}
}

func (f *fakeSearchStore) CreateSavedSearch(_ context.Context, s search.SavedSearch) (search.SavedSearch, error) {
	s.ID = fmt.Sprintf("id-%d", f.nextID)
	f.nextID++
	s.CreatedAt = time.Now().UTC()
	f.searches[s.ID] = s
	return s, nil
}

func (f *fakeSearchStore) UpdateSavedSearch(_ context.Context, s search.SavedSearch) (search.SavedSearch, error) {
	f.searches[s.ID] = s
	return s, nil
}

func (f *fakeSearchStore) DeleteSavedSearch(_ context.Context, id string) error {
	delete(f.searches, id)
	return nil
}

func (f *fakeSearchStore) QuerySavedSearchByID(_ context.Context, id string) (search.SavedSearch, error) {
	s, ok := f.searches[id]
	if !ok {
		return search.SavedSearch{}, fmt.Errorf("not found: %s", id)
	}
	return s, nil
}

func (f *fakeSearchStore) QuerySavedSearches(_ context.Context) ([]search.SavedSearch, error) {
	var out []search.SavedSearch
	for _, s := range f.searches {
		if s.Enabled {
			out = append(out, s)
		}
	}
	return out, nil
}

func (f *fakeSearchStore) CreateHitIfAbsent(_ context.Context, h search.SearchHit) error {
	// Dedup on (SavedSearchID, ListingID, Reason, HitAt)
	for _, existing := range f.hits {
		if existing.SavedSearchID == h.SavedSearchID &&
			existing.ListingID == h.ListingID &&
			existing.Reason == h.Reason &&
			existing.HitAt.Equal(h.HitAt) {
			return nil
		}
	}
	h.ID = f.nextHitID
	f.nextHitID++
	f.hits = append(f.hits, h)
	return nil
}

func (f *fakeSearchStore) QueryUnseen(_ context.Context, limit int) ([]search.SearchHit, error) {
	var out []search.SearchHit
	for _, h := range f.hits {
		if !h.Seen {
			out = append(out, h)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

func (f *fakeSearchStore) MarkHitsSeen(_ context.Context, ids []int64) error {
	idSet := make(map[int64]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	for i := range f.hits {
		if idSet[f.hits[i].ID] {
			f.hits[i].Seen = true
		}
	}
	return nil
}

// =============================================================================
// fakeListingStore — in-memory ListingStore
// =============================================================================

type fakeListingStore struct {
	listings     []listing.Listing
	priceChanges map[string][]listing.PriceChange
	snapshots    map[string][]listing.ListingSnapshot
}

func newFakeListingStore() *fakeListingStore {
	return &fakeListingStore{
		priceChanges: make(map[string][]listing.PriceChange),
		snapshots:    make(map[string][]listing.ListingSnapshot),
	}
}

func (f *fakeListingStore) QueryListingsFilter(_ context.Context, _ listing.ListingFilter, limit, offset int) ([]listing.Listing, error) {
	if offset >= len(f.listings) {
		return nil, nil
	}
	end := offset + limit
	if end > len(f.listings) {
		end = len(f.listings)
	}
	return f.listings[offset:end], nil
}

func (f *fakeListingStore) QueryPriceChangesByListing(_ context.Context, listingID string) ([]listing.PriceChange, error) {
	return f.priceChanges[listingID], nil
}

func (f *fakeListingStore) QuerySnapshotsByListing(_ context.Context, listingID string) ([]listing.ListingSnapshot, error) {
	return f.snapshots[listingID], nil
}

// =============================================================================
// helpers
// =============================================================================

func ptr[T any](v T) *T { return &v }

func newCore(ss *fakeSearchStore, ls *fakeListingStore) *searchbus.Core {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return searchbus.NewCore(ss, ls, log)
}

// =============================================================================
// Tests: CRUD
// =============================================================================

func TestCreateSavedSearch(t *testing.T) {
	ss := newFakeSearchStore()
	ls := newFakeListingStore()
	core := newCore(ss, ls)
	ctx := context.Background()

	in := search.SavedSearch{
		Name:    "My Search",
		Query:   listing.ListingFilter{AcresMin: ptr(5.0)},
		Enabled: true,
	}
	got, err := core.CreateSavedSearch(ctx, in)
	if err != nil {
		t.Fatalf("CreateSavedSearch: %v", err)
	}
	if got.ID == "" {
		t.Error("expected non-empty ID")
	}
	if got.Name != "My Search" {
		t.Errorf("Name: got %q", got.Name)
	}
	if got.Query.AcresMin == nil || *got.Query.AcresMin != 5.0 {
		t.Errorf("Query.AcresMin: got %v", got.Query.AcresMin)
	}
	if _, ok := ss.searches[got.ID]; !ok {
		t.Error("expected search to be stored")
	}
}

func TestDeleteSavedSearch(t *testing.T) {
	ss := newFakeSearchStore()
	ls := newFakeListingStore()
	core := newCore(ss, ls)
	ctx := context.Background()

	created, err := core.CreateSavedSearch(ctx, search.SavedSearch{Name: "del", Enabled: true})
	if err != nil {
		t.Fatalf("CreateSavedSearch: %v", err)
	}

	if err := core.DeleteSavedSearch(ctx, created.ID); err != nil {
		t.Fatalf("DeleteSavedSearch: %v", err)
	}
	if _, ok := ss.searches[created.ID]; ok {
		t.Error("expected search to be deleted")
	}
}

// =============================================================================
// Tests: EvaluateAll
// =============================================================================

func TestEvaluateAll_NewListing(t *testing.T) {
	now := time.Now().UTC()
	since := now.Add(-24 * time.Hour)

	ss := newFakeSearchStore()
	ls := newFakeListingStore()
	core := newCore(ss, ls)
	ctx := context.Background()

	savedSearch, err := core.CreateSavedSearch(ctx, search.SavedSearch{Name: "new-test", Enabled: true})
	if err != nil {
		t.Fatalf("CreateSavedSearch: %v", err)
	}

	// Listing first seen within the window
	ls.listings = []listing.Listing{
		{
			ID:          "listing-new",
			FirstSeenAt: since.Add(time.Hour),
		},
	}

	hits, err := core.EvaluateAll(ctx, now)
	if err != nil {
		t.Fatalf("EvaluateAll: %v", err)
	}
	if hits != 1 {
		t.Errorf("want 1 hit, got %d", hits)
	}
	if len(ss.hits) != 1 {
		t.Fatalf("want 1 stored hit, got %d", len(ss.hits))
	}
	if ss.hits[0].Reason != search.ReasonNew {
		t.Errorf("want reason %q, got %q", search.ReasonNew, ss.hits[0].Reason)
	}
	if ss.hits[0].SavedSearchID != savedSearch.ID {
		t.Errorf("SavedSearchID mismatch")
	}
}

func TestEvaluateAll_PriceDrop(t *testing.T) {
	now := time.Now().UTC()
	since := now.Add(-24 * time.Hour)

	ss := newFakeSearchStore()
	ls := newFakeListingStore()
	core := newCore(ss, ls)
	ctx := context.Background()

	_, err := core.CreateSavedSearch(ctx, search.SavedSearch{Name: "price-drop-test", Enabled: true})
	if err != nil {
		t.Fatalf("CreateSavedSearch: %v", err)
	}

	// Listing first seen well before window — no "new" hit
	ls.listings = []listing.Listing{
		{
			ID:          "listing-pd",
			FirstSeenAt: since.Add(-48 * time.Hour),
		},
	}

	// Price drop within the window: new < old
	ls.priceChanges["listing-pd"] = []listing.PriceChange{
		{
			ListingID:     "listing-pd",
			ChangedAt:     since.Add(time.Hour),
			OldPriceCents: ptr(int64(100000)),
			NewPriceCents: 90000,
		},
	}

	hits, err := core.EvaluateAll(ctx, now)
	if err != nil {
		t.Fatalf("EvaluateAll: %v", err)
	}
	if hits != 1 {
		t.Errorf("want 1 hit, got %d", hits)
	}
	if len(ss.hits) != 1 {
		t.Fatalf("want 1 stored hit, got %d", len(ss.hits))
	}
	if ss.hits[0].Reason != search.ReasonPriceDrop {
		t.Errorf("want reason %q, got %q", search.ReasonPriceDrop, ss.hits[0].Reason)
	}
}

func TestEvaluateAll_AttributeAdded(t *testing.T) {
	now := time.Now().UTC()
	since := now.Add(-24 * time.Hour)

	ss := newFakeSearchStore()
	ls := newFakeListingStore()
	core := newCore(ss, ls)
	ctx := context.Background()

	_, err := core.CreateSavedSearch(ctx, search.SavedSearch{Name: "attr-test", Enabled: true})
	if err != nil {
		t.Fatalf("CreateSavedSearch: %v", err)
	}

	// Listing first seen well before window
	ls.listings = []listing.Listing{
		{
			ID:          "listing-attr",
			FirstSeenAt: since.Add(-48 * time.Hour),
		},
	}

	// Most recent snapshot captured within window; diff has attr_ key with old=nil, new!=nil
	ls.snapshots["listing-attr"] = []listing.ListingSnapshot{
		{
			ListingID:  "listing-attr",
			CapturedAt: since.Add(time.Hour),
			Diff: map[string]any{
				"attr_well": map[string]any{"old": nil, "new": true},
			},
		},
	}

	hits, err := core.EvaluateAll(ctx, now)
	if err != nil {
		t.Fatalf("EvaluateAll: %v", err)
	}
	if hits != 1 {
		t.Errorf("want 1 hit, got %d", hits)
	}
	if len(ss.hits) != 1 {
		t.Fatalf("want 1 stored hit, got %d", len(ss.hits))
	}
	if ss.hits[0].Reason != search.ReasonAttributeAdded {
		t.Errorf("want reason %q, got %q", search.ReasonAttributeAdded, ss.hits[0].Reason)
	}
}
