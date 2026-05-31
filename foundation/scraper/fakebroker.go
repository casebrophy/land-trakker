package scraper

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// FakeBroker is an in-repo stub scraper implementing the Scraper interface
// with 3 deterministic test listings for Idaho land.
type FakeBroker struct{}

// Compile-time assertion that FakeBroker implements Scraper.
var _ Scraper = (*FakeBroker)(nil)

// NewFakeBroker creates a new FakeBroker instance.
func NewFakeBroker() *FakeBroker {
	return &FakeBroker{}
}

// Source returns the Source metadata for FakeBroker.
func (fb *FakeBroker) Source() Source {
	return Source{
		ID:          "fakebroker",
		DisplayName: "Fake Broker (Test Fixtures)",
	}
}

// ParserVersion returns the parser version for FakeBroker.
func (fb *FakeBroker) ParserVersion() string {
	return "fakebroker.v1"
}

// Discover returns 3 hardcoded ListingRef objects.
func (fb *FakeBroker) Discover(ctx context.Context) ([]ListingRef, error) {
	return []ListingRef{
		{
			SourceListingID: "fixture-1",
			URL:             "https://fakebroker.local/listing/fixture-1",
			Summary: map[string]any{
				"price": 95000,
				"acres": 40,
			},
		},
		{
			SourceListingID: "fixture-2",
			URL:             "https://fakebroker.local/listing/fixture-2",
			Summary: map[string]any{
				"price": 250000,
				"acres": 20,
			},
		},
		{
			SourceListingID: "fixture-3",
			URL:             "https://fakebroker.local/listing/fixture-3",
			Summary: map[string]any{
				"price": 450000,
				"acres": 100,
			},
		},
	}, nil
}

// Fetch returns deterministic RawFetch with mock HTML body for each fixture.
func (fb *FakeBroker) Fetch(ctx context.Context, ref ListingRef) (RawFetch, error) {
	var html string
	switch ref.SourceListingID {
	case "fixture-1":
		html = `<!DOCTYPE html>
<html>
<head><title>40 acres in Ada County</title></head>
<body>
  <h1>40 Acres - Ada County, Idaho</h1>
  <p class="price">$95,000</p>
  <p class="acres">40 acres</p>
  <p class="address">1234 Bogus Lane, Boise, Ada County, ID 83702</p>
  <p class="description">Beautiful land parcel in Ada County near Boise. No broker contact available.</p>
</body>
</html>`

	case "fixture-2":
		html = `<!DOCTYPE html>
<html>
<head><title>20 acres with broker in Gem County</title></head>
<body>
  <h1>20 Acres - Gem County, Idaho</h1>
  <p class="price">$250,000</p>
  <p class="acres">20 acres</p>
  <p class="address">5678 Prospect Road, Emmett, Gem County, ID 83617</p>
  <p class="description">Prime agricultural land in Gem County with road frontage.</p>
  <p class="broker-name">John Smith</p>
  <p class="broker-phone">(208) 555-0100</p>
  <p class="broker-email">john.smith@fakebroker.local</p>
</body>
</html>`

	case "fixture-3":
		html = `<!DOCTYPE html>
<html>
<head><title>100 acres in Owyhee County</title></head>
<body>
  <h1>100 Acres - Owyhee County, Idaho</h1>
  <p class="price">$450,000</p>
  <p class="acres">100 acres</p>
  <p class="address">9876 Rangeland Drive, Murphy, Owyhee County, ID 83650</p>
  <p class="description">Large rural parcel with excellent views and water access. No broker contact available.</p>
</body>
</html>`

	default:
		return RawFetch{}, fmt.Errorf("unknown fixture: %s", ref.SourceListingID)
	}

	return RawFetch{
		SourceID:        "fakebroker",
		SourceListingID: ref.SourceListingID,
		URL:             ref.URL,
		FetchedAt:       time.Now(),
		StatusCode:      http.StatusOK,
		ContentType:     "text/html; charset=utf-8",
		Body:            []byte(html),
		Headers:         make(http.Header),
	}, nil
}

// Parse extracts a ParsedListing from the mock HTML.
func (fb *FakeBroker) Parse(raw RawFetch) (ParsedListing, error) {
	var (
		title       string
		description string
		priceCents  *int64
		acres       *float64
		address     *Address
		county      *string
		state       *string
		broker      *Broker
	)

	switch raw.SourceListingID {
	case "fixture-1":
		title = "40 Acres - Ada County, Idaho"
		description = "Beautiful land parcel in Ada County near Boise. No broker contact available."
		p := int64(95000 * 100)
		priceCents = &p
		a := 40.0
		acres = &a
		address = &Address{
			Street: "1234 Bogus Lane",
			City:   "Boise",
			County: "Ada",
			State:  "ID",
			Zip:    "83702",
		}
		c := "Ada"
		county = &c
		s := "ID"
		state = &s

	case "fixture-2":
		title = "20 Acres - Gem County, Idaho"
		description = "Prime agricultural land in Gem County with road frontage."
		p := int64(250000 * 100)
		priceCents = &p
		a := 20.0
		acres = &a
		address = &Address{
			Street: "5678 Prospect Road",
			City:   "Emmett",
			County: "Gem",
			State:  "ID",
			Zip:    "83617",
		}
		c := "Gem"
		county = &c
		s := "ID"
		state = &s
		broker = &Broker{
			Name:  "John Smith",
			Phone: "(208) 555-0100",
			Email: "john.smith@fakebroker.local",
		}

	case "fixture-3":
		title = "100 Acres - Owyhee County, Idaho"
		description = "Large rural parcel with excellent views and water access. No broker contact available."
		p := int64(450000 * 100)
		priceCents = &p
		a := 100.0
		acres = &a
		address = &Address{
			Street: "9876 Rangeland Drive",
			City:   "Murphy",
			County: "Owyhee",
			State:  "ID",
			Zip:    "83650",
		}
		c := "Owyhee"
		county = &c
		s := "ID"
		state = &s

	default:
		return ParsedListing{}, fmt.Errorf("unknown fixture: %s", raw.SourceListingID)
	}

	return ParsedListing{
		SourceID:        raw.SourceID,
		SourceListingID: raw.SourceListingID,
		URL:             raw.URL,
		Title:           title,
		Description:     description,
		PriceCents:      priceCents,
		Acres:           acres,
		Address:         address,
		County:          county,
		State:           state,
		Broker:          broker,
		StructuredAttrs: make(map[string]any),
	}, nil
}
