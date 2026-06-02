package web_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cbrophy/land_trakker/foundation/web"
)

func TestLogCapture_Handler(t *testing.T) {
	lc := web.NewLogCapture(10)
	logger := slog.New(lc)

	logger.Info("test message 1")
	logger.Warn("test warning", "key", "value")
	logger.Error("test error", "code", 500)

	logs := lc.Recent(10)
	if len(logs) != 3 {
		t.Fatalf("expected 3 logs, got %d", len(logs))
	}

	// Most recent first
	if !strings.Contains(logs[0], "test error") {
		t.Errorf("expected error log first, got: %s", logs[0])
	}
	if !strings.Contains(logs[1], "test warning") {
		t.Errorf("expected warning log second, got: %s", logs[1])
	}
	if !strings.Contains(logs[2], "test message 1") {
		t.Errorf("expected info log third, got: %s", logs[2])
	}
}

func TestLogCapture_CircularBuffer(t *testing.T) {
	lc := web.NewLogCapture(5)
	logger := slog.New(lc)

	// Fill buffer
	for i := 0; i < 5; i++ {
		logger.Info("message", "n", i)
	}

	// Overflow
	logger.Info("overflow1")
	logger.Info("overflow2")

	logs := lc.Recent(10)
	if len(logs) != 5 {
		t.Fatalf("expected 5 logs (buffer size), got %d", len(logs))
	}

	// Most recent should be overflow2
	if !strings.Contains(logs[0], "overflow2") {
		t.Errorf("expected overflow2, got: %s", logs[0])
	}
	if !strings.Contains(logs[1], "overflow1") {
		t.Errorf("expected overflow1, got: %s", logs[1])
	}
	// Oldest should be message with n=2 (messages 0 and 1 were overwritten)
	if !strings.Contains(logs[4], "n=2") {
		t.Errorf("expected message n=2 (oldest), got: %s", logs[4])
	}
}

func TestLogCapture_Recent_Limit(t *testing.T) {
	lc := web.NewLogCapture(20)
	logger := slog.New(lc)

	for i := 0; i < 10; i++ {
		logger.Info("message", "n", i)
	}

	tests := []struct {
		name     string
		limit    int
		expected int
	}{
		{"default 50", 50, 10},
		{"limit 5", 5, 5},
		{"limit 0 defaults to 50", 0, 10}, // only 10 logs exist
		{"limit negative defaults to 50", -1, 10}, // only 10 logs exist
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := lc.Recent(tt.limit)
			if len(logs) != tt.expected {
				t.Fatalf("expected %d logs, got %d", tt.expected, len(logs))
			}
		})
	}
}

func TestLogCapture_Filter(t *testing.T) {
	lc := web.NewLogCapture(20)
	logger := slog.New(lc)

	logger.Info("info message 1")
	logger.Warn("warning message")
	logger.Error("error message 1")
	logger.Info("info message 2")
	logger.Info("another message")

	tests := []struct {
		name     string
		query    string
		limit    int
		expected int
		wantMsg  string
	}{
		{"filter error", "error", 10, 1, "error message 1"},
		{"filter info", "info", 10, 2, "info message"},
		{"filter case insensitive", "ERROR", 10, 1, "error message 1"},
		{"filter message", "message", 10, 5, ""}, // 5 logs contain "message"
		{"filter no match", "NOTFOUND", 10, 0, ""},
		{"filter with limit", "message", 2, 2, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := lc.Filter(tt.query, tt.limit)
			if len(logs) != tt.expected {
				t.Fatalf("expected %d matches, got %d", tt.expected, len(logs))
			}
			if tt.wantMsg != "" && len(logs) > 0 {
				if !strings.Contains(logs[0], tt.wantMsg) {
					t.Errorf("expected to find %q in first result, got: %s", tt.wantMsg, logs[0])
				}
			}
		})
	}
}

func TestLogsHandler_Empty(t *testing.T) {
	lc := web.NewLogCapture(10)

	h := web.LogsHandler(lc)
	r := httptest.NewRequest(http.MethodGet, "/health/logs", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "No logs available") {
		t.Errorf("expected empty message in body")
	}
}

func TestLogsHandler_DisplayLogs(t *testing.T) {
	lc := web.NewLogCapture(20)
	logger := slog.New(lc)

	logger.Info("test log entry 1")
	logger.Warn("test log entry 2")
	logger.Error("test log entry 3")

	h := web.LogsHandler(lc)
	r := httptest.NewRequest(http.MethodGet, "/health/logs", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()

	if !strings.Contains(body, "test log entry 1") {
		t.Errorf("expected log entry 1 in body")
	}
	if !strings.Contains(body, "test log entry 2") {
		t.Errorf("expected log entry 2 in body")
	}
	if !strings.Contains(body, "test log entry 3") {
		t.Errorf("expected log entry 3 in body")
	}
}

func TestLogsHandler_FilterQuery(t *testing.T) {
	lc := web.NewLogCapture(20)
	logger := slog.New(lc)

	logger.Info("user login event")
	logger.Warn("user logout event")
	logger.Error("error in payment")
	logger.Info("database query")

	h := web.LogsHandler(lc)
	r := httptest.NewRequest(http.MethodGet, "/health/logs?q=error", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()

	if !strings.Contains(body, "error in payment") {
		t.Errorf("expected filtered log (error) in body")
	}
	// Should not contain non-matching logs
	if strings.Contains(body, "user login") {
		t.Errorf("expected login log to be filtered out")
	}
	if strings.Contains(body, "database query") {
		t.Errorf("expected database log to be filtered out")
	}
}

func TestLogsHandler_LimitParameter(t *testing.T) {
	lc := web.NewLogCapture(50)
	logger := slog.New(lc)

	for i := 0; i < 30; i++ {
		logger.Info("message", "n", i)
	}

	h := web.LogsHandler(lc)
	r := httptest.NewRequest(http.MethodGet, "/health/logs?limit=10", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()

	// Count log entries by looking for "n=" pattern
	logLines := strings.Count(body, "n=")
	if logLines != 10 {
		t.Errorf("expected 10 log entries, got %d", logLines)
	}
}

func TestLogsHandler_InvalidLimit(t *testing.T) {
	lc := web.NewLogCapture(20)
	logger := slog.New(lc)

	logger.Info("test message")

	h := web.LogsHandler(lc)

	tests := []struct {
		name           string
		query          string
		expectDefaults bool
	}{
		{"invalid limit", "?limit=abc", true},
		{"negative limit", "?limit=-5", true},
		{"zero limit", "?limit=0", true},
		{"too large limit", "?limit=1000", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/health/logs"+tt.query, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
		})
	}
}
