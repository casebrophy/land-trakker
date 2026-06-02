package listingbus

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/foundation/attrs"
	"github.com/cbrophy/land_trakker/foundation/geocode"
	"github.com/cbrophy/land_trakker/foundation/scraper"
	"github.com/jackc/pgx/v5"
)

// Core provides business logic for the listing domain.
type Core struct {
	storer   listing.Storer
	geocoder geocode.Geocoder
	log      *slog.Logger
}

// NewCore constructs a Core with the given storer, geocoder, and logger.
func NewCore(storer listing.Storer, geocoder geocode.Geocoder, log *slog.Logger) *Core {
	return &Core{storer: storer, geocoder: geocoder, log: log}
}

// MissedRunConfig holds source-level thresholds for inactivation.
type MissedRunConfig struct {
	AbsenceDaysBeforeStale         int
	AbsenceDaysBeforeInactive      int
	ConsecutiveMissedRunsThreshold int
}

// UpsertFromParsed creates or updates a canonical listing from a parsed scrape result,
// inserts a snapshot with field diff, and records any price change.
// Geocoding is attempted if an address is available; daily cap exceeded results in nil geom.
func (c *Core) UpsertFromParsed(ctx context.Context, pl scraper.ParsedListing, rawFetchID int64, now time.Time) (listing.Listing, listing.ListingSnapshot, error) {
	existing, err := c.storer.QueryListingBySource(ctx, pl.SourceID, pl.SourceListingID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return listing.Listing{}, listing.ListingSnapshot{}, fmt.Errorf("querying listing by source: %w", err)
	}

	isNew := errors.Is(err, pgx.ErrNoRows)

	if isNew {
		// Build new listing
		l := listing.Listing{
			SourceID:        pl.SourceID,
			SourceListingID: pl.SourceListingID,
			URL:             pl.URL,
			Status:          listing.StatusActive,
			ConsecutiveMisses: 0,
			FirstSeenAt:     now,
			LastSeenAt:      now,
		}
		applyParsedFields(&l, pl)
		c.geocodeAndApply(ctx, &l)
		applyAttributeExtraction(&l)

		created, err := c.storer.CreateListing(ctx, l)
		if err != nil {
			return listing.Listing{}, listing.ListingSnapshot{}, fmt.Errorf("creating listing: %w", err)
		}

		statusStr := string(created.Status)
		snap := listing.ListingSnapshot{
			ListingID:       created.ID,
			RawFetchID:      rawFetchID,
			CapturedAt:      now,
			PriceCents:      created.PriceCents,
			Acres:           created.Acres,
			Status:          &statusStr,
			Title:           created.Title,
			Description:     created.Description,
			StructuredAttrs: pl.StructuredAttrs,
			Diff:            map[string]any{},
		}

		createdSnap, err := c.storer.CreateSnapshot(ctx, snap)
		if err != nil {
			return listing.Listing{}, listing.ListingSnapshot{}, fmt.Errorf("creating snapshot: %w", err)
		}

		return created, createdSnap, nil
	}

	// Existing listing — derive new status
	existing.Status = derivedStatus(existing, pl)
	existing.ConsecutiveMisses = 0
	existing.LastSeenAt = now
	applyParsedFields(&existing, pl)
	c.geocodeAndApply(ctx, &existing)
	applyAttributeExtraction(&existing)

	if err := c.storer.UpdateListing(ctx, existing); err != nil {
		return listing.Listing{}, listing.ListingSnapshot{}, fmt.Errorf("updating listing: %w", err)
	}

	// Get previous snapshot for diff
	snaps, err := c.storer.QuerySnapshotsByListing(ctx, existing.ID)
	if err != nil {
		return listing.Listing{}, listing.ListingSnapshot{}, fmt.Errorf("querying snapshots: %w", err)
	}

	var prevSnap listing.ListingSnapshot
	if len(snaps) > 0 {
		prevSnap = snaps[0]
	}

	statusStr := string(existing.Status)
	nextSnap := listing.ListingSnapshot{
		ListingID:       existing.ID,
		RawFetchID:      rawFetchID,
		CapturedAt:      now,
		PriceCents:      existing.PriceCents,
		Acres:           existing.Acres,
		Status:          &statusStr,
		Title:           existing.Title,
		Description:     existing.Description,
		StructuredAttrs: pl.StructuredAttrs,
	}

	var diff map[string]any
	if len(snaps) > 0 {
		diff = diffSnapshots(prevSnap, nextSnap)
	} else {
		diff = map[string]any{}
	}
	nextSnap.Diff = diff

	createdSnap, err := c.storer.CreateSnapshot(ctx, nextSnap)
	if err != nil {
		return listing.Listing{}, listing.ListingSnapshot{}, fmt.Errorf("creating snapshot: %w", err)
	}

	// Price change detection
	if len(snaps) > 0 && prevSnap.PriceCents != nil && existing.PriceCents != nil &&
		*prevSnap.PriceCents != *existing.PriceCents {
		snapID := createdSnap.ID
		pc := listing.PriceChange{
			ListingID:     existing.ID,
			ChangedAt:     now,
			OldPriceCents: prevSnap.PriceCents,
			NewPriceCents: *existing.PriceCents,
			SnapshotID:    &snapID,
		}
		if _, pcErr := c.storer.CreatePriceChange(ctx, pc); pcErr != nil {
			c.log.Warn("failed to record price change", "listing_id", existing.ID, "err", pcErr)
		}
	}

	return existing, createdSnap, nil
}

// QueryListings returns a paginated list of all listings ordered by first_seen_at DESC.
func (c *Core) QueryListings(ctx context.Context, limit, offset int) ([]listing.Listing, error) {
	ls, err := c.storer.QueryListings(ctx, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying listings: %w", err)
	}
	return ls, nil
}

// QueryListingsFilter returns a paginated list of listings matching the given filter.
func (c *Core) QueryListingsFilter(ctx context.Context, f listing.ListingFilter, limit, offset int) ([]listing.Listing, error) {
	ls, err := c.storer.QueryListingsFilter(ctx, f, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("querying listings with filter: %w", err)
	}
	return ls, nil
}

// QueryListingByID retrieves a canonical listing by its UUID.
func (c *Core) QueryListingByID(ctx context.Context, id string) (listing.Listing, error) {
	l, err := c.storer.QueryListingByID(ctx, id)
	if err != nil {
		return listing.Listing{}, fmt.Errorf("querying listing by id: %w", err)
	}
	return l, nil
}

// QuerySnapshotsByListing returns all snapshots for a listing ordered by captured_at DESC.
func (c *Core) QuerySnapshotsByListing(ctx context.Context, listingID string) ([]listing.ListingSnapshot, error) {
	snaps, err := c.storer.QuerySnapshotsByListing(ctx, listingID)
	if err != nil {
		return nil, fmt.Errorf("querying snapshots by listing: %w", err)
	}
	return snaps, nil
}

// QueryPriceChangesByListing returns all price changes for a listing ordered by changed_at DESC.
func (c *Core) QueryPriceChangesByListing(ctx context.Context, listingID string) ([]listing.PriceChange, error) {
	pcs, err := c.storer.QueryPriceChangesByListing(ctx, listingID)
	if err != nil {
		return nil, fmt.Errorf("querying price changes by listing: %w", err)
	}
	return pcs, nil
}

// QueryListingBySource retrieves a canonical listing by its source-specific ID.
func (c *Core) QueryListingBySource(ctx context.Context, sourceID, sourceListingID string) (listing.Listing, error) {
	l, err := c.storer.QueryListingBySource(ctx, sourceID, sourceListingID)
	if err != nil {
		return listing.Listing{}, fmt.Errorf("querying listing by source: %w", err)
	}
	return l, nil
}

// RecordParseAttempt writes a parse_attempts row for a completed parse call.
func (c *Core) RecordParseAttempt(ctx context.Context, pa listing.ParseAttempt) (listing.ParseAttempt, error) {
	created, err := c.storer.CreateParseAttempt(ctx, pa)
	if err != nil {
		return listing.ParseAttempt{}, fmt.Errorf("recording parse attempt: %w", err)
	}
	return created, nil
}

// ApplyMissedRun increments consecutive_misses and applies health-gated status transitions
// for a listing absent from a discovery run.
func (c *Core) ApplyMissedRun(ctx context.Context, listingID string, cfg MissedRunConfig, runHealthy bool, now time.Time) (listing.Listing, error) {
	l, err := c.storer.QueryListingByID(ctx, listingID)
	if err != nil {
		return listing.Listing{}, fmt.Errorf("querying listing by id: %w", err)
	}

	// Terminal states: no changes
	if l.Status == listing.StatusConfirmedSold || l.Status == listing.StatusWithdrawn {
		return l, nil
	}

	l.ConsecutiveMisses++

	if runHealthy && l.ConsecutiveMisses >= cfg.ConsecutiveMissedRunsThreshold {
		absentDays := int(now.Sub(l.LastSeenAt).Hours() / 24)

		if l.Status == listing.StatusActive || l.Status == listing.StatusStale {
			if absentDays >= cfg.AbsenceDaysBeforeInactive {
				l.Status = listing.StatusPresumedInactive
			} else if absentDays >= cfg.AbsenceDaysBeforeStale {
				l.Status = listing.StatusStale
			}
		}
		// StatusPresumedInactive: no further demotion
	}

	if err := c.storer.UpdateListing(ctx, l); err != nil {
		return listing.Listing{}, fmt.Errorf("updating listing: %w", err)
	}

	return l, nil
}

// geocodeAndApply attempts to geocode a listing and applies the result to its Geom field.
// If the daily limit is exceeded, Geom remains nil.
// If another error occurs, it is logged as a warning and upsert continues.
func (c *Core) geocodeAndApply(ctx context.Context, l *listing.Listing) {
	// Skip if no address components are available
	if l.AddressLine == nil && l.City == nil && l.County == nil && l.State == nil {
		return
	}

	// Extract address components, using empty string for nil pointers
	address := ""
	if l.AddressLine != nil {
		address = *l.AddressLine
	}
	city := ""
	if l.City != nil {
		city = *l.City
	}
	county := ""
	if l.County != nil {
		county = *l.County
	}
	state := ""
	if l.State != nil {
		state = *l.State
	}

	result, err := c.geocoder.Geocode(ctx, address, city, county, state)
	if err != nil {
		if errors.Is(err, geocode.ErrDailyLimitExceeded) {
			// Daily limit reached: leave Geom as nil, no error
			return
		}
		// Other error: log warning and continue without blocking upsert
		c.log.Warn("geocoding failed", "address_line", address, "city", city, "county", county, "state", state, "err", err)
		return
	}

	// Success: apply the geocoding result
	l.Geom = &listing.Point{Lat: result.Lat, Lng: result.Lng}
}

// derivedStatus computes the new status for an existing listing given a parsed result.
func derivedStatus(l listing.Listing, pl scraper.ParsedListing) listing.ListingStatus {
	isActiveOrStale := l.Status == listing.StatusActive || l.Status == listing.StatusStale

	switch pl.SourceStatus {
	case "sold", "under-contract":
		if isActiveOrStale {
			return listing.StatusConfirmedSold
		}
	case "withdrawn", "cancelled":
		if isActiveOrStale {
			return listing.StatusWithdrawn
		}
	}

	if l.Status == listing.StatusPresumedInactive || l.Status == listing.StatusStale {
		return listing.StatusActive
	}

	return l.Status
}

// applyParsedFields copies parsed listing fields onto a Listing.
func applyParsedFields(l *listing.Listing, pl scraper.ParsedListing) {
	l.URL = pl.URL
	l.Title = strPtrIfNonEmpty(pl.Title)
	l.Description = strPtrIfNonEmpty(pl.Description)
	l.PriceCents = pl.PriceCents
	l.Acres = pl.Acres

	if pl.Address != nil {
		l.AddressLine = strPtrIfNonEmpty(pl.Address.Street)
		l.City = strPtrIfNonEmpty(pl.Address.City)
		l.County = strPtrIfNonEmpty(pl.Address.County)
		l.State = strPtrIfNonEmpty(pl.Address.State)
		l.PostalCode = strPtrIfNonEmpty(pl.Address.Zip)
	}
	// Top-level overrides
	if pl.County != nil {
		l.County = pl.County
	}
	if pl.State != nil {
		l.State = pl.State
	}

	if pl.Photos != nil {
		l.Photos = pl.Photos
	}

	if pl.Broker != nil {
		l.BrokerName = strPtrIfNonEmpty(pl.Broker.Name)
		l.BrokerPhone = strPtrIfNonEmpty(pl.Broker.Phone)
		l.BrokerEmail = strPtrIfNonEmpty(pl.Broker.Email)
	}

	l.PostedAt = pl.PostedAt
	l.SourceUpdatedAt = pl.UpdatedAt
	l.AttrsExtra = pl.StructuredAttrs
}

// applyAttributeExtraction runs deterministic attribute extractors on a listing's
// title and description, populating attr_* fields and storing detailed extraction
// results in attrs_extraction.
func applyAttributeExtraction(l *listing.Listing) {
	// Initialize attrs_extraction
	extraction := make(map[string]any)

	// Combine title and description for extraction
	var text strings.Builder
	if l.Title != nil && *l.Title != "" {
		text.WriteString(*l.Title)
		text.WriteString(" ")
	}
	if l.Description != nil && *l.Description != "" {
		text.WriteString(*l.Description)
	}

	combinedText := text.String()
	if combinedText == "" {
		l.AttrsExtraction = extraction
		return
	}

	// Run all extractors
	results := attrs.ExtractAll(combinedText)

	// Water frontage (boolean)
	if r, ok := results["water_frontage"]; ok {
		extraction["water_frontage"] = r
		val := r.Value == "true"
		l.AttrWaterFrontage = &val
	}

	// Off-grid (boolean)
	if r, ok := results["off_grid"]; ok {
		extraction["off_grid"] = r
		val := r.Value == "true"
		l.AttrOffGrid = &val
	}

	// Road access (text)
	if r, ok := results["road_access"]; ok {
		extraction["road_access"] = r
		l.AttrRoadAccess = &r.Value
	}

	// Power (boolean)
	if r, ok := results["power"]; ok {
		extraction["power"] = r
		val := r.Value == "true"
		l.AttrPower = &val
	}

	// Well (boolean)
	if r, ok := results["well"]; ok {
		extraction["well"] = r
		val := r.Value == "true"
		l.AttrWell = &val
	}

	// Septic (boolean)
	if r, ok := results["septic"]; ok {
		extraction["septic"] = r
		val := r.Value == "true"
		l.AttrSeptic = &val
	}

	// Property type (text)
	if r, ok := results["property_type"]; ok {
		extraction["property_type"] = r
		l.AttrPropertyType = &r.Value
	}

	// Store the detailed extraction results as JSON
	l.AttrsExtraction = extraction
}

// strPtrIfNonEmpty returns nil if s is empty, otherwise a pointer to s.
func strPtrIfNonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func int64PtrEq(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func float64PtrEq(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func strPtrEq(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// diffSnapshots returns a map of changed fields between prev and next snapshots.
// Only price_cents, acres, title, description, status are compared.
// Format: {"field": {"old": <val or nil>, "new": <val or nil>}}
func diffSnapshots(prev, next listing.ListingSnapshot) map[string]any {
	diff := map[string]any{}

	if !int64PtrEq(prev.PriceCents, next.PriceCents) {
		diff["price_cents"] = map[string]any{"old": ptrVal(prev.PriceCents), "new": ptrVal(next.PriceCents)}
	}
	if !float64PtrEq(prev.Acres, next.Acres) {
		diff["acres"] = map[string]any{"old": ptrVal(prev.Acres), "new": ptrVal(next.Acres)}
	}
	if !strPtrEq(prev.Title, next.Title) {
		diff["title"] = map[string]any{"old": ptrVal(prev.Title), "new": ptrVal(next.Title)}
	}
	if !strPtrEq(prev.Description, next.Description) {
		diff["description"] = map[string]any{"old": ptrVal(prev.Description), "new": ptrVal(next.Description)}
	}
	if !strPtrEq(prev.Status, next.Status) {
		diff["status"] = map[string]any{"old": ptrVal(prev.Status), "new": ptrVal(next.Status)}
	}

	return diff
}

// ptrVal dereferences a pointer to an interface{} value, returning nil for nil pointers.
func ptrVal(v any) any {
	switch t := v.(type) {
	case *int64:
		if t == nil {
			return nil
		}
		return *t
	case *float64:
		if t == nil {
			return nil
		}
		return *t
	case *string:
		if t == nil {
			return nil
		}
		return *t
	}
	return nil
}
