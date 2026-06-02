package web

import (
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// LogCapture is a circular buffer that implements slog.Handler to capture recent logs.
type LogCapture struct {
	mu      sync.RWMutex
	entries []string
	pos     int // current write position (circular)
	filled  bool
}

// NewLogCapture creates a new log capture buffer with the given capacity.
func NewLogCapture(capacity int) *LogCapture {
	if capacity < 1 {
		capacity = 100
	}
	return &LogCapture{
		entries: make([]string, capacity),
	}
}

// Enabled reports whether the handler handles records at the given level.
func (lc *LogCapture) Enabled(ctx context.Context, level slog.Level) bool {
	return true
}

// Handle stores the log record in the circular buffer.
func (lc *LogCapture) Handle(ctx context.Context, record slog.Record) error {
	// Convert record to JSON-like string format
	logStr := record.Message
	if record.NumAttrs() > 0 {
		attrs := []string{}
		record.Attrs(func(a slog.Attr) bool {
			attrs = append(attrs, fmt.Sprintf("%s=%v", a.Key, a.Value.Any()))
			return true
		})
		if len(attrs) > 0 {
			logStr += " " + strings.Join(attrs, " ")
		}
	}

	lc.mu.Lock()
	defer lc.mu.Unlock()

	lc.entries[lc.pos] = logStr
	lc.pos++
	if lc.pos >= len(lc.entries) {
		lc.pos = 0
		lc.filled = true
	}

	return nil
}

// WithAttrs returns a new handler with the given attributes.
func (lc *LogCapture) WithAttrs(attrs []slog.Attr) slog.Handler {
	// For our purposes, we don't need to preserve attributes
	return lc
}

// WithGroup returns a new handler with the given group.
func (lc *LogCapture) WithGroup(name string) slog.Handler {
	// For our purposes, we don't need to preserve groups
	return lc
}

// Recent returns the most recent N log entries (in reverse chronological order).
func (lc *LogCapture) Recent(n int) []string {
	if n < 1 {
		n = 50
	}

	lc.mu.RLock()
	defer lc.mu.RUnlock()

	total := len(lc.entries)
	if !lc.filled {
		total = lc.pos
	}

	if total == 0 {
		return []string{}
	}

	if n > total {
		n = total
	}

	result := make([]string, 0, n)
	for i := 0; i < n; i++ {
		idx := (lc.pos - 1 - i + len(lc.entries)) % len(lc.entries)
		if lc.entries[idx] != "" {
			result = append(result, lc.entries[idx])
		}
	}

	return result
}

// Filter returns log entries matching a substring (case-insensitive).
func (lc *LogCapture) Filter(query string, n int) []string {
	if n < 1 {
		n = 50
	}

	lc.mu.RLock()
	defer lc.mu.RUnlock()

	query = strings.ToLower(query)

	result := make([]string, 0)
	pos := lc.pos

	// Iterate through buffer in reverse (most recent first)
	for i := 0; i < len(lc.entries) && len(result) < n; i++ {
		pos--
		if pos < 0 {
			pos = len(lc.entries) - 1
		}

		entry := lc.entries[pos]
		if entry != "" && strings.Contains(strings.ToLower(entry), query) {
			result = append(result, entry)
		}
	}

	return result
}

var logsTmpl = template.Must(template.ParseFS(templateFS, "templates/logs.html"))

type logsPageData struct {
	Logs  []string
	Query string
	Limit int
}

// LogsHandler returns an http.HandlerFunc that displays recent logs.
func LogsHandler(lc *LogCapture) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := strings.TrimSpace(r.URL.Query().Get("q"))
		limit := 50

		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				if n > 500 {
					n = 500
				}
				limit = n
			}
		}

		var logs []string
		if query != "" {
			logs = lc.Filter(query, limit)
		} else {
			logs = lc.Recent(limit)
		}

		data := logsPageData{
			Logs:  logs,
			Query: query,
			Limit: limit,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		logsTmpl.Execute(w, data)
	}
}
