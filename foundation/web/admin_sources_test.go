package web_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/source"
	"github.com/cbrophy/land_trakker/foundation/web"
	"github.com/go-chi/chi/v5"
)

// -- mock types --

type mockAdminSourcesQuerier struct {
	sources   []source.Source
	runs      []source.ScrapeRun
	eligible  map[string]int
	eligibErr error
}

func (m *mockAdminSourcesQuerier) QuerySources(_ context.Context) ([]source.Source, error) {
	return m.sources, nil
}

func (m *mockAdminSourcesQuerier) QueryRecentRuns(_ context.Context, _ string, _ int) ([]source.ScrapeRun, error) {
	return m.runs, nil
}

func (m *mockAdminSourcesQuerier) CountBackfillEligible(_ context.Context, id string) (int, error) {
	if m.eligibErr != nil {
		return 0, m.eligibErr
	}
	if m.eligible != nil {
		return m.eligible[id], nil
	}
	return 0, nil
}

type mockAdminSourcesUpdater struct {
	src     source.Source
	updated source.Source
	getErr  error
	putErr  error
}

func (m *mockAdminSourcesUpdater) QuerySourceByID(_ context.Context, _ string) (source.Source, error) {
	if m.getErr != nil {
		return source.Source{}, m.getErr
	}
	return m.src, nil
}

func (m *mockAdminSourcesUpdater) UpdateSource(_ context.Context, src source.Source) error {
	m.updated = src
	return m.putErr
}

type mockBackfillTrigger struct {
	triggered []string
}

func (m *mockBackfillTrigger) TriggerBackfill(sourceID string) {
	m.triggered = append(m.triggered, sourceID)
}

// -- GET /admin/sources --

func TestAdminSourcesHandler_nilQuerier(t *testing.T) {
	h := web.AdminSourcesHandler(nil)
	r := httptest.NewRequest(http.MethodGet, "/admin/sources", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Source Configuration") {
		t.Fatal("expected page title in body")
	}
}

func TestAdminSourcesHandler_withSources(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-3 * time.Hour)
	disc := 42
	parsed := 40
	errCnt := 2

	mq := &mockAdminSourcesQuerier{
		sources: []source.Source{
			{
				ID:                             "landwatch",
				DisplayName:                    "LandWatch",
				BaseURL:                        "https://landwatch.com",
				Enabled:                        true,
				RateLimitMS:                    1000,
				Concurrency:                    1,
				AbsenceDaysBeforeStale:         14,
				AbsenceDaysBeforeInactive:      30,
				ConsecutiveMissedRunsThreshold: 3,
				MinResultRatioForInactivation:  0.5,
				LastRunAt:                      &earlier,
			},
		},
		runs: []source.ScrapeRun{
			{
				ID:              1,
				SourceID:        "landwatch",
				StartedAt:       earlier,
				FinishedAt:      &now,
				Status:          source.RunStatusOK,
				DiscoveredCount: &disc,
				ParsedCount:     &parsed,
				ErrorCount:      &errCnt,
			},
		},
		eligible: map[string]int{"landwatch": 7},
	}

	h := web.AdminSourcesHandler(mq)
	r := httptest.NewRequest(http.MethodGet, "/admin/sources", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "LandWatch") {
		t.Fatalf("expected source name in body: %s", body)
	}
	if !strings.Contains(body, "landwatch") {
		t.Fatal("expected source ID in body")
	}
	if !strings.Contains(body, "7") {
		t.Fatal("expected eligible count in body")
	}
}

func TestAdminSourcesHandler_eligibilityUnavailable(t *testing.T) {
	mq := &mockAdminSourcesQuerier{
		sources:  []source.Source{{ID: "src1", DisplayName: "Src One"}},
		eligibErr: errors.New("db unavailable"),
	}

	h := web.AdminSourcesHandler(mq)
	r := httptest.NewRequest(http.MethodGet, "/admin/sources", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "N/A") {
		t.Fatal("expected N/A when eligibility unavailable")
	}
}

func TestAdminSourcesHandler_flashParam(t *testing.T) {
	h := web.AdminSourcesHandler(nil)
	r := httptest.NewRequest(http.MethodGet, "/admin/sources?flash=saved", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Configuration saved") {
		t.Fatal("expected saved flash message in body")
	}
}

// -- POST /admin/sources/{id} --

func chiContextWithID(id string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", id)
		r = r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
		h.ServeHTTP(w, r)
	})
}

func TestAdminSourcesUpdateHandler_nilUpdater(t *testing.T) {
	h := web.AdminSourcesUpdateHandler(nil)
	r := httptest.NewRequest(http.MethodPost, "/admin/sources/landwatch", nil)
	w := httptest.NewRecorder()
	chiContextWithID("landwatch", h).ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "flash=no_db") {
		t.Fatalf("expected no_db flash in redirect, got %s", loc)
	}
}

func TestAdminSourcesUpdateHandler_sourceNotFound(t *testing.T) {
	mu := &mockAdminSourcesUpdater{getErr: errors.New("not found")}
	h := web.AdminSourcesUpdateHandler(mu)

	form := url.Values{"absence_days_stale": {"14"}}
	r := httptest.NewRequest(http.MethodPost, "/admin/sources/missing", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	chiContextWithID("missing", h).ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestAdminSourcesUpdateHandler_success(t *testing.T) {
	mu := &mockAdminSourcesUpdater{
		src: source.Source{
			ID:                             "landwatch",
			DisplayName:                    "LandWatch",
			AbsenceDaysBeforeStale:         14,
			AbsenceDaysBeforeInactive:      30,
			ConsecutiveMissedRunsThreshold: 3,
			MinResultRatioForInactivation:  0.5,
			RateLimitMS:                    1000,
			Concurrency:                    1,
			Enabled:                        true,
		},
	}
	h := web.AdminSourcesUpdateHandler(mu)

	form := url.Values{
		"absence_days_stale":   {"21"},
		"absence_days_inactive": {"45"},
		"consecutive_missed":   {"5"},
		"min_ratio":            {"0.600"},
		"rate_limit_ms":        {"2000"},
		"concurrency":          {"2"},
		"enabled":              {"true"},
	}
	r := httptest.NewRequest(http.MethodPost, "/admin/sources/landwatch", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	chiContextWithID("landwatch", h).ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "flash=saved") {
		t.Fatalf("expected saved flash in redirect, got %s", loc)
	}

	if mu.updated.AbsenceDaysBeforeStale != 21 {
		t.Errorf("expected AbsenceDaysBeforeStale=21, got %d", mu.updated.AbsenceDaysBeforeStale)
	}
	if mu.updated.AbsenceDaysBeforeInactive != 45 {
		t.Errorf("expected AbsenceDaysBeforeInactive=45, got %d", mu.updated.AbsenceDaysBeforeInactive)
	}
	if mu.updated.ConsecutiveMissedRunsThreshold != 5 {
		t.Errorf("expected ConsecutiveMissedRunsThreshold=5, got %d", mu.updated.ConsecutiveMissedRunsThreshold)
	}
	if mu.updated.RateLimitMS != 2000 {
		t.Errorf("expected RateLimitMS=2000, got %d", mu.updated.RateLimitMS)
	}
	if mu.updated.Concurrency != 2 {
		t.Errorf("expected Concurrency=2, got %d", mu.updated.Concurrency)
	}
	if !mu.updated.Enabled {
		t.Error("expected Enabled=true")
	}
}

func TestAdminSourcesUpdateHandler_disableSource(t *testing.T) {
	mu := &mockAdminSourcesUpdater{
		src: source.Source{ID: "src1", Enabled: true},
	}
	h := web.AdminSourcesUpdateHandler(mu)

	form := url.Values{"enabled": {"false"}}
	r := httptest.NewRequest(http.MethodPost, "/admin/sources/src1", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	chiContextWithID("src1", h).ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if mu.updated.Enabled {
		t.Error("expected Enabled=false after update")
	}
}

// -- POST /admin/sources/{id}/backfill --

func TestAdminSourcesBackfillHandler_nilTrigger(t *testing.T) {
	h := web.AdminSourcesBackfillHandler(nil)
	r := httptest.NewRequest(http.MethodPost, "/admin/sources/landwatch/backfill", nil)
	w := httptest.NewRecorder()
	chiContextWithID("landwatch", h).ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "flash=backfill_started") {
		t.Fatalf("expected backfill_started flash, got %s", loc)
	}
}

func TestAdminSourcesBackfillHandler_triggersCalled(t *testing.T) {
	bt := &mockBackfillTrigger{}
	h := web.AdminSourcesBackfillHandler(bt)

	r := httptest.NewRequest(http.MethodPost, "/admin/sources/knipe/backfill", nil)
	w := httptest.NewRecorder()
	chiContextWithID("knipe", h).ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected 303, got %d", w.Code)
	}
	if len(bt.triggered) != 1 || bt.triggered[0] != "knipe" {
		t.Errorf("expected trigger called with 'knipe', got %v", bt.triggered)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "flash=backfill_started") {
		t.Fatalf("expected backfill_started flash, got %s", loc)
	}
}
