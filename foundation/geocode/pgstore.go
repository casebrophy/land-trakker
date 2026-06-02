package geocode

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGStore is a pgx-backed CacheStore that persists to geocode_cache.
type PGStore struct {
	pool *pgxpool.Pool
}

// NewPGStore returns a PGStore backed by pool.
func NewPGStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool}
}

// Lookup returns a cached Result for the given address key, or (Result{}, false, nil) if not found.
func (s *PGStore) Lookup(ctx context.Context, key string) (Result, bool, error) {
	const q = `
		SELECT ST_AsText(geom), precision, provider, COALESCE(confidence, 0)
		FROM geocode_cache
		WHERE address_key = $1`
	var wkt, prec, provider string
	var conf float64
	err := s.pool.QueryRow(ctx, q, key).Scan(&wkt, &prec, &provider, &conf)
	if errors.Is(err, pgx.ErrNoRows) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, fmt.Errorf("geocode pgstore lookup: %w", err)
	}
	lat, lng, err := parseWKTPoint(wkt)
	if err != nil {
		return Result{}, false, fmt.Errorf("geocode pgstore parse geom: %w", err)
	}
	return Result{Lat: lat, Lng: lng, Precision: Precision(prec), Provider: provider, Confidence: conf}, true, nil
}

// Store upserts a geocode result into geocode_cache.
func (s *PGStore) Store(ctx context.Context, key string, r Result) error {
	const q = `
		INSERT INTO geocode_cache (address_key, geom, precision, provider, confidence)
		VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326), $4, $5, $6)
		ON CONFLICT (address_key) DO UPDATE
			SET geom       = EXCLUDED.geom,
				precision  = EXCLUDED.precision,
				provider   = EXCLUDED.provider,
				confidence = EXCLUDED.confidence,
				cached_at  = now()`
	_, err := s.pool.Exec(ctx, q, key, r.Lng, r.Lat, string(r.Precision), r.Provider, r.Confidence)
	if err != nil {
		return fmt.Errorf("geocode pgstore store: %w", err)
	}
	return nil
}

// parseWKTPoint parses PostGIS "POINT(lng lat)" into lat, lng.
func parseWKTPoint(wkt string) (lat, lng float64, err error) {
	inner := strings.TrimPrefix(wkt, "POINT(")
	inner = strings.TrimSuffix(inner, ")")
	parts := strings.Fields(inner)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected WKT point: %q", wkt)
	}
	if lng, err = strconv.ParseFloat(parts[0], 64); err != nil {
		return 0, 0, fmt.Errorf("parse lng: %w", err)
	}
	if lat, err = strconv.ParseFloat(parts[1], 64); err != nil {
		return 0, 0, fmt.Errorf("parse lat: %w", err)
	}
	return lat, lng, nil
}
