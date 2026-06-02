//go:build integration

package listingdb_test

import (
	"context"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
)

func TestQueryListingsFilter_FullText_matchesTitle(t *testing.T) {
	ss := setupListingDB(t)
	ctx := context.Background()
	mustCreateSource(t, ss.source, "src-title")

	now := time.Now()
	_, err := ss.listing.CreateListing(ctx, listing.Listing{
		SourceID:        "src-title",
		SourceListingID: "1",
		Title:           ptr("Mountain Ranch 50 acres"),
		Description:     ptr("Beautiful property"),
		Status:          listing.StatusActive,
		FirstSeenAt:     now,
		LastSeenAt:      now,
	})
	if err != nil {
		t.Fatalf("CreateListing: %v", err)
	}

	_, err = ss.listing.CreateListing(ctx, listing.Listing{
		SourceID:        "src-title",
		SourceListingID: "2",
		Title:           ptr("Lakefront Property"),
		Description:     ptr("Nice waterfront"),
		Status:          listing.StatusActive,
		FirstSeenAt:     now,
		LastSeenAt:      now,
	})
	if err != nil {
		t.Fatalf("CreateListing: %v", err)
	}

	f := listing.ListingFilter{
		FullText: ptr("mountain"),
	}
	results, err := ss.listing.QueryListingsFilter(ctx, f, 100, 0)
	if err != nil {
		t.Fatalf("QueryListingsFilter: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].SourceListingID != "1" {
		t.Fatalf("expected SourceListingID=1, got %s", results[0].SourceListingID)
	}
}

func TestQueryListingsFilter_FullText_matchesDescription(t *testing.T) {
	ss := setupListingDB(t)
	ctx := context.Background()
	mustCreateSource(t, ss.source, "src-desc")

	now := time.Now()
	_, err := ss.listing.CreateListing(ctx, listing.Listing{
		SourceID:        "src-desc",
		SourceListingID: "3",
		Title:           ptr("Rural Land"),
		Description:     ptr("Great mountain views"),
		Status:          listing.StatusActive,
		FirstSeenAt:     now,
		LastSeenAt:      now,
	})
	if err != nil {
		t.Fatalf("CreateListing: %v", err)
	}

	_, err = ss.listing.CreateListing(ctx, listing.Listing{
		SourceID:        "src-desc",
		SourceListingID: "4",
		Title:           ptr("Urban Plot"),
		Description:     ptr("City center location"),
		Status:          listing.StatusActive,
		FirstSeenAt:     now,
		LastSeenAt:      now,
	})
	if err != nil {
		t.Fatalf("CreateListing: %v", err)
	}

	f := listing.ListingFilter{
		FullText: ptr("mountain"),
	}
	results, err := ss.listing.QueryListingsFilter(ctx, f, 100, 0)
	if err != nil {
		t.Fatalf("QueryListingsFilter: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].SourceListingID != "3" {
		t.Fatalf("expected SourceListingID=3, got %s", results[0].SourceListingID)
	}
}

func TestQueryListingsFilter_FullText_noMatch(t *testing.T) {
	ss := setupListingDB(t)
	ctx := context.Background()
	mustCreateSource(t, ss.source, "src-nomatch")

	now := time.Now()
	_, err := ss.listing.CreateListing(ctx, listing.Listing{
		SourceID:        "src-nomatch",
		SourceListingID: "5",
		Title:           ptr("Field Land"),
		Description:     ptr("Open property"),
		Status:          listing.StatusActive,
		FirstSeenAt:     now,
		LastSeenAt:      now,
	})
	if err != nil {
		t.Fatalf("CreateListing: %v", err)
	}

	f := listing.ListingFilter{
		FullText: ptr("mountain"),
	}
	results, err := ss.listing.QueryListingsFilter(ctx, f, 100, 0)
	if err != nil {
		t.Fatalf("QueryListingsFilter: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestQueryListingsFilter_FullText_nilFields(t *testing.T) {
	ss := setupListingDB(t)
	ctx := context.Background()
	mustCreateSource(t, ss.source, "src-nil")

	now := time.Now()
	_, err := ss.listing.CreateListing(ctx, listing.Listing{
		SourceID:        "src-nil",
		SourceListingID: "6",
		Status:          listing.StatusActive,
		FirstSeenAt:     now,
		LastSeenAt:      now,
	})
	if err != nil {
		t.Fatalf("CreateListing: %v", err)
	}

	f := listing.ListingFilter{
		FullText: ptr("anything"),
	}
	results, err := ss.listing.QueryListingsFilter(ctx, f, 100, 0)
	if err != nil {
		t.Fatalf("QueryListingsFilter: %v", err)
	}

	if len(results) != 0 {
		t.Fatalf("expected 0 results, got %d", len(results))
	}
}

func TestQueryListingsFilter_FullText_combined(t *testing.T) {
	ss := setupListingDB(t)
	ctx := context.Background()
	mustCreateSource(t, ss.source, "test-src2")

	now := time.Now()
	acresMin := 10.0
	acresMax := 50.0
	query := "mountain"

	listings := []listing.Listing{
		{
			SourceID:        "test-src2",
			SourceListingID: "m1",
			Title:           ptr("Mountain Property 30 acres"),
			Description:     ptr("Beautiful"),
			Acres:           ptr(30.0),
			Status:          listing.StatusActive,
			FirstSeenAt:     now,
			LastSeenAt:      now,
		},
		{
			SourceID:        "test-src2",
			SourceListingID: "m2",
			Title:           ptr("Mountain Land 60 acres"),
			Description:     ptr("Large property"),
			Acres:           ptr(60.0),
			Status:          listing.StatusActive,
			FirstSeenAt:     now,
			LastSeenAt:      now,
		},
		{
			SourceID:        "test-src2",
			SourceListingID: "m3",
			Title:           ptr("Field 25 acres"),
			Description:     ptr("Open land"),
			Acres:           ptr(25.0),
			Status:          listing.StatusActive,
			FirstSeenAt:     now,
			LastSeenAt:      now,
		},
	}

	for _, l := range listings {
		_, err := ss.listing.CreateListing(ctx, l)
		if err != nil {
			t.Fatalf("CreateListing: %v", err)
		}
	}

	f := listing.ListingFilter{
		FullText: &query,
		AcresMin: &acresMin,
		AcresMax: &acresMax,
	}
	results, err := ss.listing.QueryListingsFilter(ctx, f, 100, 0)
	if err != nil {
		t.Fatalf("QueryListingsFilter: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result matching both fulltext and acres filters, got %d", len(results))
	}
	if results[0].SourceListingID != "m1" {
		t.Fatalf("expected result to be m1, got %s", results[0].SourceListingID)
	}
}
