package scraper

import (
	"context"
	"testing"
)

func TestFakeBrokerDiscover(t *testing.T) {
	fb := NewFakeBroker()
	refs, err := fb.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("expected 3 listings, got %d", len(refs))
	}

	// Verify fixture IDs
	expectedIDs := map[string]bool{
		"fixture-1": false,
		"fixture-2": false,
		"fixture-3": false,
	}
	for _, ref := range refs {
		if _, ok := expectedIDs[ref.SourceListingID]; !ok {
			t.Errorf("unexpected SourceListingID: %s", ref.SourceListingID)
		}
		expectedIDs[ref.SourceListingID] = true
	}
	for id, found := range expectedIDs {
		if !found {
			t.Errorf("missing fixture: %s", id)
		}
	}
}

func TestFakeBrokerFetch(t *testing.T) {
	fb := NewFakeBroker()
	refs, err := fb.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	for _, ref := range refs {
		t.Run(ref.SourceListingID, func(t *testing.T) {
			raw, err := fb.Fetch(context.Background(), ref)
			if err != nil {
				t.Fatalf("Fetch failed: %v", err)
			}
			if raw.SourceID != "fakebroker" {
				t.Errorf("expected SourceID fakebroker, got %s", raw.SourceID)
			}
			if raw.SourceListingID != ref.SourceListingID {
				t.Errorf("expected SourceListingID %s, got %s", ref.SourceListingID, raw.SourceListingID)
			}
			if raw.StatusCode != 200 {
				t.Errorf("expected StatusCode 200, got %d", raw.StatusCode)
			}
			if len(raw.Body) == 0 {
				t.Error("expected non-empty Body")
			}
		})
	}
}

func TestFakeBrokerParse(t *testing.T) {
	fb := NewFakeBroker()
	refs, err := fb.Discover(context.Background())
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}

	testCases := []struct {
		fixtureID     string
		expectedTitle string
		expectedPrice int64
		expectedAcres float64
		expectedCity  string
		expectedCounty string
		hasBroker     bool
	}{
		{
			fixtureID:     "fixture-1",
			expectedTitle: "40 Acres - Ada County, Idaho",
			expectedPrice: 9500000,
			expectedAcres: 40,
			expectedCity:  "Boise",
			expectedCounty: "Ada",
			hasBroker:     false,
		},
		{
			fixtureID:     "fixture-2",
			expectedTitle: "20 Acres - Gem County, Idaho",
			expectedPrice: 25000000,
			expectedAcres: 20,
			expectedCity:  "Emmett",
			expectedCounty: "Gem",
			hasBroker:     true,
		},
		{
			fixtureID:     "fixture-3",
			expectedTitle: "100 Acres - Owyhee County, Idaho",
			expectedPrice: 45000000,
			expectedAcres: 100,
			expectedCity:  "Murphy",
			expectedCounty: "Owyhee",
			hasBroker:     false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.fixtureID, func(t *testing.T) {
			// Find the ref for this fixture
			var ref ListingRef
			for _, r := range refs {
				if r.SourceListingID == tc.fixtureID {
					ref = r
					break
				}
			}

			raw, err := fb.Fetch(context.Background(), ref)
			if err != nil {
				t.Fatalf("Fetch failed: %v", err)
			}

			parsed, err := fb.Parse(raw)
			if err != nil {
				t.Fatalf("Parse failed: %v", err)
			}

			if parsed.Title != tc.expectedTitle {
				t.Errorf("expected Title %s, got %s", tc.expectedTitle, parsed.Title)
			}
			if parsed.PriceCents == nil || *parsed.PriceCents != tc.expectedPrice {
				var got int64
				if parsed.PriceCents != nil {
					got = *parsed.PriceCents
				}
				t.Errorf("expected PriceCents %d, got %d", tc.expectedPrice, got)
			}
			if parsed.Acres == nil || *parsed.Acres != tc.expectedAcres {
				var got float64
				if parsed.Acres != nil {
					got = *parsed.Acres
				}
				t.Errorf("expected Acres %f, got %f", tc.expectedAcres, got)
			}
			if parsed.Address == nil {
				t.Error("expected Address to be set")
			} else {
				if parsed.Address.City != tc.expectedCity {
					t.Errorf("expected City %s, got %s", tc.expectedCity, parsed.Address.City)
				}
				if parsed.Address.County != tc.expectedCounty {
					t.Errorf("expected County %s, got %s", tc.expectedCounty, parsed.Address.County)
				}
			}

			if tc.hasBroker {
				if parsed.Broker == nil {
					t.Error("expected Broker to be set")
				} else {
					if parsed.Broker.Name == "" {
						t.Error("expected Broker.Name to be set")
					}
					if parsed.Broker.Phone == "" {
						t.Error("expected Broker.Phone to be set")
					}
					if parsed.Broker.Email == "" {
						t.Error("expected Broker.Email to be set")
					}
				}
			} else {
				if parsed.Broker != nil {
					t.Error("expected Broker to be nil")
				}
			}
		})
	}
}

func TestFakeBrokerParserVersion(t *testing.T) {
	fb := NewFakeBroker()
	pv := fb.ParserVersion()
	if pv == "" {
		t.Error("expected non-empty ParserVersion")
	}
	if pv != "fakebroker.v1" {
		t.Errorf("expected ParserVersion fakebroker.v1, got %s", pv)
	}

	// Call it twice to verify consistency
	pv2 := fb.ParserVersion()
	if pv != pv2 {
		t.Errorf("ParserVersion not consistent: %s != %s", pv, pv2)
	}
}
