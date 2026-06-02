package searchdb

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/search"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store implements search.Storer using raw pgx queries.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore constructs a Store backed by the given connection pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateSavedSearch inserts a new saved search and returns it with the DB-assigned ID.
func (s *Store) CreateSavedSearch(ctx context.Context, ss search.SavedSearch) (search.SavedSearch, error) {
	queryJSON, err := marshalFilter(ss.Query)
	if err != nil {
		return search.SavedSearch{}, fmt.Errorf("searchdb.CreateSavedSearch: marshal query: %w", err)
	}

	const q = `
		INSERT INTO saved_searches (name, query, enabled)
		VALUES ($1, $2, $3)
		RETURNING id, name, query, created_at, enabled`

	row := s.pool.QueryRow(ctx, q, ss.Name, queryJSON, ss.Enabled)
	return scanSavedSearch(row)
}

// UpdateSavedSearch updates name, query, and enabled for an existing saved search.
func (s *Store) UpdateSavedSearch(ctx context.Context, ss search.SavedSearch) (search.SavedSearch, error) {
	queryJSON, err := marshalFilter(ss.Query)
	if err != nil {
		return search.SavedSearch{}, fmt.Errorf("searchdb.UpdateSavedSearch: marshal query: %w", err)
	}

	const q = `
		UPDATE saved_searches
		SET name = $1, query = $2, enabled = $3
		WHERE id = $4
		RETURNING id, name, query, created_at, enabled`

	row := s.pool.QueryRow(ctx, q, ss.Name, queryJSON, ss.Enabled, strToUUID(ss.ID))
	return scanSavedSearch(row)
}

// DeleteSavedSearch removes a saved search by ID.
func (s *Store) DeleteSavedSearch(ctx context.Context, id string) error {
	const q = `DELETE FROM saved_searches WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, strToUUID(id))
	if err != nil {
		return fmt.Errorf("searchdb.DeleteSavedSearch: %w", err)
	}
	return nil
}

// QuerySavedSearchByID retrieves a single saved search by UUID.
func (s *Store) QuerySavedSearchByID(ctx context.Context, id string) (search.SavedSearch, error) {
	const q = `
		SELECT id, name, query, created_at, enabled
		FROM saved_searches
		WHERE id = $1`

	row := s.pool.QueryRow(ctx, q, strToUUID(id))
	return scanSavedSearch(row)
}

// QuerySavedSearches returns all enabled saved searches.
func (s *Store) QuerySavedSearches(ctx context.Context) ([]search.SavedSearch, error) {
	const q = `
		SELECT id, name, query, created_at, enabled
		FROM saved_searches
		WHERE enabled = true
		ORDER BY created_at ASC`

	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("searchdb.QuerySavedSearches: %w", err)
	}
	defer rows.Close()

	var out []search.SavedSearch
	for rows.Next() {
		ss, err := scanSavedSearch(rows)
		if err != nil {
			return nil, fmt.Errorf("searchdb.QuerySavedSearches: scan: %w", err)
		}
		out = append(out, ss)
	}
	return out, rows.Err()
}

// CreateHitIfAbsent inserts a search hit, ignoring conflicts on the unique index.
func (s *Store) CreateHitIfAbsent(ctx context.Context, h search.SearchHit) error {
	const q = `
		INSERT INTO search_hits (saved_search_id, listing_id, hit_at, reason)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT DO NOTHING`

	_, err := s.pool.Exec(ctx, q,
		strToUUID(h.SavedSearchID),
		strToUUID(h.ListingID),
		timeToTZ(h.HitAt),
		string(h.Reason),
	)
	if err != nil {
		return fmt.Errorf("searchdb.CreateHitIfAbsent: %w", err)
	}
	return nil
}

// QueryUnseen returns up to limit unseen search hits ordered by hit_at DESC.
func (s *Store) QueryUnseen(ctx context.Context, limit int) ([]search.SearchHit, error) {
	const q = `
		SELECT id, saved_search_id, listing_id, hit_at, reason, seen
		FROM search_hits
		WHERE seen = false
		ORDER BY hit_at DESC
		LIMIT $1`

	rows, err := s.pool.Query(ctx, q, limit)
	if err != nil {
		return nil, fmt.Errorf("searchdb.QueryUnseen: %w", err)
	}
	defer rows.Close()

	var out []search.SearchHit
	for rows.Next() {
		h, err := scanSearchHit(rows)
		if err != nil {
			return nil, fmt.Errorf("searchdb.QueryUnseen: scan: %w", err)
		}
		out = append(out, h)
	}
	return out, rows.Err()
}

// MarkHitsSeen sets seen=true for the given hit IDs.
func (s *Store) MarkHitsSeen(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	const q = `UPDATE search_hits SET seen = true WHERE id = ANY($1)`
	_, err := s.pool.Exec(ctx, q, ids)
	if err != nil {
		return fmt.Errorf("searchdb.MarkHitsSeen: %w", err)
	}
	return nil
}

// -- row scanner helpers --

type scannable interface {
	Scan(dest ...any) error
}

func scanSavedSearch(row scannable) (search.SavedSearch, error) {
	var (
		id        pgtype.UUID
		name      string
		queryJSON []byte
		createdAt pgtype.Timestamptz
		enabled   bool
	)
	if err := row.Scan(&id, &name, &queryJSON, &createdAt, &enabled); err != nil {
		return search.SavedSearch{}, fmt.Errorf("scanSavedSearch: %w", err)
	}
	f, err := unmarshalFilter(queryJSON)
	if err != nil {
		return search.SavedSearch{}, fmt.Errorf("scanSavedSearch: unmarshal query: %w", err)
	}
	return search.SavedSearch{
		ID:        uuidToStr(id.Bytes),
		Name:      name,
		Query:     f,
		CreatedAt: createdAt.Time,
		Enabled:   enabled,
	}, nil
}

func scanSearchHit(row scannable) (search.SearchHit, error) {
	var (
		id            int64
		savedSearchID pgtype.UUID
		listingID     pgtype.UUID
		hitAt         pgtype.Timestamptz
		reason        string
		seen          bool
	)
	if err := row.Scan(&id, &savedSearchID, &listingID, &hitAt, &reason, &seen); err != nil {
		return search.SearchHit{}, fmt.Errorf("scanSearchHit: %w", err)
	}
	return search.SearchHit{
		ID:            id,
		SavedSearchID: uuidToStr(savedSearchID.Bytes),
		ListingID:     uuidToStr(listingID.Bytes),
		HitAt:         hitAt.Time,
		Reason:        search.HitReason(reason),
		Seen:          seen,
	}, nil
}

// -- type conversion helpers --

func timeToTZ(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func uuidToStr(b [16]byte) string {
	s := hex.EncodeToString(b[:])
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:]
}

func strToUUID(s string) pgtype.UUID {
	var u pgtype.UUID
	_ = u.Scan(s)
	return u
}

func marshalFilter(f listing.ListingFilter) ([]byte, error) {
	return json.Marshal(f)
}

func unmarshalFilter(b []byte) (listing.ListingFilter, error) {
	var f listing.ListingFilter
	if len(b) == 0 {
		return f, nil
	}
	if err := json.Unmarshal(b, &f); err != nil {
		return f, err
	}
	return f, nil
}

var _ search.Storer = (*Store)(nil)
