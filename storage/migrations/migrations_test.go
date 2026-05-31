//go:build integration

package migrations_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/pressly/goose/v3"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var allTables = []string{
	"sources",
	"scrape_runs",
	"raw_fetches",
	"parse_attempts",
	"listings",
	"listing_snapshots",
	"price_changes",
}

func TestMigrations(t *testing.T) {
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

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping: %v", err)
	}

	// Run all migrations up
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose.SetDialect: %v", err)
	}
	if err := goose.Up(db, "."); err != nil {
		t.Fatalf("goose.Up: %v", err)
	}

	// Verify all tables exist
	t.Run("tables_exist_after_up", func(t *testing.T) {
		for _, table := range allTables {
			if !tableExists(t, db, table) {
				t.Errorf("table %q does not exist after Up", table)
			}
		}
	})

	// Roll back all migrations
	if err := goose.DownTo(db, ".", 0); err != nil {
		t.Fatalf("goose.DownTo(0): %v", err)
	}

	// Verify all tables are gone
	t.Run("tables_gone_after_down", func(t *testing.T) {
		for _, table := range allTables {
			if tableExists(t, db, table) {
				t.Errorf("table %q still exists after DownTo(0)", table)
			}
		}
	})
}

func tableExists(t *testing.T, db *sql.DB, name string) bool {
	t.Helper()
	var exists bool
	err := db.QueryRow(
		`SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)`, name,
	).Scan(&exists)
	if err != nil {
		t.Fatalf("querying information_schema for table %q: %v", name, err)
	}
	return exists
}
