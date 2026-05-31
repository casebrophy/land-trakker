package scraper

import (
	"context"
	"testing"
	"time"
)

// stubScraper is a compile-time assertion that the Scraper interface is satisfiable.
type stubScraper struct{}

var _ Scraper = (*stubScraper)(nil)

func (s *stubScraper) Source() Source       { return Source{} }
func (s *stubScraper) ParserVersion() string { return "stub.v1" }
func (s *stubScraper) Discover(ctx context.Context) ([]ListingRef, error) { return nil, nil }
func (s *stubScraper) Fetch(ctx context.Context, ref ListingRef) (RawFetch, error) {
	return RawFetch{}, nil
}
func (s *stubScraper) Parse(raw RawFetch) (ParsedListing, error) { return ParsedListing{}, nil }

func TestScraperInterfaceShape(t *testing.T) {
	var s Scraper = &stubScraper{}
	src := s.Source()
	if src.Concurrency != 0 {
		t.Fatal("unexpected default")
	}
	pv := s.ParserVersion()
	if pv == "" {
		t.Fatal("empty ParserVersion")
	}
	ctx := context.Background()
	refs, err := s.Discover(ctx)
	if err != nil || refs != nil {
		t.Fatal("unexpected discover result")
	}
	rf, err := s.Fetch(ctx, ListingRef{SourceListingID: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if rf.FetchedAt == (time.Time{}) {
		// ok, zero value is fine
	}
	pl, err := s.Parse(rf)
	if err != nil || pl.SourceID != "" {
		// ok
	}
}
