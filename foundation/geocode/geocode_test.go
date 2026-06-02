package geocode_test

import (
	"context"
	"errors"
	"testing"

	"github.com/cbrophy/land_trakker/foundation/geocode"
)

// countingGeocoder records the number of Geocode calls forwarded to inner.
type countingGeocoder struct {
	calls int
	inner geocode.Geocoder
}

func (c *countingGeocoder) Geocode(ctx context.Context, address, city, county, state string) (geocode.Result, error) {
	c.calls++
	return c.inner.Geocode(ctx, address, city, county, state)
}

func TestCachingGeocoder_CacheHit(t *testing.T) {
	inner := &countingGeocoder{inner: geocode.FakeGeocoder{}}
	g := geocode.NewCachingGeocoder(inner, geocode.NewMemStore(), 0)
	ctx := context.Background()

	r1, err := g.Geocode(ctx, "123 Main St", "Boise", "Ada", "ID")
	if err != nil {
		t.Fatalf("first Geocode: %v", err)
	}
	r2, err := g.Geocode(ctx, "123 Main St", "Boise", "Ada", "ID")
	if err != nil {
		t.Fatalf("second Geocode (cache hit): %v", err)
	}
	if inner.calls != 1 {
		t.Errorf("expected 1 inner call, got %d", inner.calls)
	}
	if r1 != r2 {
		t.Errorf("cached result differs: r1=%+v r2=%+v", r1, r2)
	}
}

func TestCachingGeocoder_CountyCentroidFallback(t *testing.T) {
	inner := &countingGeocoder{inner: geocode.FakeGeocoder{}}
	g := geocode.NewCachingGeocoder(inner, geocode.NewMemStore(), 0)
	ctx := context.Background()

	r, err := g.Geocode(ctx, "unknown address", "", "Ada", "ID")
	if err != nil {
		t.Fatalf("Geocode: %v", err)
	}
	if r.Precision != geocode.PrecisionCountyCentroid {
		t.Errorf("expected county_centroid precision, got %q", r.Precision)
	}
}

func TestCachingGeocoder_DailyLimit(t *testing.T) {
	inner := &countingGeocoder{inner: geocode.FakeGeocoder{}}
	g := geocode.NewCachingGeocoder(inner, geocode.NewMemStore(), 2)
	ctx := context.Background()

	if _, err := g.Geocode(ctx, "address one", "", "", ""); err != nil {
		t.Fatalf("call 1: %v", err)
	}
	if _, err := g.Geocode(ctx, "address two", "", "", ""); err != nil {
		t.Fatalf("call 2: %v", err)
	}
	_, err := g.Geocode(ctx, "address three", "", "", "")
	if !errors.Is(err, geocode.ErrDailyLimitExceeded) {
		t.Errorf("expected ErrDailyLimitExceeded, got %v", err)
	}
	if got := g.DailyUsed(); got != 2 {
		t.Errorf("expected DailyUsed=2, got %d", got)
	}
}

func TestCachingGeocoder_ResetDailyCount(t *testing.T) {
	inner := &countingGeocoder{inner: geocode.FakeGeocoder{}}
	g := geocode.NewCachingGeocoder(inner, geocode.NewMemStore(), 1)
	ctx := context.Background()

	if _, err := g.Geocode(ctx, "first", "", "", ""); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := g.Geocode(ctx, "second", "", "", ""); !errors.Is(err, geocode.ErrDailyLimitExceeded) {
		t.Fatalf("expected limit, got %v", err)
	}
	g.ResetDailyCount()
	if _, err := g.Geocode(ctx, "third", "", "", ""); err != nil {
		t.Errorf("after reset expected success, got %v", err)
	}
}

func TestCountyCentroid_Known(t *testing.T) {
	r, ok := geocode.CountyCentroid("Ada", "ID")
	if !ok {
		t.Fatal("Ada,ID should have a centroid")
	}
	if r.Precision != geocode.PrecisionCountyCentroid {
		t.Errorf("expected county_centroid, got %q", r.Precision)
	}
	if r.Lat == 0 || r.Lng == 0 {
		t.Errorf("expected non-zero lat/lng, got %+v", r)
	}
}

func TestCountyCentroid_Unknown(t *testing.T) {
	_, ok := geocode.CountyCentroid("NoSuchCounty", "ZZ")
	if ok {
		t.Error("expected false for unknown county")
	}
}

func TestFakeGeocoder_Normal(t *testing.T) {
	var g geocode.FakeGeocoder
	r, err := g.Geocode(context.Background(), "123 Main St", "Boise", "Ada", "ID")
	if err != nil {
		t.Fatalf("Geocode: %v", err)
	}
	if r.Precision != geocode.PrecisionRooftop {
		t.Errorf("expected rooftop, got %q", r.Precision)
	}
	if r.Provider != "fake" {
		t.Errorf("expected provider=fake, got %q", r.Provider)
	}
}

func TestFakeGeocoder_Unknown(t *testing.T) {
	var g geocode.FakeGeocoder
	_, err := g.Geocode(context.Background(), "unknown street", "", "", "ID")
	if err == nil {
		t.Error("expected error for 'unknown' address")
	}
}
