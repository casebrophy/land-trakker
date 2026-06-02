package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/search"
	"github.com/cbrophy/land_trakker/foundation/web"
)

func TestDigestHandler_nil_returns503(t *testing.T) {
	h := web.DigestHandler(nil, nil)
	r := httptest.NewRequest(http.MethodGet, "/digest", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDigestHandler_empty(t *testing.T) {
	sc := &fakeSearchCore{}
	h := web.DigestHandler(sc, nil)
	r := httptest.NewRequest(http.MethodGet, "/digest", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Land Trakker") {
		t.Fatal("expected page to contain 'Land Trakker'")
	}
	if !strings.Contains(body, "All caught up") {
		t.Fatalf("expected empty state message, got:\n%s", body)
	}
}

func TestDigestHandler_withHits(t *testing.T) {
	listingID := "00000000-0000-0000-0000-000000000099"
	searchID := "srch-1"
	title := "Riverside 50 acres"
	price := int64(45000000)
	acres := 50.0

	sc := &fakeSearchCore{
		searches: []search.SavedSearch{
			{ID: searchID, Name: "River Searches", Enabled: true},
		},
		hits: []search.SearchHit{
			{
				ID:            42,
				SavedSearchID: searchID,
				ListingID:     listingID,
				HitAt:         time.Now(),
				Reason:        search.ReasonNew,
				Seen:          false,
			},
		},
	}
	lq := &fakeQuerier{
		listings: []listing.Listing{
			{
				ID:          listingID,
				SourceID:    "test",
				Status:      listing.StatusActive,
				Title:       &title,
				PriceCents:  &price,
				Acres:       &acres,
				FirstSeenAt: time.Now(),
				LastSeenAt:  time.Now(),
			},
		},
	}

	h := web.DigestHandler(sc, lq)
	r := httptest.NewRequest(http.MethodGet, "/digest", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Riverside 50 acres") {
		t.Fatalf("expected listing title in body, got:\n%s", body)
	}
	if !strings.Contains(body, "New Listing") {
		t.Fatalf("expected hit reason 'New Listing' in body")
	}
	if !strings.Contains(body, "River Searches") {
		t.Fatalf("expected search name in body")
	}
	if !strings.Contains(body, "Mark all seen") {
		t.Fatalf("expected 'Mark all seen' button in body")
	}
	if !strings.Contains(body, "42") {
		t.Fatalf("expected hit ID 42 in form data")
	}
}

func TestDigestHandler_withNilListingsQuerier(t *testing.T) {
	searchID := "srch-2"
	sc := &fakeSearchCore{
		searches: []search.SavedSearch{
			{ID: searchID, Name: "Test Search", Enabled: true},
		},
		hits: []search.SearchHit{
			{
				ID:            10,
				SavedSearchID: searchID,
				ListingID:     "some-listing-id",
				HitAt:         time.Now(),
				Reason:        search.ReasonPriceDrop,
			},
		},
	}

	h := web.DigestHandler(sc, nil)
	r := httptest.NewRequest(http.MethodGet, "/digest", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Price Drop") {
		t.Fatalf("expected 'Price Drop' reason in body")
	}
}

func TestDigestMarkSeenHandler_nil_returns503(t *testing.T) {
	h := web.DigestMarkSeenHandler(nil)
	r := httptest.NewRequest(http.MethodPost, "/digest/mark-seen", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDigestMarkSeenHandler_marksAndRedirects(t *testing.T) {
	sc := &fakeSearchCore{}
	h := web.DigestMarkSeenHandler(sc)

	form := url.Values{"hit_ids": {"1,2,3"}}
	r := httptest.NewRequest(http.MethodPost, "/digest/mark-seen", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/digest" {
		t.Fatalf("expected redirect to /digest, got %q", loc)
	}
	if len(sc.markSeenIDs) != 3 {
		t.Fatalf("expected 3 IDs marked, got %v", sc.markSeenIDs)
	}
	if sc.markSeenIDs[0] != 1 || sc.markSeenIDs[1] != 2 || sc.markSeenIDs[2] != 3 {
		t.Fatalf("expected IDs [1,2,3], got %v", sc.markSeenIDs)
	}
}

func TestDigestMarkSeenHandler_emptyIDs_redirects(t *testing.T) {
	sc := &fakeSearchCore{}
	h := web.DigestMarkSeenHandler(sc)

	form := url.Values{"hit_ids": {""}}
	r := httptest.NewRequest(http.MethodPost, "/digest/mark-seen", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if len(sc.markSeenIDs) != 0 {
		t.Fatalf("expected no IDs marked for empty input")
	}
}
