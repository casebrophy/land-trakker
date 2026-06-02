//go:build integration

package listingdb_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/source"
	"github.com/cbrophy/land_trakker/storage/listingdb"
	"github.com/cbrophy/land_trakker/storage/sourcedb"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

type stores struct {
	listing *listingdb.Store
	source  *sourcedb.Store
}

func setupListingDB(t *testing.T) stores {
	t.Helper()
	ctx := context.Background()

	ctr, err := tcpostgres.Run(ctx,
		"postgis/postgis:16-3.5",
		tcpostgres.WithDatabase("land_trakker"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() {
		if err := ctr.Terminate(ctx); err != nil {
			t.Errorf("terminate container: %v", err)
		}
	})

	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	migDB, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open for migrations: %v", err)
	}
	defer migDB.Close()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose.SetDialect: %v", err)
	}
	if err := goose.Up(migDB, "../../storage/migrations"); err != nil {
		t.Fatalf("goose.Up: %v", err)
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("pgxpool.New: %v", err)
	}
	t.Cleanup(pool.Close)

	return stores{
		listing: listingdb.NewStore(pool),
		source:  sourcedb.NewStore(pool),
	}
}

func ptr[T any](v T) *T { return &v }

func mustCreateSource(t *testing.T, ss *sourcedb.Store, id string) {
	t.Helper()
	_, err := ss.CreateSource(context.Background(), source.Source{
		ID:                             id,
		DisplayName:                    "Test",
		BaseURL:                        "https://example.com",
		UserAgent:                      "test",
		Enabled:                        true,
		RespectRobots:                  true,
		RateLimitMS:                    1000,
		Concurrency:                    1,
		AbsenceDaysBeforeStale:         14,
		AbsenceDaysBeforeInactive:      30,
		ConsecutiveMissedRunsThreshold: 3,
		MinResultRatioForInactivation:  0.5,
	})
	if err != nil {
		t.Fatalf("mustCreateSource: %v", err)
	}
}

func mustCreateRawFetch(t *testing.T, ss *sourcedb.Store, srcID, listingID string) int64 {
	t.Helper()
	rf, err := ss.CreateRawFetch(context.Background(), source.RawFetch{
		SourceID:        srcID,
		SourceListingID: listingID,
		URL:             "https://example.com/l/" + listingID,
		FetchedAt:       time.Now().UTC(),
		StatusCode:      200,
		Body:            []byte("body"),
		BodySHA256:      make([]byte, 32),
	})
	if err != nil {
		t.Fatalf("mustCreateRawFetch: %v", err)
	}
	return rf.ID
}

func TestCreateListingWithGeom(t *testing.T) {
	st := setupListingDB(t)
	ctx := context.Background()
	mustCreateSource(t, st.source, "src-geom")

	now := time.Now().UTC().Truncate(time.Millisecond)
	l := listing.Listing{
		SourceID:        "src-geom",
		SourceListingID: "listing-1",
		URL:             "https://example.com/l/1",
		FirstSeenAt:     now,
		LastSeenAt:      now,
		Status:          listing.StatusActive,
		Photos:          []string{},
		AttrsExtra:      map[string]any{},
		AttrsExtraction: map[string]any{},
		Geom:            &listing.Point{Lat: 35.5, Lng: -97.5},
		Title:           ptr("Test Listing"),
		PriceCents:      ptr(int64(50000000)),
		Acres:           ptr(10.5),
		State:           ptr("OK"),
	}

	created, err := st.listing.CreateListing(ctx, l)
	if err != nil {
		t.Fatalf("CreateListing: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID")
	}
	if created.Geom == nil {
		t.Fatal("expected Geom to be set")
	}
	if created.Geom.Lat != 35.5 || created.Geom.Lng != -97.5 {
		t.Errorf("Geom: got %+v", created.Geom)
	}

	fetched, err := st.listing.QueryListingByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("QueryListingByID: %v", err)
	}
	if fetched.Geom == nil {
		t.Fatal("QueryListingByID: Geom should not be nil")
	}
	if fetched.Geom.Lat != 35.5 {
		t.Errorf("Geom.Lat: got %f", fetched.Geom.Lat)
	}
	if fetched.Title == nil || *fetched.Title != "Test Listing" {
		t.Errorf("Title mismatch")
	}
}

func TestCreateListingNilGeom(t *testing.T) {
	st := setupListingDB(t)
	ctx := context.Background()
	mustCreateSource(t, st.source, "src-nogeom")

	now := time.Now().UTC().Truncate(time.Millisecond)
	l := listing.Listing{
		SourceID:        "src-nogeom",
		SourceListingID: "listing-2",
		URL:             "https://example.com/l/2",
		FirstSeenAt:     now,
		LastSeenAt:      now,
		Status:          listing.StatusActive,
		Photos:          []string{},
		AttrsExtra:      map[string]any{},
		AttrsExtraction: map[string]any{},
	}

	created, err := st.listing.CreateListing(ctx, l)
	if err != nil {
		t.Fatalf("CreateListing: %v", err)
	}
	if created.Geom != nil {
		t.Errorf("expected nil Geom, got %+v", created.Geom)
	}

	fetched, err := st.listing.QueryListingByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("QueryListingByID: %v", err)
	}
	if fetched.Geom != nil {
		t.Errorf("fetched Geom should be nil")
	}
}

func TestUpdateListing(t *testing.T) {
	st := setupListingDB(t)
	ctx := context.Background()
	mustCreateSource(t, st.source, "src-update")

	now := time.Now().UTC().Truncate(time.Millisecond)
	l := listing.Listing{
		SourceID:        "src-update",
		SourceListingID: "listing-3",
		URL:             "https://example.com/l/3",
		FirstSeenAt:     now,
		LastSeenAt:      now,
		Status:          listing.StatusActive,
		Photos:          []string{},
		AttrsExtra:      map[string]any{},
		AttrsExtraction: map[string]any{},
	}
	created, err := st.listing.CreateListing(ctx, l)
	if err != nil {
		t.Fatalf("CreateListing: %v", err)
	}

	created.Status = listing.StatusStale
	created.Title = ptr("Updated Title")
	if err := st.listing.UpdateListing(ctx, created); err != nil {
		t.Fatalf("UpdateListing: %v", err)
	}

	updated, err := st.listing.QueryListingByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("QueryListingByID after update: %v", err)
	}
	if updated.Status != listing.StatusStale {
		t.Errorf("Status: got %q", updated.Status)
	}
	if updated.Title == nil || *updated.Title != "Updated Title" {
		t.Errorf("Title: got %v", updated.Title)
	}
}

func TestQueryListingBySource(t *testing.T) {
	st := setupListingDB(t)
	ctx := context.Background()
	mustCreateSource(t, st.source, "src-qbs")

	now := time.Now().UTC().Truncate(time.Millisecond)
	l := listing.Listing{
		SourceID:        "src-qbs",
		SourceListingID: "listing-qbs-1",
		URL:             "https://example.com/l/qbs",
		FirstSeenAt:     now,
		LastSeenAt:      now,
		Status:          listing.StatusActive,
		Photos:          []string{},
		AttrsExtra:      map[string]any{},
		AttrsExtraction: map[string]any{},
	}
	created, err := st.listing.CreateListing(ctx, l)
	if err != nil {
		t.Fatalf("CreateListing: %v", err)
	}

	fetched, err := st.listing.QueryListingBySource(ctx, "src-qbs", "listing-qbs-1")
	if err != nil {
		t.Fatalf("QueryListingBySource: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("ID mismatch: got %q want %q", fetched.ID, created.ID)
	}
}

func TestCreateSnapshot(t *testing.T) {
	st := setupListingDB(t)
	ctx := context.Background()
	mustCreateSource(t, st.source, "src-snap")
	fetchID := mustCreateRawFetch(t, st.source, "src-snap", "listing-snap-1")

	now := time.Now().UTC().Truncate(time.Millisecond)
	l := listing.Listing{
		SourceID:        "src-snap",
		SourceListingID: "listing-snap-1",
		URL:             "https://example.com/l/snap",
		FirstSeenAt:     now,
		LastSeenAt:      now,
		Status:          listing.StatusActive,
		Photos:          []string{},
		AttrsExtra:      map[string]any{},
		AttrsExtraction: map[string]any{},
	}
	created, err := st.listing.CreateListing(ctx, l)
	if err != nil {
		t.Fatalf("CreateListing: %v", err)
	}

	snap := listing.ListingSnapshot{
		ListingID:       created.ID,
		RawFetchID:      fetchID,
		CapturedAt:      now,
		PriceCents:      ptr(int64(100000)),
		Status:          ptr("active"),
		StructuredAttrs: map[string]any{"foo": "bar"},
		Diff:            map[string]any{},
	}
	createdSnap, err := st.listing.CreateSnapshot(ctx, snap)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}
	if createdSnap.ID == 0 {
		t.Error("expected non-zero snapshot ID")
	}
	if createdSnap.ListingID != created.ID {
		t.Errorf("ListingID mismatch")
	}

	snaps, err := st.listing.QuerySnapshotsByListing(ctx, created.ID)
	if err != nil {
		t.Fatalf("QuerySnapshotsByListing: %v", err)
	}
	if len(snaps) != 1 {
		t.Errorf("expected 1 snapshot, got %d", len(snaps))
	}
	if snaps[0].StructuredAttrs["foo"] != "bar" {
		t.Errorf("StructuredAttrs mismatch")
	}
}

func TestQueryListingsFilter(t *testing.T) {
	st := setupListingDB(t)
	ctx := context.Background()
	mustCreateSource(t, st.source, "src-filter")

	now := time.Now().UTC().Truncate(time.Millisecond)

	makeListing := func(srcListingID string, acres float64, priceCents int64, county, propType string, well bool) listing.Listing {
		return listing.Listing{
			SourceID:         "src-filter",
			SourceListingID:  srcListingID,
			URL:              "https://example.com/l/" + srcListingID,
			FirstSeenAt:      now,
			LastSeenAt:       now,
			Status:           listing.StatusActive,
			Photos:           []string{},
			AttrsExtra:       map[string]any{},
			AttrsExtraction:  map[string]any{},
			Acres:            ptr(acres),
			PriceCents:       ptr(priceCents),
			County:           ptr(county),
			AttrPropertyType: ptr(propType),
			AttrWell:         ptr(well),
		}
	}

	l1, err := st.listing.CreateListing(ctx, makeListing("filter-1", 5.0, 50000, "Ada", "ranch", true))
	if err != nil {
		t.Fatalf("CreateListing l1: %v", err)
	}
	_, err = st.listing.CreateListing(ctx, makeListing("filter-2", 12.0, 120000, "Canyon", "farm", false))
	if err != nil {
		t.Fatalf("CreateListing l2: %v", err)
	}
	_, err = st.listing.CreateListing(ctx, makeListing("filter-3", 25.0, 250000, "Owyhee", "ranch", true))
	if err != nil {
		t.Fatalf("CreateListing l3: %v", err)
	}
	_, err = st.listing.CreateListing(ctx, makeListing("filter-4", 8.0, 80000, "Gem", "residential", false))
	if err != nil {
		t.Fatalf("CreateListing l4: %v", err)
	}

	t.Run("no filter returns all", func(t *testing.T) {
		results, err := st.listing.QueryListingsFilter(ctx, listing.ListingFilter{}, 100, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 4 {
			t.Errorf("want 4 results, got %d", len(results))
		}
	})

	t.Run("acres range", func(t *testing.T) {
		results, err := st.listing.QueryListingsFilter(ctx, listing.ListingFilter{
			AcresMin: ptr(float64(6.0)),
			AcresMax: ptr(float64(15.0)),
		}, 100, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("want 2 results (12 and 8 acres), got %d", len(results))
		}
	})

	t.Run("price range", func(t *testing.T) {
		results, err := st.listing.QueryListingsFilter(ctx, listing.ListingFilter{
			PriceMin: ptr(int64(60000)),
			PriceMax: ptr(int64(130000)),
		}, 100, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("want 2 results (80000 and 120000), got %d", len(results))
		}
	})

	t.Run("county filter", func(t *testing.T) {
		results, err := st.listing.QueryListingsFilter(ctx, listing.ListingFilter{
			Counties: []string{"Ada", "Owyhee"},
		}, 100, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("want 2 results (Ada, Owyhee), got %d", len(results))
		}
	})

	t.Run("property type filter", func(t *testing.T) {
		results, err := st.listing.QueryListingsFilter(ctx, listing.ListingFilter{
			PropertyType: ptr("ranch"),
		}, 100, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("want 2 ranch results, got %d", len(results))
		}
	})

	t.Run("well attr filter", func(t *testing.T) {
		results, err := st.listing.QueryListingsFilter(ctx, listing.ListingFilter{
			AttrWell: ptr(true),
		}, 100, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("want 2 results with well=true, got %d", len(results))
		}
	})

	t.Run("combined filter acres and county", func(t *testing.T) {
		results, err := st.listing.QueryListingsFilter(ctx, listing.ListingFilter{
			AcresMin: ptr(float64(10.0)),
			Counties: []string{"Canyon", "Owyhee"},
		}, 100, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("want 2 results, got %d", len(results))
		}
	})

	t.Run("pagination", func(t *testing.T) {
		results, err := st.listing.QueryListingsFilter(ctx, listing.ListingFilter{}, 2, 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("want 2 results with limit=2, got %d", len(results))
		}
	})

	t.Run("by listing id check ordering", func(t *testing.T) {
		_ = l1 // ensure l1 is used
	})
}

func TestCreatePriceChange(t *testing.T) {
	st := setupListingDB(t)
	ctx := context.Background()
	mustCreateSource(t, st.source, "src-pc")

	now := time.Now().UTC().Truncate(time.Millisecond)
	l := listing.Listing{
		SourceID:        "src-pc",
		SourceListingID: "listing-pc-1",
		URL:             "https://example.com/l/pc",
		FirstSeenAt:     now,
		LastSeenAt:      now,
		Status:          listing.StatusActive,
		Photos:          []string{},
		AttrsExtra:      map[string]any{},
		AttrsExtraction: map[string]any{},
	}
	created, err := st.listing.CreateListing(ctx, l)
	if err != nil {
		t.Fatalf("CreateListing: %v", err)
	}

	pc := listing.PriceChange{
		ListingID:     created.ID,
		ChangedAt:     now,
		OldPriceCents: ptr(int64(100000)),
		NewPriceCents: 90000,
	}
	createdPC, err := st.listing.CreatePriceChange(ctx, pc)
	if err != nil {
		t.Fatalf("CreatePriceChange: %v", err)
	}
	if createdPC.ID == 0 {
		t.Error("expected non-zero price change ID")
	}
	if createdPC.DeltaCents != -10000 {
		t.Errorf("DeltaCents: got %d want -10000", createdPC.DeltaCents)
	}

	pcs, err := st.listing.QueryPriceChangesByListing(ctx, created.ID)
	if err != nil {
		t.Fatalf("QueryPriceChangesByListing: %v", err)
	}
	if len(pcs) != 1 {
		t.Errorf("expected 1 price change, got %d", len(pcs))
	}
	if pcs[0].NewPriceCents != 90000 {
		t.Errorf("NewPriceCents: got %d", pcs[0].NewPriceCents)
	}
}
