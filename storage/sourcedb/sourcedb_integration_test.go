//go:build integration

package sourcedb_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/source"
	"github.com/cbrophy/land_trakker/storage/sourcedb"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

func setupSourceDB(t *testing.T) *sourcedb.Store {
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

	return sourcedb.NewStore(pool)
}

func ptr[T any](v T) *T { return &v }

func TestSourceCRUD(t *testing.T) {
	store := setupSourceDB(t)
	ctx := context.Background()

	src := source.Source{
		ID:                             "test-source-1",
		DisplayName:                    "Test Source",
		BaseURL:                        "https://example.com",
		UserAgent:                      "land-trakker/test",
		Enabled:                        true,
		RespectRobots:                  true,
		RateLimitMS:                    1500,
		Concurrency:                    2,
		AbsenceDaysBeforeStale:         14,
		AbsenceDaysBeforeInactive:      30,
		ConsecutiveMissedRunsThreshold: 3,
		MinResultRatioForInactivation:  0.5,
		Notes:                          ptr("initial notes"),
	}

	created, err := store.CreateSource(ctx, src)
	if err != nil {
		t.Fatalf("CreateSource: %v", err)
	}
	if created.ID != src.ID {
		t.Errorf("ID mismatch: got %q want %q", created.ID, src.ID)
	}
	if created.RateLimitMS != src.RateLimitMS {
		t.Errorf("RateLimitMS: got %d want %d", created.RateLimitMS, src.RateLimitMS)
	}
	if created.Notes == nil || *created.Notes != *src.Notes {
		t.Errorf("Notes mismatch")
	}

	fetched, err := store.QuerySourceByID(ctx, src.ID)
	if err != nil {
		t.Fatalf("QuerySourceByID: %v", err)
	}
	if fetched.DisplayName != src.DisplayName {
		t.Errorf("DisplayName: got %q want %q", fetched.DisplayName, src.DisplayName)
	}

	fetched.DisplayName = "Updated Name"
	if err := store.UpdateSource(ctx, fetched); err != nil {
		t.Fatalf("UpdateSource: %v", err)
	}
	updated, err := store.QuerySourceByID(ctx, src.ID)
	if err != nil {
		t.Fatalf("QuerySourceByID after update: %v", err)
	}
	if updated.DisplayName != "Updated Name" {
		t.Errorf("DisplayName after update: got %q", updated.DisplayName)
	}

	all, err := store.QuerySources(ctx)
	if err != nil {
		t.Fatalf("QuerySources: %v", err)
	}
	if len(all) < 1 {
		t.Errorf("QuerySources: expected at least 1, got %d", len(all))
	}
}

func TestScrapeRunCRUD(t *testing.T) {
	store := setupSourceDB(t)
	ctx := context.Background()

	src := source.Source{
		ID:                             "run-source-1",
		DisplayName:                    "Run Source",
		BaseURL:                        "https://example.com",
		UserAgent:                      "land-trakker/test",
		Enabled:                        true,
		RespectRobots:                  true,
		RateLimitMS:                    1000,
		Concurrency:                    1,
		AbsenceDaysBeforeStale:         14,
		AbsenceDaysBeforeInactive:      30,
		ConsecutiveMissedRunsThreshold: 3,
		MinResultRatioForInactivation:  0.5,
	}
	if _, err := store.CreateSource(ctx, src); err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Millisecond)
	run := source.ScrapeRun{
		SourceID:  src.ID,
		StartedAt: now,
		Status:    source.RunStatusRunning,
	}
	createdRun, err := store.CreateRun(ctx, run)
	if err != nil {
		t.Fatalf("CreateRun: %v", err)
	}
	if createdRun.ID == 0 {
		t.Error("expected non-zero run ID")
	}
	if createdRun.Status != source.RunStatusRunning {
		t.Errorf("Status: got %q", createdRun.Status)
	}

	fetched, err := store.QueryRunByID(ctx, createdRun.ID)
	if err != nil {
		t.Fatalf("QueryRunByID: %v", err)
	}
	if fetched.SourceID != src.ID {
		t.Errorf("SourceID: got %q want %q", fetched.SourceID, src.ID)
	}

	finishedAt := now.Add(time.Minute)
	createdRun.FinishedAt = &finishedAt
	createdRun.Status = source.RunStatusOK
	createdRun.DiscoveredCount = ptr(42)
	if err := store.UpdateRun(ctx, createdRun); err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}
	updated, err := store.QueryRunByID(ctx, createdRun.ID)
	if err != nil {
		t.Fatalf("QueryRunByID after update: %v", err)
	}
	if updated.FinishedAt == nil {
		t.Error("FinishedAt should be set after update")
	}
	if updated.Status != source.RunStatusOK {
		t.Errorf("Status after update: got %q", updated.Status)
	}

	runs, err := store.QueryRunsBySource(ctx, src.ID, 10)
	if err != nil {
		t.Fatalf("QueryRunsBySource: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("QueryRunsBySource: expected 1, got %d", len(runs))
	}
}

func TestRawFetchCRUD(t *testing.T) {
	store := setupSourceDB(t)
	ctx := context.Background()

	src := source.Source{
		ID:                             "fetch-source-1",
		DisplayName:                    "Fetch Source",
		BaseURL:                        "https://example.com",
		UserAgent:                      "land-trakker/test",
		Enabled:                        true,
		RespectRobots:                  true,
		RateLimitMS:                    1000,
		Concurrency:                    1,
		AbsenceDaysBeforeStale:         14,
		AbsenceDaysBeforeInactive:      30,
		ConsecutiveMissedRunsThreshold: 3,
		MinResultRatioForInactivation:  0.5,
	}
	if _, err := store.CreateSource(ctx, src); err != nil {
		t.Fatalf("CreateSource: %v", err)
	}

	rf := source.RawFetch{
		SourceID:        src.ID,
		SourceListingID: "listing-abc",
		URL:             "https://example.com/listing/abc",
		FetchedAt:       time.Now().UTC().Truncate(time.Millisecond),
		StatusCode:      200,
		ContentType:     ptr("text/html"),
		Body:            []byte("<html>test</html>"),
		BodySHA256:      make([]byte, 32),
		HeadersJSON:     []byte(`{"Content-Type":"text/html"}`),
	}
	created, err := store.CreateRawFetch(ctx, rf)
	if err != nil {
		t.Fatalf("CreateRawFetch: %v", err)
	}
	if created.ID == 0 {
		t.Error("expected non-zero fetch ID")
	}
	if created.StatusCode != rf.StatusCode {
		t.Errorf("StatusCode: got %d want %d", created.StatusCode, rf.StatusCode)
	}

	fetched, err := store.QueryRawFetchByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("QueryRawFetchByID: %v", err)
	}
	if fetched.URL != rf.URL {
		t.Errorf("URL: got %q want %q", fetched.URL, rf.URL)
	}

	fetches, err := store.QueryRawFetchesByListing(ctx, src.ID, "listing-abc")
	if err != nil {
		t.Fatalf("QueryRawFetchesByListing: %v", err)
	}
	if len(fetches) != 1 {
		t.Errorf("QueryRawFetchesByListing: expected 1, got %d", len(fetches))
	}
}
