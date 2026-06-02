package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/foundation/web"
)

type fakeQuerier struct {
	listings        []listing.Listing
	snapshots       map[string][]listing.ListingSnapshot
	filteredResults []listing.Listing
	lastFilter      *listing.ListingFilter
}

func (f *fakeQuerier) QueryListings(_ context.Context, limit, offset int) ([]listing.Listing, error) {
	end := offset + limit
	if end > len(f.listings) {
		end = len(f.listings)
	}
	if offset >= len(f.listings) {
		return nil, nil
	}
	return f.listings[offset:end], nil
}

func (f *fakeQuerier) QueryListingByID(_ context.Context, id string) (listing.Listing, error) {
	for _, l := range f.listings {
		if l.ID == id {
			return l, nil
		}
	}
	return listing.Listing{}, fmt.Errorf("no rows in result set")
}

func (f *fakeQuerier) QuerySnapshotsByListing(_ context.Context, listingID string) ([]listing.ListingSnapshot, error) {
	return f.snapshots[listingID], nil
}

func (f *fakeQuerier) QueryListingsFilter(_ context.Context, filter listing.ListingFilter, limit, offset int) ([]listing.Listing, error) {
	f.lastFilter = &filter
	src := f.filteredResults
	if src == nil {
		src = f.listings
	}
	end := offset + limit
	if end > len(src) {
		end = len(src)
	}
	if offset >= len(src) {
		return nil, nil
	}
	return src[offset:end], nil
}

func TestListingsHandler_empty(t *testing.T) {
	q := &fakeQuerier{}
	h := web.ListingsHandler(q)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestListingsHandler_withListings(t *testing.T) {
	title := "Mountain Ranch 40 acres"
	q := &fakeQuerier{
		listings: []listing.Listing{
			{
				ID:          "00000000-0000-0000-0000-000000000001",
				SourceID:    "test",
				Status:      listing.StatusActive,
				Title:       &title,
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
		},
	}
	h := web.ListingsHandler(q)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, title) {
		t.Fatalf("expected body to contain %q, got:\n%s", title, body)
	}
}

func TestListingDetailHandler_notFound(t *testing.T) {
	q := &fakeQuerier{}

	r := httptest.NewRequest(http.MethodGet, "/listings/no-such-id", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "no-such-id")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	web.ListingDetailHandler(q).ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestListingDetailHandler_withSnapshots(t *testing.T) {
	listingID := "00000000-0000-0000-0000-000000000002"
	title := "Lakefront 10 acres"
	price := int64(5000000)
	snapPrice := int64(4900000)
	status := "active"

	q := &fakeQuerier{
		listings: []listing.Listing{
			{
				ID:          listingID,
				SourceID:    "test",
				Status:      listing.StatusActive,
				Title:       &title,
				PriceCents:  &price,
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
		},
		snapshots: map[string][]listing.ListingSnapshot{
			listingID: {
				{
					ID:         1,
					ListingID:  listingID,
					CapturedAt: time.Now(),
					PriceCents: &snapPrice,
					Status:     &status,
				},
			},
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/listings/"+listingID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", listingID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	web.ListingDetailHandler(q).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, title) {
		t.Fatalf("expected body to contain listing title %q", title)
	}
	if !strings.Contains(body, "$49,000") {
		t.Fatalf("expected body to contain snapshot price $49,000, got:\n%s", body)
	}
}

func TestListingDetailHandler_rendersTimeline(t *testing.T) {
	listingID := "00000000-0000-0000-0000-000000000006"
	title := "Mountain Property"
	price := int64(3000000)
	acres := 25.5

	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	snapPrice1 := int64(2800000)
	snapAcres1 := 25.5

	snapPrice2 := int64(2900000)
	snapAcres2 := 25.5

	status := "active"

	q := &fakeQuerier{
		listings: []listing.Listing{
			{
				ID:          listingID,
				SourceID:    "test",
				Status:      listing.StatusActive,
				Title:       &title,
				PriceCents:  &price,
				Acres:       &acres,
				FirstSeenAt: now,
				LastSeenAt:  now,
			},
		},
		snapshots: map[string][]listing.ListingSnapshot{
			listingID: {
				{
					ID:         1,
					ListingID:  listingID,
					CapturedAt: now.AddDate(0, 0, -7),
					PriceCents: &snapPrice1,
					Acres:      &snapAcres1,
					Status:     &status,
				},
				{
					ID:         2,
					ListingID:  listingID,
					CapturedAt: now,
					PriceCents: &snapPrice2,
					Acres:      &snapAcres2,
					Status:     &status,
				},
			},
		},
	}

	r := httptest.NewRequest(http.MethodGet, "/listings/"+listingID, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", listingID)
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	web.ListingDetailHandler(q).ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()

	// Verify timeline is present
	if !strings.Contains(body, "priceTimeline") {
		t.Fatalf("expected body to contain timeline canvas element")
	}
	if !strings.Contains(body, "Price Timeline") {
		t.Fatalf("expected body to contain 'Price Timeline' heading")
	}

	// Verify timeline data is embedded
	if !strings.Contains(body, "28000") {
		t.Fatalf("expected body to contain first snapshot price data (28000), got:\n%s", body)
	}
	if !strings.Contains(body, "29000") {
		t.Fatalf("expected body to contain second snapshot price data (29000), got:\n%s", body)
	}

	// Verify Chart.js is loaded
	if !strings.Contains(body, "chart.umd.js") {
		t.Fatalf("expected body to include Chart.js CDN")
	}
}

func TestListingsHandler_withFilter_callsQueryListingsFilter(t *testing.T) {
	title := "River Ranch 20 acres"
	q := &fakeQuerier{
		listings: []listing.Listing{
			{
				ID:          "00000000-0000-0000-0000-000000000003",
				SourceID:    "test",
				Status:      listing.StatusActive,
				Title:       &title,
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
		},
	}
	h := web.ListingsHandler(q)

	r := httptest.NewRequest(http.MethodGet, "/?acres_min=10&counties=Ada", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if q.lastFilter == nil {
		t.Fatal("expected QueryListingsFilter to be called with non-nil filter")
	}
	if q.lastFilter.AcresMin == nil || *q.lastFilter.AcresMin != 10.0 {
		t.Fatalf("expected AcresMin=10, got %v", q.lastFilter.AcresMin)
	}
	if len(q.lastFilter.Counties) != 1 || q.lastFilter.Counties[0] != "Ada" {
		t.Fatalf("expected Counties=[Ada], got %v", q.lastFilter.Counties)
	}
}

func TestListingsHandler_htmx_returnsPartial(t *testing.T) {
	title := "Hilltop 5 acres"
	q := &fakeQuerier{
		listings: []listing.Listing{
			{
				ID:          "00000000-0000-0000-0000-000000000004",
				SourceID:    "test",
				Status:      listing.StatusActive,
				Title:       &title,
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
		},
	}
	h := web.ListingsHandler(q)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	// Partial must contain the results table but NOT the full-page chrome.
	if strings.Contains(body, "<!DOCTYPE html>") {
		t.Fatal("HTMX response must not include full-page DOCTYPE")
	}
	if !strings.Contains(body, title) {
		t.Fatalf("expected partial body to contain %q", title)
	}
}

func TestListingsHandler_mapMarkers_renderedForGeocodedListings(t *testing.T) {
	title := "Lakeside 15 acres"
	lat, lng := 44.5, -116.0
	q := &fakeQuerier{
		listings: []listing.Listing{
			{
				ID:          "00000000-0000-0000-0000-000000000005",
				SourceID:    "test",
				Status:      listing.StatusActive,
				Title:       &title,
				Geom:        &listing.Point{Lat: lat, Lng: lng},
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
		},
	}
	h := web.ListingsHandler(q)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "44.5") {
		t.Fatalf("expected map marker lat 44.5 in body, got:\n%s", body)
	}
	if !strings.Contains(body, "-116") {
		t.Fatalf("expected map marker lng -116 in body, got:\n%s", body)
	}
}

func TestListingsHandler_pagination_nextURL(t *testing.T) {
	// Exactly limit listings → HasMore=true → NextURL present.
	ls := make([]listing.Listing, 50)
	for i := range ls {
		ls[i] = listing.Listing{ID: fmt.Sprintf("id-%d", i), SourceID: "test", Status: listing.StatusActive, FirstSeenAt: time.Now(), LastSeenAt: time.Now()}
	}
	q := &fakeQuerier{listings: ls}
	h := web.ListingsHandler(q)

	r := httptest.NewRequest(http.MethodGet, "/?limit=50&offset=0", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "offset=50") {
		t.Fatalf("expected next-page link with offset=50 in body, got:\n%s", body)
	}
}

func TestListingsHandler_withFilter_fulltext(t *testing.T) {
	title := "Mountain Ranch"
	q := &fakeQuerier{
		listings: []listing.Listing{
			{
				ID:          "00000000-0000-0000-0000-000000000007",
				SourceID:    "test",
				Status:      listing.StatusActive,
				Title:       &title,
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
		},
	}
	h := web.ListingsHandler(q)

	r := httptest.NewRequest(http.MethodGet, "/?q=mountain", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if q.lastFilter == nil {
		t.Fatal("expected QueryListingsFilter to be called with non-nil filter")
	}
	if q.lastFilter.FullText == nil || *q.lastFilter.FullText != "mountain" {
		t.Fatalf("expected FullText=mountain, got %v", q.lastFilter.FullText)
	}
}

func TestListingsHandler_renderForm_searchInput(t *testing.T) {
	q := &fakeQuerier{}
	h := web.ListingsHandler(q)

	r := httptest.NewRequest(http.MethodGet, "/?q=test+query", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, `name="q"`) {
		t.Fatalf("expected search input field in body")
	}
	if !strings.Contains(body, `value="test query"`) {
		t.Fatalf("expected search value in body")
	}
}
