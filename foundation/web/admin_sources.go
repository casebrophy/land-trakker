package web

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	"github.com/cbrophy/land_trakker/business/domain/source"
	"github.com/go-chi/chi/v5"
)

// AdminSourcesQuerier provides read access for the /admin/sources page.
type AdminSourcesQuerier interface {
	QuerySources(ctx context.Context) ([]source.Source, error)
	QueryRecentRuns(ctx context.Context, sourceID string, limit int) ([]source.ScrapeRun, error)
	CountBackfillEligible(ctx context.Context, sourceID string) (int, error)
}

// AdminSourcesUpdater provides write access for per-source configuration.
type AdminSourcesUpdater interface {
	QuerySourceByID(ctx context.Context, id string) (source.Source, error)
	UpdateSource(ctx context.Context, src source.Source) error
}

// BackfillTrigger can initiate a background backfill job for a source.
type BackfillTrigger interface {
	TriggerBackfill(sourceID string)
}

var adminSourcesTmpl = template.Must(template.ParseFS(templateFS, "templates/admin_sources.html"))

type adminRunRow struct {
	StartedAt  string
	Status     string
	Discovered string
	Parsed     string
	Errors     string
}

type adminSourcePanel struct {
	ID                             string
	DisplayName                    string
	BaseURL                        string
	Enabled                        bool
	RateLimitMS                    int
	Concurrency                    int
	AbsenceDaysBeforeStale         int
	AbsenceDaysBeforeInactive      int
	ConsecutiveMissedRunsThreshold int
	MinResultRatioForInactivation  string
	LastRunAt                      string
	RecentRuns                     []adminRunRow
	EligibleCount                  int
	EligibleAvailable              bool
}

type adminSourcesPageData struct {
	Sources []adminSourcePanel
	Flash   string
}

// AdminSourcesHandler renders GET /admin/sources.
func AdminSourcesHandler(asq AdminSourcesQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		flash := r.URL.Query().Get("flash")

		var panels []adminSourcePanel
		if asq != nil {
			srcs, err := asq.QuerySources(ctx)
			if err == nil {
				for _, src := range srcs {
					runs, _ := asq.QueryRecentRuns(ctx, src.ID, 5)
					eligible, eligErr := asq.CountBackfillEligible(ctx, src.ID)
					panel := buildAdminSourcePanel(src, runs)
					panel.EligibleAvailable = eligErr == nil
					panel.EligibleCount = eligible
					panels = append(panels, panel)
				}
			}
		}

		data := adminSourcesPageData{Sources: panels, Flash: flash}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		adminSourcesTmpl.Execute(w, data)
	}
}

// AdminSourcesUpdateHandler processes POST /admin/sources/{id} to update thresholds.
func AdminSourcesUpdateHandler(asu AdminSourcesUpdater) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if asu == nil {
			http.Redirect(w, r, "/admin/sources?flash=no_db", http.StatusSeeOther)
			return
		}

		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad form", http.StatusBadRequest)
			return
		}

		id := chi.URLParam(r, "id")
		ctx := r.Context()

		src, err := asu.QuerySourceByID(ctx, id)
		if err != nil {
			http.Error(w, "source not found", http.StatusNotFound)
			return
		}

		if v, err := strconv.Atoi(r.FormValue("absence_days_stale")); err == nil {
			src.AbsenceDaysBeforeStale = v
		}
		if v, err := strconv.Atoi(r.FormValue("absence_days_inactive")); err == nil {
			src.AbsenceDaysBeforeInactive = v
		}
		if v, err := strconv.Atoi(r.FormValue("consecutive_missed")); err == nil {
			src.ConsecutiveMissedRunsThreshold = v
		}
		if v, err := strconv.ParseFloat(r.FormValue("min_ratio"), 64); err == nil {
			src.MinResultRatioForInactivation = v
		}
		if v, err := strconv.Atoi(r.FormValue("rate_limit_ms")); err == nil {
			src.RateLimitMS = v
		}
		if v, err := strconv.Atoi(r.FormValue("concurrency")); err == nil {
			src.Concurrency = v
		}
		src.Enabled = r.FormValue("enabled") == "true"

		if err := asu.UpdateSource(ctx, src); err != nil {
			http.Error(w, "update failed", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin/sources?flash=saved", http.StatusSeeOther)
	}
}

// AdminSourcesBackfillHandler processes POST /admin/sources/{id}/backfill.
func AdminSourcesBackfillHandler(bt BackfillTrigger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		if bt != nil {
			bt.TriggerBackfill(id)
		}
		http.Redirect(w, r, "/admin/sources?flash=backfill_started", http.StatusSeeOther)
	}
}

func buildAdminSourcePanel(src source.Source, runs []source.ScrapeRun) adminSourcePanel {
	panel := adminSourcePanel{
		ID:                             src.ID,
		DisplayName:                    src.DisplayName,
		BaseURL:                        src.BaseURL,
		Enabled:                        src.Enabled,
		RateLimitMS:                    src.RateLimitMS,
		Concurrency:                    src.Concurrency,
		AbsenceDaysBeforeStale:         src.AbsenceDaysBeforeStale,
		AbsenceDaysBeforeInactive:      src.AbsenceDaysBeforeInactive,
		ConsecutiveMissedRunsThreshold: src.ConsecutiveMissedRunsThreshold,
		MinResultRatioForInactivation:  fmt.Sprintf("%.3f", src.MinResultRatioForInactivation),
		LastRunAt:                      "never",
	}
	if src.LastRunAt != nil {
		panel.LastRunAt = timeAgo(*src.LastRunAt)
	}
	for _, run := range runs {
		row := adminRunRow{
			StartedAt: run.StartedAt.Format("2006-01-02 15:04"),
			Status:    string(run.Status),
		}
		if run.DiscoveredCount != nil {
			row.Discovered = strconv.Itoa(*run.DiscoveredCount)
		} else {
			row.Discovered = "—"
		}
		if run.ParsedCount != nil {
			row.Parsed = strconv.Itoa(*run.ParsedCount)
		} else {
			row.Parsed = "—"
		}
		if run.ErrorCount != nil {
			row.Errors = strconv.Itoa(*run.ErrorCount)
		} else {
			row.Errors = "—"
		}
		panel.RecentRuns = append(panel.RecentRuns, row)
	}
	return panel
}
