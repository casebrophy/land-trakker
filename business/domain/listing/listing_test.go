package listing_test

import (
	"testing"

	"github.com/cbrophy/land_trakker/business/domain/listing"
)

func TestListingStatusValues(t *testing.T) {
	want := map[listing.ListingStatus]bool{
		listing.StatusActive:           true,
		listing.StatusStale:            true,
		listing.StatusPresumedInactive: true,
		listing.StatusConfirmedSold:    true,
		listing.StatusWithdrawn:        true,
	}

	if got := len(listing.AllStatuses); got != len(want) {
		t.Fatalf("AllStatuses length = %d, want %d", got, len(want))
	}

	for _, s := range listing.AllStatuses {
		if !want[s] {
			t.Errorf("unexpected status in AllStatuses: %q", s)
		}
	}
}

func TestListingStatusIsValid(t *testing.T) {
	for _, s := range listing.AllStatuses {
		if !s.IsValid() {
			t.Errorf("status %q should be valid", s)
		}
	}

	invalid := listing.ListingStatus("bogus")
	if invalid.IsValid() {
		t.Errorf("status %q should not be valid", invalid)
	}
}

func TestListingStatusStringValues(t *testing.T) {
	cases := []struct {
		status listing.ListingStatus
		want   string
	}{
		{listing.StatusActive, "active"},
		{listing.StatusStale, "stale"},
		{listing.StatusPresumedInactive, "presumed_inactive"},
		{listing.StatusConfirmedSold, "confirmed_sold"},
		{listing.StatusWithdrawn, "withdrawn"},
	}

	for _, c := range cases {
		if string(c.status) != c.want {
			t.Errorf("status value = %q, want %q", c.status, c.want)
		}
	}
}

func TestPointFields(t *testing.T) {
	p := listing.Point{Lat: 43.615, Lng: -116.202}
	if p.Lat != 43.615 || p.Lng != -116.202 {
		t.Errorf("Point fields round-trip failed: %+v", p)
	}
}

func TestListingZeroValue(t *testing.T) {
	var l listing.Listing
	if l.Status.IsValid() {
		t.Error("zero-value ListingStatus should not be valid")
	}
	if l.Photos != nil {
		t.Error("zero-value Listing.Photos should be nil (storage layer initialises to empty slice)")
	}
}
