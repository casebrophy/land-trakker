package geocode

import (
	"context"
	"errors"
	"strings"
)

// FakeGeocoder is a deterministic Geocoder for tests and build-track operation.
// Addresses containing "unknown" trigger an error to exercise the county-centroid fallback path.
type FakeGeocoder struct{}

var errFakeUnknown = errors.New("fake geocoder: unknown address")

// Geocode implements Geocoder.
func (FakeGeocoder) Geocode(_ context.Context, address, _, _, _ string) (Result, error) {
	if strings.Contains(strings.ToLower(address), "unknown") {
		return Result{}, errFakeUnknown
	}
	return Result{
		Lat:        43.6150,
		Lng:        -116.2023,
		Precision:  PrecisionRooftop,
		Provider:   "fake",
		Confidence: 0.99,
	}, nil
}
