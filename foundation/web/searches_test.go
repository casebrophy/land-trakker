package web_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/search"
	"github.com/cbrophy/land_trakker/foundation/web"
)

// fakeSearchCore is a test double for web.SearchCore.
type fakeSearchCore struct {
	searches      []search.SavedSearch
	hits          []search.SearchHit
	createCalled  bool
	updateCalled  *search.SavedSearch
	deletedID     string
	markSeenIDs   []int64
}

func (f *fakeSearchCore) QuerySavedSearches(_ context.Context) ([]search.SavedSearch, error) {
	return f.searches, nil
}

func (f *fakeSearchCore) QuerySavedSearchByID(_ context.Context, id string) (search.SavedSearch, error) {
	for _, ss := range f.searches {
		if ss.ID == id {
			return ss, nil
		}
	}
	return search.SavedSearch{}, fmt.Errorf("no rows in result set")
}

func (f *fakeSearchCore) CreateSavedSearch(_ context.Context, ss search.SavedSearch) (search.SavedSearch, error) {
	f.createCalled = true
	ss.ID = "new-id"
	f.searches = append(f.searches, ss)
	return ss, nil
}

func (f *fakeSearchCore) UpdateSavedSearch(_ context.Context, ss search.SavedSearch) (search.SavedSearch, error) {
	f.updateCalled = &ss
	for i, s := range f.searches {
		if s.ID == ss.ID {
			f.searches[i] = ss
			return ss, nil
		}
	}
	return search.SavedSearch{}, fmt.Errorf("no rows in result set")
}

func (f *fakeSearchCore) DeleteSavedSearch(_ context.Context, id string) error {
	f.deletedID = id
	return nil
}

func (f *fakeSearchCore) QueryUnseen(_ context.Context, _ int) ([]search.SearchHit, error) {
	return f.hits, nil
}

func (f *fakeSearchCore) MarkHitsSeen(_ context.Context, ids []int64) error {
	f.markSeenIDs = ids
	return nil
}

func TestSearchesHandler_nil_returns503(t *testing.T) {
	h := web.SearchesHandler(nil)
	r := httptest.NewRequest(http.MethodGet, "/searches", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestSearchesHandler_empty(t *testing.T) {
	sc := &fakeSearchCore{}
	h := web.SearchesHandler(sc)
	r := httptest.NewRequest(http.MethodGet, "/searches", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Land Trakker") {
		t.Fatal("expected page to contain 'Land Trakker'")
	}
}

func TestSearchesHandler_withSearches(t *testing.T) {
	sc := &fakeSearchCore{
		searches: []search.SavedSearch{
			{
				ID:        "abc-123",
				Name:      "Idaho 40ac under 500k",
				Enabled:   true,
				CreatedAt: time.Now(),
			},
		},
	}
	h := web.SearchesHandler(sc)
	r := httptest.NewRequest(http.MethodGet, "/searches", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Idaho 40ac under 500k") {
		t.Fatalf("expected body to contain search name, got:\n%s", body)
	}
	if !strings.Contains(body, "enabled") {
		t.Fatalf("expected body to show enabled status")
	}
}

func TestSearchesNewHandler_rendersForm(t *testing.T) {
	h := web.SearchesNewHandler()
	r := httptest.NewRequest(http.MethodGet, "/searches/new", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "New Saved Search") {
		t.Fatalf("expected 'New Saved Search' in body")
	}
	if !strings.Contains(body, `action="/searches"`) {
		t.Fatalf("expected form action /searches")
	}
}

func TestSearchesCreateHandler_nil_returns503(t *testing.T) {
	h := web.SearchesCreateHandler(nil)
	r := httptest.NewRequest(http.MethodPost, "/searches", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestSearchesCreateHandler_missingName_returns422(t *testing.T) {
	sc := &fakeSearchCore{}
	h := web.SearchesCreateHandler(sc)

	form := url.Values{"name": {""}}
	r := httptest.NewRequest(http.MethodPost, "/searches", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", w.Code)
	}
}

func TestSearchesCreateHandler_success_redirects(t *testing.T) {
	sc := &fakeSearchCore{}
	h := web.SearchesCreateHandler(sc)

	form := url.Values{
		"name":      {"Mountain Ranch"},
		"enabled":   {"true"},
		"acres_min": {"20"},
		"price_max": {"500000"},
	}
	r := httptest.NewRequest(http.MethodPost, "/searches", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/searches" {
		t.Fatalf("expected redirect to /searches, got %q", loc)
	}
	if !sc.createCalled {
		t.Fatal("expected CreateSavedSearch to be called")
	}
}

func TestSearchesCreateHandler_parsesFilter(t *testing.T) {
	sc := &fakeSearchCore{}
	h := web.SearchesCreateHandler(sc)

	acresMin := 40.0
	priceMax := int64(75000000) // $750,000 in cents

	form := url.Values{
		"name":         {"Test Search"},
		"enabled":      {"true"},
		"acres_min":    {"40"},
		"price_max":    {"750000"},
		"counties":     {"Ada, Bonner"},
		"attr_water":   {"true"},
	}
	r := httptest.NewRequest(http.MethodPost, "/searches", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if len(sc.searches) == 0 {
		t.Fatal("expected a saved search to be created")
	}
	created := sc.searches[0]
	if created.Query.AcresMin == nil || *created.Query.AcresMin != acresMin {
		t.Fatalf("expected AcresMin=%v, got %v", acresMin, created.Query.AcresMin)
	}
	if created.Query.PriceMax == nil || *created.Query.PriceMax != priceMax {
		t.Fatalf("expected PriceMax=%d, got %v", priceMax, created.Query.PriceMax)
	}
	if len(created.Query.Counties) != 2 {
		t.Fatalf("expected 2 counties, got %v", created.Query.Counties)
	}
	if created.Query.AttrWaterFrontage == nil || !*created.Query.AttrWaterFrontage {
		t.Fatal("expected AttrWaterFrontage=true")
	}
}

func TestSearchesEditHandler_notFound(t *testing.T) {
	sc := &fakeSearchCore{}
	h := web.SearchesEditHandler(sc)

	r := httptest.NewRequest(http.MethodGet, "/searches/no-such/edit", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "no-such")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestSearchesEditHandler_rendersForm(t *testing.T) {
	acresMin := 10.0
	sc := &fakeSearchCore{
		searches: []search.SavedSearch{
			{
				ID:      "edit-id",
				Name:    "River Ranch",
				Enabled: true,
				Query: listing.ListingFilter{
					AcresMin: &acresMin,
				},
				CreatedAt: time.Now(),
			},
		},
	}
	h := web.SearchesEditHandler(sc)

	r := httptest.NewRequest(http.MethodGet, "/searches/edit-id/edit", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "edit-id")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Edit Saved Search") {
		t.Fatalf("expected 'Edit Saved Search' in body")
	}
	if !strings.Contains(body, "River Ranch") {
		t.Fatalf("expected search name in body")
	}
	if !strings.Contains(body, "10") {
		t.Fatalf("expected acres_min=10 in body")
	}
}

func TestSearchesUpdateHandler_success_redirects(t *testing.T) {
	sc := &fakeSearchCore{
		searches: []search.SavedSearch{
			{ID: "upd-id", Name: "Old Name", Enabled: true},
		},
	}
	h := web.SearchesUpdateHandler(sc)

	form := url.Values{
		"name":    {"New Name"},
		"enabled": {"true"},
	}
	r := httptest.NewRequest(http.MethodPost, "/searches/upd-id", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "upd-id")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if sc.updateCalled == nil {
		t.Fatal("expected UpdateSavedSearch to be called")
	}
	if sc.updateCalled.Name != "New Name" {
		t.Fatalf("expected updated name 'New Name', got %q", sc.updateCalled.Name)
	}
}

func TestSearchesDeleteHandler_success_redirects(t *testing.T) {
	sc := &fakeSearchCore{}
	h := web.SearchesDeleteHandler(sc)

	r := httptest.NewRequest(http.MethodPost, "/searches/del-id/delete", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "del-id")
	r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if sc.deletedID != "del-id" {
		t.Fatalf("expected DeleteSavedSearch called with 'del-id', got %q", sc.deletedID)
	}
}
