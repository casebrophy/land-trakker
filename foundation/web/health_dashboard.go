package web

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"runtime"
	"time"

	"github.com/cbrophy/land_trakker/business/domain/source"
)

// HealthQuerier provides per-source run data for the health dashboard.
type HealthQuerier interface {
	QuerySources(ctx context.Context) ([]source.Source, error)
	QueryRecentRuns(ctx context.Context, sourceID string, limit int) ([]source.ScrapeRun, error)
}

var healthTmpl = template.Must(template.ParseFS(templateFS, "templates/health.html"))

type sparkDay struct {
	Status string // "ok","partial","failed","running","none"
	Title  string // tooltip text
}

type sourcePanelData struct {
	Name       string
	LastRunAt  string // "2 hours ago" or "never"
	LastStatus string // "ok","partial","failed","running","never"
	Sparkline  []sparkDay
	ErrorRate  string // "0.0%"
}

type systemStatsData struct {
	Goroutines int
	AllocMB    string
	SysMB      string
	NumGC      uint32
}

type healthPageData struct {
	Sources []sourcePanelData
	System  systemStatsData
}

// HealthDashboardHandler returns an http.HandlerFunc that renders the health dashboard.
func HealthDashboardHandler(hq HealthQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var panels []sourcePanelData
		if hq != nil {
			sources, err := hq.QuerySources(ctx)
			if err == nil {
				for _, src := range sources {
					runs, _ := hq.QueryRecentRuns(ctx, src.ID, 30)
					panels = append(panels, buildSourcePanel(src, runs))
				}
			}
		}

		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)

		data := healthPageData{
			Sources: panels,
			System: systemStatsData{
				Goroutines: runtime.NumGoroutine(),
				AllocMB:    fmt.Sprintf("%.1f", float64(ms.Alloc)/1024/1024),
				SysMB:      fmt.Sprintf("%.1f", float64(ms.Sys)/1024/1024),
				NumGC:      ms.NumGC,
			},
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		healthTmpl.Execute(w, data)
	}
}

func buildSourcePanel(src source.Source, runs []source.ScrapeRun) sourcePanelData {
	panel := sourcePanelData{
		Name:       src.DisplayName,
		LastRunAt:  "never",
		LastStatus: "never",
	}

	if src.LastRunAt != nil {
		panel.LastRunAt = timeAgo(*src.LastRunAt)
	}

	if len(runs) > 0 {
		panel.LastStatus = string(runs[0].Status)
	}

	// Build sparkline — one square per run (most recent first in runs slice,
	// display oldest-to-newest so most recent is on the right).
	spark := make([]sparkDay, len(runs))
	for i, run := range runs {
		idx := len(runs) - 1 - i // reverse: oldest at index 0
		discovered := 0
		if run.DiscoveredCount != nil {
			discovered = *run.DiscoveredCount
		}
		title := fmt.Sprintf("%s: %s (%d discovered)", run.StartedAt.Format("2006-01-02"), string(run.Status), discovered)
		spark[idx] = sparkDay{
			Status: string(run.Status),
			Title:  title,
		}
	}
	panel.Sparkline = spark

	// Calculate error rate across all runs.
	var totalErrors, totalParsed int
	for _, run := range runs {
		if run.ErrorCount != nil {
			totalErrors += *run.ErrorCount
		}
		if run.ParsedCount != nil {
			totalParsed += *run.ParsedCount
		}
	}
	if totalParsed > 0 {
		rate := float64(totalErrors) / float64(totalParsed) * 100
		panel.ErrorRate = fmt.Sprintf("%.1f%%", rate)
	} else {
		panel.ErrorRate = "0.0%"
	}

	return panel
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
