package searchbus

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/search"
)

// ListingStore is the subset of listing persistence needed by searchbus.
type ListingStore interface {
	QueryListingsFilter(ctx context.Context, f listing.ListingFilter, limit, offset int) ([]listing.Listing, error)
	QueryPriceChangesByListing(ctx context.Context, listingID string) ([]listing.PriceChange, error)
	QuerySnapshotsByListing(ctx context.Context, listingID string) ([]listing.ListingSnapshot, error)
}

// Core provides business logic for the search domain.
type Core struct {
	storer   search.Storer
	listings ListingStore
	log      *slog.Logger
}

// NewCore constructs a Core with the given storers and logger.
func NewCore(storer search.Storer, listings ListingStore, log *slog.Logger) *Core {
	return &Core{storer: storer, listings: listings, log: log}
}

// CreateSavedSearch creates and persists a new saved search.
func (c *Core) CreateSavedSearch(ctx context.Context, ss search.SavedSearch) (search.SavedSearch, error) {
	created, err := c.storer.CreateSavedSearch(ctx, ss)
	if err != nil {
		return search.SavedSearch{}, fmt.Errorf("searchbus.CreateSavedSearch: %w", err)
	}
	return created, nil
}

// UpdateSavedSearch updates an existing saved search.
func (c *Core) UpdateSavedSearch(ctx context.Context, ss search.SavedSearch) (search.SavedSearch, error) {
	updated, err := c.storer.UpdateSavedSearch(ctx, ss)
	if err != nil {
		return search.SavedSearch{}, fmt.Errorf("searchbus.UpdateSavedSearch: %w", err)
	}
	return updated, nil
}

// DeleteSavedSearch removes a saved search by ID.
func (c *Core) DeleteSavedSearch(ctx context.Context, id string) error {
	if err := c.storer.DeleteSavedSearch(ctx, id); err != nil {
		return fmt.Errorf("searchbus.DeleteSavedSearch: %w", err)
	}
	return nil
}

// QuerySavedSearchByID retrieves a saved search by UUID.
func (c *Core) QuerySavedSearchByID(ctx context.Context, id string) (search.SavedSearch, error) {
	ss, err := c.storer.QuerySavedSearchByID(ctx, id)
	if err != nil {
		return search.SavedSearch{}, fmt.Errorf("searchbus.QuerySavedSearchByID: %w", err)
	}
	return ss, nil
}

// QuerySavedSearches returns all enabled saved searches.
func (c *Core) QuerySavedSearches(ctx context.Context) ([]search.SavedSearch, error) {
	searches, err := c.storer.QuerySavedSearches(ctx)
	if err != nil {
		return nil, fmt.Errorf("searchbus.QuerySavedSearches: %w", err)
	}
	return searches, nil
}

// QueryUnseen returns up to limit unseen search hits.
func (c *Core) QueryUnseen(ctx context.Context, limit int) ([]search.SearchHit, error) {
	hits, err := c.storer.QueryUnseen(ctx, limit)
	if err != nil {
		return nil, fmt.Errorf("searchbus.QueryUnseen: %w", err)
	}
	return hits, nil
}

// MarkHitsSeen marks the given hit IDs as seen.
func (c *Core) MarkHitsSeen(ctx context.Context, ids []int64) error {
	if err := c.storer.MarkHitsSeen(ctx, ids); err != nil {
		return fmt.Errorf("searchbus.MarkHitsSeen: %w", err)
	}
	return nil
}

const (
	evalPageSize = 500
	evalMaxRows  = 10000
)

// EvaluateAll runs every enabled saved search against current listings and
// records hits. Returns total number of hits created.
func (c *Core) EvaluateAll(ctx context.Context, now time.Time) (int, error) {
	searches, err := c.storer.QuerySavedSearches(ctx)
	if err != nil {
		return 0, fmt.Errorf("searchbus.EvaluateAll: list searches: %w", err)
	}

	since := now.Add(-24 * time.Hour)
	hitAt := now.UTC().Truncate(24 * time.Hour)

	var totalHits int

	for _, ss := range searches {
		hits, err := c.evaluateOne(ctx, ss, since, hitAt)
		if err != nil {
			c.log.Warn("EvaluateAll: search failed", "search_id", ss.ID, "err", err)
			continue
		}
		totalHits += hits
	}

	return totalHits, nil
}

func (c *Core) evaluateOne(ctx context.Context, ss search.SavedSearch, since, hitAt time.Time) (int, error) {
	var hits int
	offset := 0

	for offset < evalMaxRows {
		limit := evalPageSize
		if offset+limit > evalMaxRows {
			limit = evalMaxRows - offset
		}

		listings, err := c.listings.QueryListingsFilter(ctx, ss.Query, limit, offset)
		if err != nil {
			return hits, fmt.Errorf("query listings: %w", err)
		}
		if len(listings) == 0 {
			break
		}

		for _, l := range listings {
			n, err := c.evaluateListing(ctx, ss, l, since, hitAt)
			if err != nil {
				c.log.Warn("EvaluateAll: listing eval failed",
					"search_id", ss.ID,
					"listing_id", l.ID,
					"err", err,
				)
				continue
			}
			hits += n
		}

		if len(listings) < limit {
			break
		}
		offset += limit
	}

	return hits, nil
}

func (c *Core) evaluateListing(ctx context.Context, ss search.SavedSearch, l listing.Listing, since, hitAt time.Time) (int, error) {
	var hits int

	// "new" hit: listing first seen within the window
	if l.FirstSeenAt.After(since) {
		h := search.SearchHit{
			SavedSearchID: ss.ID,
			ListingID:     l.ID,
			HitAt:         hitAt,
			Reason:        search.ReasonNew,
		}
		if err := c.storer.CreateHitIfAbsent(ctx, h); err != nil {
			return hits, fmt.Errorf("create new hit: %w", err)
		}
		hits++
	}

	// "price_drop" hit: a price change within the window where new < old
	pcs, err := c.listings.QueryPriceChangesByListing(ctx, l.ID)
	if err != nil {
		return hits, fmt.Errorf("query price changes: %w", err)
	}
	for _, pc := range pcs {
		if pc.ChangedAt.After(since) && pc.OldPriceCents != nil && pc.NewPriceCents < *pc.OldPriceCents {
			h := search.SearchHit{
				SavedSearchID: ss.ID,
				ListingID:     l.ID,
				HitAt:         hitAt,
				Reason:        search.ReasonPriceDrop,
			}
			if err := c.storer.CreateHitIfAbsent(ctx, h); err != nil {
				return hits, fmt.Errorf("create price_drop hit: %w", err)
			}
			hits++
			break // one hit per listing per day
		}
	}

	// "attribute_added" hit: most recent snapshot captured within window has
	// a diff entry for a key starting with "attr_" where old is nil and new is non-nil
	snaps, err := c.listings.QuerySnapshotsByListing(ctx, l.ID)
	if err != nil {
		return hits, fmt.Errorf("query snapshots: %w", err)
	}
	if len(snaps) > 0 {
		latest := snaps[0] // DESC order → newest first
		if latest.CapturedAt.After(since) {
			if hasNewAttr(latest.Diff) {
				h := search.SearchHit{
					SavedSearchID: ss.ID,
					ListingID:     l.ID,
					HitAt:         hitAt,
					Reason:        search.ReasonAttributeAdded,
				}
				if err := c.storer.CreateHitIfAbsent(ctx, h); err != nil {
					return hits, fmt.Errorf("create attribute_added hit: %w", err)
				}
				hits++
			}
		}
	}

	return hits, nil
}

// hasNewAttr returns true if the diff map contains any key starting with "attr_"
// where the entry's "old" value is nil and "new" value is non-nil.
func hasNewAttr(diff map[string]any) bool {
	for k, v := range diff {
		if !strings.HasPrefix(k, "attr_") {
			continue
		}
		entry, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if entry["old"] == nil && entry["new"] != nil {
			return true
		}
	}
	return false
}
