package listingbus_test

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/sdk/listingbus"
)

func ptr[T any](v T) *T { return &v }

func makePoint(lat, lng float64) *listing.Point {
	return &listing.Point{Lat: lat, Lng: lng}
}

// TestScorePair_KnownDup verifies that a pair matching all 5 dimensions gets score 1.0.
func TestScorePair_KnownDup(t *testing.T) {
	cfg := listingbus.DefaultDedupConfig()
	brokerA := "Acme Realty"
	brokerB := "Acme Realty"
	titleA := "Beautiful Ranch Land for Sale"
	titleB := "Beautiful Ranch Land for Sale"
	acres := 100.0
	price := int64(500000)

	a := listing.Listing{
		ID:         "aaaaaaaa-0000-0000-0000-000000000001",
		SourceID:   "src-a",
		Geom:       makePoint(43.0, -116.0),
		Acres:      &acres,
		PriceCents: &price,
		BrokerName: &brokerA,
		Title:      &titleA,
	}
	b := listing.Listing{
		ID:         "bbbbbbbb-0000-0000-0000-000000000002",
		SourceID:   "src-b",
		Geom:       makePoint(43.001, -116.001), // ~0.14 km away
		Acres:      &acres,
		PriceCents: &price,
		BrokerName: &brokerB,
		Title:      &titleB,
	}

	score, reasons := listingbus.ScorePair(a, b, cfg)
	if score != 1.0 {
		t.Errorf("want score 1.0, got %f (reasons: %v)", score, reasons)
	}
	if len(reasons) != 5 {
		t.Errorf("want 5 reasons, got %d: %v", len(reasons), reasons)
	}
}

// TestScorePair_NonDup verifies that a pair with no matching dimensions gets score 0.0.
func TestScorePair_NonDup(t *testing.T) {
	cfg := listingbus.DefaultDedupConfig()
	brokerA := "Acme Realty"
	brokerB := "Pacific Brokers"
	titleA := "Mountain Ranch Property"
	titleB := "Coastal Wetland Parcel"
	acresA := 100.0
	acresB := 5000.0
	priceA := int64(100000)
	priceB := int64(9000000)

	a := listing.Listing{
		ID:         "aaaaaaaa-0000-0000-0000-000000000001",
		SourceID:   "src-a",
		Geom:       makePoint(43.0, -116.0),
		Acres:      &acresA,
		PriceCents: &priceA,
		BrokerName: &brokerA,
		Title:      &titleA,
	}
	b := listing.Listing{
		ID:         "bbbbbbbb-0000-0000-0000-000000000002",
		SourceID:   "src-b",
		Geom:       makePoint(47.0, -122.0), // ~500+ km away
		Acres:      &acresB,
		PriceCents: &priceB,
		BrokerName: &brokerB,
		Title:      &titleB,
	}

	score, reasons := listingbus.ScorePair(a, b, cfg)
	if score != 0.0 {
		t.Errorf("want score 0.0, got %f (reasons: %v)", score, reasons)
	}
	if len(reasons) != 0 {
		t.Errorf("want 0 reasons, got %d: %v", len(reasons), reasons)
	}
}

// TestScorePair_PartialMatch verifies that a pair matching 2 dimensions gets score 0.40.
func TestScorePair_PartialMatch(t *testing.T) {
	cfg := listingbus.DefaultDedupConfig()
	brokerA := "Acme Realty"
	brokerB := "Pacific Brokers"
	titleA := "Scenic Ranch Land"
	titleB := "Scenic Ranch Land"   // matches title
	acresA := 100.0
	acresB := 5000.0
	priceA := int64(500000)
	priceB := int64(9000000)

	a := listing.Listing{
		ID:         "aaaaaaaa-0000-0000-0000-000000000001",
		SourceID:   "src-a",
		Geom:       makePoint(47.0, -122.0),
		Acres:      &acresA,
		PriceCents: &priceA,
		BrokerName: &brokerA,
		Title:      &titleA,
	}
	b := listing.Listing{
		ID:         "bbbbbbbb-0000-0000-0000-000000000002",
		SourceID:   "src-b",
		Geom:       makePoint(47.001, -122.001), // geo matches too (~0.13 km)
		Acres:      &acresB,
		PriceCents: &priceB,
		BrokerName: &brokerB,
		Title:      &titleB,
	}

	score, reasons := listingbus.ScorePair(a, b, cfg)
	if len(reasons) != 2 {
		t.Errorf("want 2 reasons, got %d: %v", len(reasons), reasons)
	}
	want := 2.0 / 5.0
	if math.Abs(score-want) > 1e-9 {
		t.Errorf("want score %.4f, got %.4f", want, score)
	}
}

// TestScorePair_GeoOnly verifies that only geo matching gives score 0.20.
func TestScorePair_GeoOnly(t *testing.T) {
	cfg := listingbus.DefaultDedupConfig()
	brokerA := "Acme Realty"
	brokerB := "Pacific Brokers"
	titleA := "Mountain Ranch"
	titleB := "Coastal Wetland"
	acresA := 100.0
	acresB := 5000.0
	priceA := int64(100000)
	priceB := int64(9000000)

	a := listing.Listing{
		ID:         "aaaaaaaa-0000-0000-0000-000000000001",
		SourceID:   "src-a",
		Geom:       makePoint(43.0, -116.0),
		Acres:      &acresA,
		PriceCents: &priceA,
		BrokerName: &brokerA,
		Title:      &titleA,
	}
	b := listing.Listing{
		ID:         "bbbbbbbb-0000-0000-0000-000000000002",
		SourceID:   "src-b",
		Geom:       makePoint(43.001, -116.001), // ~0.14 km — geo matches
		Acres:      &acresB,
		PriceCents: &priceB,
		BrokerName: &brokerB,
		Title:      &titleB,
	}

	score, reasons := listingbus.ScorePair(a, b, cfg)
	if len(reasons) != 1 || reasons[0] != listing.DedupReasonGeo {
		t.Errorf("want [geo] reason, got %v", reasons)
	}
	want := 1.0 / 5.0
	if math.Abs(score-want) > 1e-9 {
		t.Errorf("want score %.4f, got %.4f", want, score)
	}
}

// TestRunDedup_CanonicalOrdering verifies that the stored pair always has a.ID < b.ID.
func TestRunDedup_CanonicalOrdering(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	cfg := listingbus.DefaultDedupConfig()
	now := time.Now()
	county := "Ada"

	// IDs chosen so that "listing-9" > "listing-1" lexicographically
	acres := 100.0
	price := int64(500000)
	brokerName := "Same Broker"
	title := "Same Title Ranch Land"

	// Insert two identical-looking listings from different sources with IDs
	// where "listing-9" > "listing-1" so we can verify swap.
	store.listings["listing-9"] = listing.Listing{
		ID:         "listing-9",
		SourceID:   "src-b",
		Status:     listing.StatusActive,
		County:     &county,
		Geom:       makePoint(43.0, -116.0),
		Acres:      &acres,
		PriceCents: &price,
		BrokerName: &brokerName,
		Title:      &title,
	}
	store.listings["listing-1"] = listing.Listing{
		ID:         "listing-1",
		SourceID:   "src-a",
		Status:     listing.StatusActive,
		County:     &county,
		Geom:       makePoint(43.001, -116.001),
		Acres:      &acres,
		PriceCents: &price,
		BrokerName: &brokerName,
		Title:      &title,
	}

	if err := core.RunDedup(context.Background(), cfg, now); err != nil {
		t.Fatalf("RunDedup: %v", err)
	}

	if len(store.duplicates) != 1 {
		t.Fatalf("want 1 duplicate stored, got %d", len(store.duplicates))
	}
	for key, pd := range store.duplicates {
		if pd.ListingAID >= pd.ListingBID {
			t.Errorf("canonical ordering violated: key=%s a=%s b=%s", key, pd.ListingAID, pd.ListingBID)
		}
	}
}

// TestRunDedup_SameSourceSkipped verifies that two listings from the same source are not flagged.
func TestRunDedup_SameSourceSkipped(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	cfg := listingbus.DefaultDedupConfig()
	now := time.Now()
	county := "Ada"

	acres := 100.0
	price := int64(500000)
	brokerName := "Same Broker"
	title := "Same Title Ranch"

	store.listings["listing-1"] = listing.Listing{
		ID:         "listing-1",
		SourceID:   "src-same",
		Status:     listing.StatusActive,
		County:     &county,
		Geom:       makePoint(43.0, -116.0),
		Acres:      &acres,
		PriceCents: &price,
		BrokerName: &brokerName,
		Title:      &title,
	}
	store.listings["listing-2"] = listing.Listing{
		ID:         "listing-2",
		SourceID:   "src-same",
		Status:     listing.StatusActive,
		County:     &county,
		Geom:       makePoint(43.001, -116.001),
		Acres:      &acres,
		PriceCents: &price,
		BrokerName: &brokerName,
		Title:      &title,
	}

	if err := core.RunDedup(context.Background(), cfg, now); err != nil {
		t.Fatalf("RunDedup: %v", err)
	}

	if len(store.duplicates) != 0 {
		t.Errorf("want 0 duplicates (same source), got %d", len(store.duplicates))
	}
}

// TestRunDedup_CrossSourceDetected verifies that two listings from different sources that match are flagged.
func TestRunDedup_CrossSourceDetected(t *testing.T) {
	store := newFakeStore()
	core := newCore(store)
	cfg := listingbus.DefaultDedupConfig()
	now := time.Now()
	county := "Ada"

	acres := 100.0
	price := int64(500000)
	brokerName := "Same Broker"
	title := "Same Title Ranch"

	store.listings["listing-1"] = listing.Listing{
		ID:         "listing-1",
		SourceID:   "src-a",
		Status:     listing.StatusActive,
		County:     &county,
		Geom:       makePoint(43.0, -116.0),
		Acres:      &acres,
		PriceCents: &price,
		BrokerName: &brokerName,
		Title:      &title,
	}
	store.listings["listing-2"] = listing.Listing{
		ID:         "listing-2",
		SourceID:   "src-b",
		Status:     listing.StatusActive,
		County:     &county,
		Geom:       makePoint(43.001, -116.001),
		Acres:      &acres,
		PriceCents: &price,
		BrokerName: &brokerName,
		Title:      &title,
	}

	if err := core.RunDedup(context.Background(), cfg, now); err != nil {
		t.Fatalf("RunDedup: %v", err)
	}

	if len(store.duplicates) != 1 {
		t.Errorf("want 1 duplicate (cross-source), got %d", len(store.duplicates))
	}
}
