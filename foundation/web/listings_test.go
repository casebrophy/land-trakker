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
	listings  []listing.Listing
	snapshots map[string][]listing.ListingSnapshot
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
