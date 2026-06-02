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

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/foundation/web"
)

type mockDuplicatesQuerier struct {
	possibleDups []listing.PossibleDuplicate
	listings     map[string]listing.Listing
	updateCalls  []struct {
		aID      string
		bID      string
		decision string
	}
	queryError  error
	updateError error
}

func (m *mockDuplicatesQuerier) QueryPossibleDuplicates(ctx context.Context, decision *string) ([]listing.PossibleDuplicate, error) {
	if m.queryError != nil {
		return nil, m.queryError
	}
	if decision == nil {
		return m.possibleDups, nil
	}
	var filtered []listing.PossibleDuplicate
	for _, pd := range m.possibleDups {
		if pd.UserDecision == decision {
			filtered = append(filtered, pd)
		}
	}
	return filtered, nil
}

func (m *mockDuplicatesQuerier) UpdateDuplicateDecision(ctx context.Context, aID, bID string, decision string) error {
	m.updateCalls = append(m.updateCalls, struct {
		aID      string
		bID      string
		decision string
	}{aID, bID, decision})
	return m.updateError
}

func (m *mockDuplicatesQuerier) QueryListingByID(ctx context.Context, id string) (listing.Listing, error) {
	if l, ok := m.listings[id]; ok {
		return l, nil
	}
	return listing.Listing{}, fmt.Errorf("not found")
}

func TestDuplicatesHandler_nil_returns503(t *testing.T) {
	h := web.DuplicatesHandler(nil)

	r := httptest.NewRequest(http.MethodGet, "/duplicates", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDuplicatesHandler_empty(t *testing.T) {
	dq := &mockDuplicatesQuerier{
		possibleDups: []listing.PossibleDuplicate{},
		listings:     map[string]listing.Listing{},
	}
	h := web.DuplicatesHandler(dq)

	r := httptest.NewRequest(http.MethodGet, "/duplicates", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Duplicates") {
		t.Fatal("expected page to mention 'Duplicates'")
	}
}

func TestDuplicatesHandler_withPairs(t *testing.T) {
	title1 := "Beautiful 50 acre ranch"
	price1 := int64(5000000) // $50k
	addr1 := "123 Main St"
	city1 := "Boise"

	title2 := "Beautiful 50 acre property"
	price2 := int64(4950000) // $49.5k
	addr2 := "123 Main St"
	city2 := "Boise"

	dq := &mockDuplicatesQuerier{
		possibleDups: []listing.PossibleDuplicate{
			{
				ListingAID:   "list-a-id",
				ListingBID:   "list-b-id",
				Score:        0.95,
				Reasons:      []string{listing.DedupReasonGeo, listing.DedupReasonPrice, listing.DedupReasonTitle},
				DetectedAt:   time.Now(),
				UserDecision: nil,
			},
		},
		listings: map[string]listing.Listing{
			"list-a-id": {
				ID:         "list-a-id",
				Title:      &title1,
				PriceCents: &price1,
				URL:        "https://landwatch.com/listing-a",
				AddressLine: &addr1,
				City:       &city1,
			},
			"list-b-id": {
				ID:         "list-b-id",
				Title:      &title2,
				PriceCents: &price2,
				URL:        "https://landwatch.com/listing-b",
				AddressLine: &addr2,
				City:       &city2,
			},
		},
	}
	h := web.DuplicatesHandler(dq)

	r := httptest.NewRequest(http.MethodGet, "/duplicates", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, title1) {
		t.Fatalf("expected body to contain listing A title %q", title1)
	}
	if !strings.Contains(body, title2) {
		t.Fatalf("expected body to contain listing B title %q", title2)
	}
	if !strings.Contains(body, "Location") {
		t.Fatal("expected reasons to include 'Location'")
	}
	if !strings.Contains(body, "95%") {
		t.Fatal("expected score percentage to be shown")
	}
}

func TestDuplicatesHandler_queryError(t *testing.T) {
	dq := &mockDuplicatesQuerier{
		queryError: fmt.Errorf("database error"),
	}
	h := web.DuplicatesHandler(dq)

	r := httptest.NewRequest(http.MethodGet, "/duplicates", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestDuplicatesUpdateHandler_nil_returns503(t *testing.T) {
	h := web.DuplicatesUpdateHandler(nil)

	form := url.Values{
		"action": {"same"},
		"a_id":   {"a-uuid"},
		"b_id":   {"b-uuid"},
	}
	r := httptest.NewRequest(http.MethodPost, "/duplicates/decision", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDuplicatesUpdateHandler_recordsSame(t *testing.T) {
	dq := &mockDuplicatesQuerier{
		possibleDups: []listing.PossibleDuplicate{},
		listings:     map[string]listing.Listing{},
	}
	h := web.DuplicatesUpdateHandler(dq)

	form := url.Values{
		"action": {"same"},
		"a_id":   {"a-uuid"},
		"b_id":   {"b-uuid"},
	}
	r := httptest.NewRequest(http.MethodPost, "/duplicates/decision", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (303), got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/duplicates" {
		t.Fatalf("expected redirect to /duplicates, got %q", loc)
	}

	if len(dq.updateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(dq.updateCalls))
	}
	call := dq.updateCalls[0]
	if call.aID != "a-uuid" || call.bID != "b-uuid" || call.decision != "same" {
		t.Fatalf("unexpected update call: %+v", call)
	}
}

func TestDuplicatesUpdateHandler_recordsDifferent(t *testing.T) {
	dq := &mockDuplicatesQuerier{
		possibleDups: []listing.PossibleDuplicate{},
		listings:     map[string]listing.Listing{},
	}
	h := web.DuplicatesUpdateHandler(dq)

	form := url.Values{
		"action": {"different"},
		"a_id":   {"a-uuid"},
		"b_id":   {"b-uuid"},
	}
	r := httptest.NewRequest(http.MethodPost, "/duplicates/decision", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (303), got %d", w.Code)
	}

	if len(dq.updateCalls) != 1 {
		t.Fatalf("expected 1 update call, got %d", len(dq.updateCalls))
	}
	if dq.updateCalls[0].decision != "different" {
		t.Fatalf("expected decision 'different', got %q", dq.updateCalls[0].decision)
	}
}

func TestDuplicatesUpdateHandler_missingFields(t *testing.T) {
	dq := &mockDuplicatesQuerier{
		possibleDups: []listing.PossibleDuplicate{},
		listings:     map[string]listing.Listing{},
	}
	h := web.DuplicatesUpdateHandler(dq)

	// Missing a_id
	form := url.Values{
		"action": {"same"},
		"b_id":   {"b-uuid"},
	}
	r := httptest.NewRequest(http.MethodPost, "/duplicates/decision", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDuplicatesUpdateHandler_invalidAction(t *testing.T) {
	dq := &mockDuplicatesQuerier{
		possibleDups: []listing.PossibleDuplicate{},
		listings:     map[string]listing.Listing{},
	}
	h := web.DuplicatesUpdateHandler(dq)

	form := url.Values{
		"action": {"invalid_action"},
		"a_id":   {"a-uuid"},
		"b_id":   {"b-uuid"},
	}
	r := httptest.NewRequest(http.MethodPost, "/duplicates/decision", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDuplicatesUpdateHandler_updateError(t *testing.T) {
	dq := &mockDuplicatesQuerier{
		updateError: fmt.Errorf("database error"),
	}
	h := web.DuplicatesUpdateHandler(dq)

	form := url.Values{
		"action": {"same"},
		"a_id":   {"a-uuid"},
		"b_id":   {"b-uuid"},
	}
	r := httptest.NewRequest(http.MethodPost, "/duplicates/decision", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
