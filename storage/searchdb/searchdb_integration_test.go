//go:build integration

package searchdb_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/search"
	"github.com/cbrophy/land_trakker/business/domain/source"
	"github.com/cbrophy/land_trakker/storage/listingdb"
	"github.com/cbrophy/land_trakker/storage/searchdb"
	"github.com/cbrophy/land_trakker/storage/sourcedb"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupSearchDB(t *testing.T) (*searchdb.Store, *listingdb.Store, *sourcedb.Store) {
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

	return searchdb.NewStore(pool), listingdb.NewStore(pool), sourcedb.NewStore(pool)
}

func ptr[T any](v T) *T { return &v }

func mustCreateSource(t *testing.T, ss *sourcedb.Store, id string) {
	t.Helper()
	ctx := context.Background()
	_, err := ss.CreateSource(ctx, source.Source{
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

func mustCreateListing(t *testing.T, ls *listingdb.Store, sourceID, sourceListingID string) listing.Listing {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Millisecond)
	l, err := ls.CreateListing(ctx, listing.Listing{
		SourceID:        sourceID,
		SourceListingID: sourceListingID,
		URL:             "https://example.com/l/" + sourceListingID,
		FirstSeenAt:     now,
		LastSeenAt:      now,
		Status:          listing.StatusActive,
		Photos:          []string{},
		AttrsExtra:      map[string]any{},
		AttrsExtraction: map[string]any{},
	})
	if err != nil {
		t.Fatalf("mustCreateListing: %v", err)
	}
	return l
}

func TestCreateAndQuerySavedSearch(t *testing.T) {
	store, ls, ss := setupSearchDB(t)
	_ = ls
	_ = ss
	ctx := context.Background()

	in := search.SavedSearch{
		Name:    "Integration Test Search",
		Query:   listing.ListingFilter{AcresMin: ptr(5.0), Counties: []string{"Ada"}},
		Enabled: true,
	}
	created, err := store.CreateSavedSearch(ctx, in)
	if err != nil {
		t.Fatalf("CreateSavedSearch: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID")
	}
	if created.Name != in.Name {
		t.Errorf("Name: got %q want %q", created.Name, in.Name)
	}
	if created.Query.AcresMin == nil || *created.Query.AcresMin != 5.0 {
		t.Errorf("Query.AcresMin: got %v", created.Query.AcresMin)
	}
	if len(created.Query.Counties) != 1 || created.Query.Counties[0] != "Ada" {
		t.Errorf("Query.Counties: got %v", created.Query.Counties)
	}

	fetched, err := store.QuerySavedSearchByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("QuerySavedSearchByID: %v", err)
	}
	if fetched.ID != created.ID {
		t.Errorf("ID mismatch")
	}
	if fetched.Name != created.Name {
		t.Errorf("Name mismatch")
	}
}

func TestUpdateSavedSearch(t *testing.T) {
	store, ls, ss := setupSearchDB(t)
	_ = ls
	_ = ss
	ctx := context.Background()

	created, err := store.CreateSavedSearch(ctx, search.SavedSearch{
		Name:    "Original",
		Query:   listing.ListingFilter{},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateSavedSearch: %v", err)
	}

	created.Name = "Updated"
	created.Query = listing.ListingFilter{AcresMin: ptr(10.0)}
	updated, err := store.UpdateSavedSearch(ctx, created)
	if err != nil {
		t.Fatalf("UpdateSavedSearch: %v", err)
	}
	if updated.Name != "Updated" {
		t.Errorf("Name: got %q want %q", updated.Name, "Updated")
	}
	if updated.Query.AcresMin == nil || *updated.Query.AcresMin != 10.0 {
		t.Errorf("Query.AcresMin: got %v", updated.Query.AcresMin)
	}

	fetched, err := store.QuerySavedSearchByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("QuerySavedSearchByID after update: %v", err)
	}
	if fetched.Name != "Updated" {
		t.Errorf("persisted Name: got %q", fetched.Name)
	}
}

func TestDeleteSavedSearch(t *testing.T) {
	store, ls, ss := setupSearchDB(t)
	_ = ls
	_ = ss
	ctx := context.Background()

	created, err := store.CreateSavedSearch(ctx, search.SavedSearch{
		Name:    "ToDelete",
		Query:   listing.ListingFilter{},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateSavedSearch: %v", err)
	}

	if err := store.DeleteSavedSearch(ctx, created.ID); err != nil {
		t.Fatalf("DeleteSavedSearch: %v", err)
	}

	_, err = store.QuerySavedSearchByID(ctx, created.ID)
	if err == nil {
		t.Error("expected error querying deleted search, got nil")
	}
}

func TestCreateHitIfAbsent_Idempotency(t *testing.T) {
	store, ls, ss := setupSearchDB(t)
	ctx := context.Background()

	mustCreateSource(t, ss, "src-hit")
	l := mustCreateListing(t, ls, "src-hit", "listing-hit-1")

	savedSearch, err := store.CreateSavedSearch(ctx, search.SavedSearch{
		Name:    "HitTest",
		Query:   listing.ListingFilter{},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateSavedSearch: %v", err)
	}

	hitAt := time.Now().UTC().Truncate(24 * time.Hour)
	h := search.SearchHit{
		SavedSearchID: savedSearch.ID,
		ListingID:     l.ID,
		HitAt:         hitAt,
		Reason:        search.ReasonNew,
	}

	// Insert once
	if err := store.CreateHitIfAbsent(ctx, h); err != nil {
		t.Fatalf("CreateHitIfAbsent first: %v", err)
	}
	// Insert again — should be idempotent
	if err := store.CreateHitIfAbsent(ctx, h); err != nil {
		t.Fatalf("CreateHitIfAbsent second: %v", err)
	}

	hits, err := store.QueryUnseen(ctx, 100)
	if err != nil {
		t.Fatalf("QueryUnseen: %v", err)
	}
	if len(hits) != 1 {
		t.Errorf("want 1 hit after idempotent insert, got %d", len(hits))
	}
}

func TestQueryUnseenAndMarkHitsSeen(t *testing.T) {
	store, ls, ss := setupSearchDB(t)
	ctx := context.Background()

	mustCreateSource(t, ss, "src-seen")
	l1 := mustCreateListing(t, ls, "src-seen", "listing-seen-1")
	l2 := mustCreateListing(t, ls, "src-seen", "listing-seen-2")

	savedSearch, err := store.CreateSavedSearch(ctx, search.SavedSearch{
		Name:    "SeenTest",
		Query:   listing.ListingFilter{},
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("CreateSavedSearch: %v", err)
	}

	hitAt := time.Now().UTC().Truncate(24 * time.Hour)
	for _, listingID := range []string{l1.ID, l2.ID} {
		h := search.SearchHit{
			SavedSearchID: savedSearch.ID,
			ListingID:     listingID,
			HitAt:         hitAt,
			Reason:        search.ReasonNew,
		}
		if err := store.CreateHitIfAbsent(ctx, h); err != nil {
			t.Fatalf("CreateHitIfAbsent: %v", err)
		}
	}

	unseen, err := store.QueryUnseen(ctx, 100)
	if err != nil {
		t.Fatalf("QueryUnseen: %v", err)
	}
	if len(unseen) != 2 {
		t.Errorf("want 2 unseen hits, got %d", len(unseen))
	}

	ids := []int64{unseen[0].ID}
	if err := store.MarkHitsSeen(ctx, ids); err != nil {
		t.Fatalf("MarkHitsSeen: %v", err)
	}

	unseen2, err := store.QueryUnseen(ctx, 100)
	if err != nil {
		t.Fatalf("QueryUnseen after mark: %v", err)
	}
	if len(unseen2) != 1 {
		t.Errorf("want 1 unseen hit after marking one seen, got %d", len(unseen2))
	}
}
