package search

import (
	"context"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
)

// HitReason describes why a search hit was recorded.
type HitReason string

const (
	ReasonNew            HitReason = "new"
	ReasonPriceDrop      HitReason = "price_drop"
	ReasonAttributeAdded HitReason = "attribute_added"
)

// SavedSearch is a persisted filter plus metadata.
type SavedSearch struct {
	ID        string
	Name      string
	Query     listing.ListingFilter // serialized as JSONB
	CreatedAt time.Time
	Enabled   bool
}

// SearchHit records a single match between a saved search and a listing.
type SearchHit struct {
	ID            int64
	SavedSearchID string
	ListingID     string
	HitAt         time.Time
	Reason        HitReason
	Seen          bool
}

// Storer defines the persistence contract for the search domain.
// Implementations live in storage/searchdb.
type Storer interface {
	CreateSavedSearch(ctx context.Context, s SavedSearch) (SavedSearch, error)
	UpdateSavedSearch(ctx context.Context, s SavedSearch) (SavedSearch, error)
	DeleteSavedSearch(ctx context.Context, id string) error
	QuerySavedSearchByID(ctx context.Context, id string) (SavedSearch, error)
	QuerySavedSearches(ctx context.Context) ([]SavedSearch, error)
	CreateHitIfAbsent(ctx context.Context, h SearchHit) error // INSERT ... ON CONFLICT DO NOTHING
	QueryUnseen(ctx context.Context, limit int) ([]SearchHit, error)
	MarkHitsSeen(ctx context.Context, ids []int64) error
}
