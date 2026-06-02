package scraper

import (
	"context"
	"net/http"
	"time"
)

type Source struct {
	ID              string
	DisplayName     string
	BaseURL         string
	RateLimit       time.Duration
	Concurrency     int
	UserAgent       string
	RespectRobots   bool

	AbsenceDaysBeforeStale         int
	AbsenceDaysBeforeInactive      int
	ConsecutiveMissedRunsThreshold int
	MinResultRatioForInactivation  float64
}

type Scraper interface {
	Source() Source
	ParserVersion() string
	Discover(ctx context.Context) ([]ListingRef, error)
	Fetch(ctx context.Context, ref ListingRef) (RawFetch, error)
	Parse(raw RawFetch) (ParsedListing, error)
}

type ListingRef struct {
	SourceListingID string
	URL             string
	Summary         map[string]any
}

type RawFetch struct {
	SourceID        string
	SourceListingID string
	URL             string
	FetchedAt       time.Time
	StatusCode      int
	ContentType     string
	Body            []byte
	Headers         http.Header
}

type Address struct {
	Street string
	City   string
	County string
	State  string
	Zip    string
}

type Broker struct {
	Name  string
	Phone string
	Email string
}

type ParsedListing struct {
	SourceID        string
	SourceListingID string
	URL             string
	Title           string
	Description     string
	PriceCents      *int64
	Acres           *float64
	Address         *Address
	County          *string
	State           *string
	Photos          []string
	Broker          *Broker
	StructuredAttrs map[string]any
	PostedAt        *time.Time
	UpdatedAt       *time.Time
	SourceStatus    string
	// Auction fields (optional, all three are pointers)
	AuctionEndDate   *time.Time
	AuctionCurrentBid *int64 // cents
	AuctionReserve   *int64 // cents
}
