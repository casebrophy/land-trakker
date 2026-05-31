package listing

import (
	"context"
	"time"
)

// ListingStatus is the lifecycle state of a canonical listing.
type ListingStatus string

const (
	StatusActive           ListingStatus = "active"
	StatusStale            ListingStatus = "stale"
	StatusPresumedInactive ListingStatus = "presumed_inactive"
	StatusConfirmedSold    ListingStatus = "confirmed_sold"
	StatusWithdrawn        ListingStatus = "withdrawn"
)

// AllStatuses contains every valid ListingStatus in the state machine.
var AllStatuses = []ListingStatus{
	StatusActive,
	StatusStale,
	StatusPresumedInactive,
	StatusConfirmedSold,
	StatusWithdrawn,
}

// IsValid reports whether s is a recognised listing status.
func (s ListingStatus) IsValid() bool {
	for _, v := range AllStatuses {
		if s == v {
			return true
		}
	}
	return false
}

// Point is a WGS-84 geographic coordinate.
type Point struct {
	Lat float64
	Lng float64
}

// Listing is the canonical record for a land listing, aggregated across
// one or more scrape runs from a single source.
type Listing struct {
	// Core identity
	ID              string // UUID, assigned by storage layer
	SourceID        string
	SourceListingID string
	URL             string
	FirstSeenAt     time.Time
	LastSeenAt      time.Time

	// State machine fields (see §11)
	Status            ListingStatus
	ConsecutiveMisses int
	Dismissed         bool
	DismissedReason   *string
	Saved             bool

	// Parsed fields
	Title       *string
	Description *string
	PriceCents  *int64
	Acres       *float64
	// PricePerAcreCents is computed by the DB; read-only for callers.
	PricePerAcreCents *int64

	// Location
	AddressLine *string
	City        *string
	County      *string
	State       *string
	PostalCode  *string
	Geom        *Point

	Photos []string

	// Broker
	BrokerName  *string
	BrokerPhone *string
	BrokerEmail *string

	PostedAt        *time.Time
	SourceUpdatedAt *time.Time

	// Structured attributes
	AttrWaterFrontage *bool
	AttrOffGrid       *bool
	AttrRoadAccess    *string
	AttrPower         *bool
	AttrWell          *bool
	AttrSeptic        *bool
	AttrPropertyType  *string
	AttrsExtra        map[string]any
	AttrsExtraction   map[string]any
}

// ListingSnapshot captures the parsed state of a listing at a point in time.
type ListingSnapshot struct {
	ID          int64
	ListingID   string
	RawFetchID  int64
	CapturedAt  time.Time
	PriceCents  *int64
	Acres       *float64
	Status      *string
	Title       *string
	Description *string
	// StructuredAttrs holds the full attribute map at snapshot time.
	StructuredAttrs map[string]any
	// Diff holds a JSON diff versus the previous snapshot.
	Diff map[string]any
}

// PriceChange records a price movement detected between two snapshots.
type PriceChange struct {
	ID            int64
	ListingID     string
	ChangedAt     time.Time
	OldPriceCents *int64
	NewPriceCents int64
	// DeltaCents is computed by the DB; read-only for callers.
	DeltaCents int64
	SnapshotID *int64
}

// ParseAttemptOutcome is the result of a single parse attempt.
type ParseAttemptOutcome string

const (
	OutcomeSuccess     ParseAttemptOutcome = "success"
	OutcomePartial     ParseAttemptOutcome = "partial"
	OutcomeParserError ParseAttemptOutcome = "parser_error"
	OutcomeUnparseable ParseAttemptOutcome = "unparseable"
)

// ParseAttempt records a single attempt to parse a raw fetch.
type ParseAttempt struct {
	ID            int64
	RawFetchID    int64
	ParserVersion string
	AttemptedAt   time.Time
	Outcome       ParseAttemptOutcome
	ErrorMessage  *string
	SnapshotID    *int64
}

// Storer defines the persistence contract for listing-domain objects.
// Implementations live in storage/listingdb.
type Storer interface {
	// Listing CRUD
	CreateListing(ctx context.Context, l Listing) (Listing, error)
	UpdateListing(ctx context.Context, l Listing) error
	QueryListingByID(ctx context.Context, id string) (Listing, error)
	QueryListingBySource(ctx context.Context, sourceID, sourceListingID string) (Listing, error)

	// Snapshot operations
	CreateSnapshot(ctx context.Context, snap ListingSnapshot) (ListingSnapshot, error)
	QuerySnapshotsByListing(ctx context.Context, listingID string) ([]ListingSnapshot, error)

	// Price-change operations
	CreatePriceChange(ctx context.Context, pc PriceChange) (PriceChange, error)
	QueryPriceChangesByListing(ctx context.Context, listingID string) ([]PriceChange, error)

	// ParseAttempt operations
	CreateParseAttempt(ctx context.Context, pa ParseAttempt) (ParseAttempt, error)
	QueryEligibleRawFetchIDs(ctx context.Context, sourceID string, parserVersion string) ([]int64, error)
}
