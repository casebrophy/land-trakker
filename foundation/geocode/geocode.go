package geocode

import (
	"context"
	"errors"
)

// Precision describes the accuracy of a geocode result.
type Precision string

const (
	PrecisionRooftop        Precision = "rooftop"
	PrecisionStreet         Precision = "street"
	PrecisionLocality       Precision = "locality"
	PrecisionCountyCentroid Precision = "county_centroid"
)

// Result is the output of a geocoding operation.
type Result struct {
	Lat        float64
	Lng        float64
	Precision  Precision
	Provider   string
	Confidence float64
}

// Geocoder geocodes a structured address.
type Geocoder interface {
	Geocode(ctx context.Context, address, city, county, state string) (Result, error)
}

// ErrDailyLimitExceeded is returned when the configured daily geocoding cap is reached.
var ErrDailyLimitExceeded = errors.New("geocode: daily request limit exceeded")
