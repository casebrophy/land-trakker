package web_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/source"
	"github.com/cbrophy/land_trakker/foundation/web"
)

// mockHealthQuerier implements web.HealthQuerier for testing.
type mockHealthQuerier struct {
	sources []source.Source
	runs    []source.ScrapeRun
}

func (m *mockHealthQuerier) QuerySources(ctx context.Context) ([]source.Source, error) {
	return m.sources, nil
}

func (m *mockHealthQuerier) QueryRecentRuns(ctx context.Context, sourceID string, limit int) ([]source.ScrapeRun, error) {
	return m.runs, nil
}

func TestHealthDashboardHandler_nilQuerier(t *testing.T) {
	h := web.HealthDashboardHandler(nil)

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "OK") {
		t.Fatal("expected body to contain 'OK'")
	}
	if !strings.Contains(body, "System") {
		t.Fatal("expected body to contain system stats section")
	}
	if !strings.Contains(body, "Goroutines") {
		t.Fatal("expected body to contain Goroutines stat")
	}
}

func TestHealthDashboardHandler_withData(t *testing.T) {
	now := time.Now()
	earlier := now.Add(-2 * time.Hour)
	discovered := 47
	parsed := 45
	errorCount := 2

	mq := &mockHealthQuerier{
		sources: []source.Source{
			{
				ID:          "src-1",
				DisplayName: "Test Source Alpha",
				Enabled:     true,
				LastRunAt:   &earlier,
			},
		},
		runs: []source.ScrapeRun{
			{
				ID:              1,
				SourceID:        "src-1",
				StartedAt:       earlier,
				FinishedAt:      &now,
				Status:          source.RunStatusOK,
				DiscoveredCount: &discovered,
				ParsedCount:     &parsed,
				ErrorCount:      &errorCount,
			},
			{
				ID:        2,
				SourceID:  "src-1",
				StartedAt: earlier.Add(-24 * time.Hour),
				Status:    source.RunStatusFailed,
			},
			{
				ID:        3,
				SourceID:  "src-1",
				StartedAt: earlier.Add(-48 * time.Hour),
				Status:    source.RunStatusPartial,
			},
		},
	}

	h := web.HealthDashboardHandler(mq)

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "OK") {
		t.Fatal("expected body to contain 'OK'")
	}
	if !strings.Contains(body, "Test Source Alpha") {
		t.Fatalf("expected source name in body, got: %s", body)
	}
	if !strings.Contains(body, "System") {
		t.Fatal("expected System section in body")
	}
}
