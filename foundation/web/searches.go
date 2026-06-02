package web

import (
	"context"
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/cbrophy/land_trakker/business/domain/search"
	"github.com/go-chi/chi/v5"
)

// formCheckbox returns true if the named checkbox field is present with value "true".
func formCheckbox(r *http.Request, name string) bool {
	for _, v := range r.Form[name] {
		if v == "true" {
			return true
		}
	}
	return false
}

// SearchCore is the minimal interface the searches and digest handlers require.
type SearchCore interface {
	QuerySavedSearches(ctx context.Context) ([]search.SavedSearch, error)
	QuerySavedSearchByID(ctx context.Context, id string) (search.SavedSearch, error)
	CreateSavedSearch(ctx context.Context, ss search.SavedSearch) (search.SavedSearch, error)
	UpdateSavedSearch(ctx context.Context, ss search.SavedSearch) (search.SavedSearch, error)
	DeleteSavedSearch(ctx context.Context, id string) error
	QueryUnseen(ctx context.Context, limit int) ([]search.SearchHit, error)
	MarkHitsSeen(ctx context.Context, ids []int64) error
}

var (
	searchesTmpl   = template.Must(template.ParseFS(templateFS, "templates/searches.html"))
	searchFormTmpl = template.Must(template.ParseFS(templateFS, "templates/search_form.html"))
)

type savedSearchRow struct {
	ID            string
	Name          string
	Enabled       bool
	CreatedAt     string
	FilterSummary string
}

func buildSavedSearchRow(ss search.SavedSearch) savedSearchRow {
	return savedSearchRow{
		ID:            ss.ID,
		Name:          ss.Name,
		Enabled:       ss.Enabled,
		CreatedAt:     ss.CreatedAt.Format("2006-01-02"),
		FilterSummary: filterSummary(ss.Query),
	}
}

func filterSummary(f listing.ListingFilter) string {
	var parts []string
	if f.AcresMin != nil {
		parts = append(parts, "acres≥"+strconv.FormatFloat(*f.AcresMin, 'f', 0, 64))
	}
	if f.AcresMax != nil {
		parts = append(parts, "acres≤"+strconv.FormatFloat(*f.AcresMax, 'f', 0, 64))
	}
	if f.PriceMin != nil {
		parts = append(parts, "price≥"+formatCents(*f.PriceMin))
	}
	if f.PriceMax != nil {
		parts = append(parts, "price≤"+formatCents(*f.PriceMax))
	}
	if len(f.Counties) > 0 {
		parts = append(parts, "counties: "+strings.Join(f.Counties, ", "))
	}
	if f.PropertyType != nil {
		parts = append(parts, "type: "+*f.PropertyType)
	}
	if len(parts) == 0 {
		return "(all listings)"
	}
	return strings.Join(parts, "; ")
}

// filterToFormData converts a ListingFilter back to a filterForm for pre-populating edit forms.
func filterToFormData(f listing.ListingFilter) filterForm {
	var ff filterForm
	if f.AcresMin != nil {
		ff.AcresMin = strconv.FormatFloat(*f.AcresMin, 'f', -1, 64)
	}
	if f.AcresMax != nil {
		ff.AcresMax = strconv.FormatFloat(*f.AcresMax, 'f', -1, 64)
	}
	if f.PriceMin != nil {
		ff.PriceMin = strconv.FormatInt(*f.PriceMin/100, 10)
	}
	if f.PriceMax != nil {
		ff.PriceMax = strconv.FormatInt(*f.PriceMax/100, 10)
	}
	if len(f.Counties) > 0 {
		ff.Counties = strings.Join(f.Counties, ", ")
	}
	if f.PropertyType != nil {
		ff.PropertyType = *f.PropertyType
	}
	if f.AttrWaterFrontage != nil {
		ff.AttrWater = *f.AttrWaterFrontage
	}
	if f.AttrOffGrid != nil {
		ff.AttrOffGrid = *f.AttrOffGrid
	}
	if f.AttrPower != nil {
		ff.AttrPower = *f.AttrPower
	}
	if f.AttrWell != nil {
		ff.AttrWell = *f.AttrWell
	}
	if f.AttrSeptic != nil {
		ff.AttrSeptic = *f.AttrSeptic
	}
	return ff
}

// parseFilterFromForm parses a ListingFilter from POST form values.
func parseFilterFromForm(r *http.Request) listing.ListingFilter {
	get := r.FormValue
	var f listing.ListingFilter

	if v := get("acres_min"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			f.AcresMin = &n
		}
	}
	if v := get("acres_max"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			f.AcresMax = &n
		}
	}
	if v := get("price_min"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cents := n * 100
			f.PriceMin = &cents
		}
	}
	if v := get("price_max"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cents := n * 100
			f.PriceMax = &cents
		}
	}
	if v := get("counties"); v != "" {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				f.Counties = append(f.Counties, p)
			}
		}
	}
	if v := get("property_type"); v != "" {
		f.PropertyType = &v
	}
	if get("attr_water") == "true" {
		b := true
		f.AttrWaterFrontage = &b
	}
	if get("attr_off_grid") == "true" {
		b := true
		f.AttrOffGrid = &b
	}
	if get("attr_power") == "true" {
		b := true
		f.AttrPower = &b
	}
	if get("attr_well") == "true" {
		b := true
		f.AttrWell = &b
	}
	if get("attr_septic") == "true" {
		b := true
		f.AttrSeptic = &b
	}
	return f
}

// SearchesHandler lists all saved searches.
func SearchesHandler(sc SearchCore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sc == nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		searches, err := sc.QuerySavedSearches(r.Context())
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		rows := make([]savedSearchRow, len(searches))
		for i, ss := range searches {
			rows[i] = buildSavedSearchRow(ss)
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		searchesTmpl.Execute(w, map[string]any{"Searches": rows})
	}
}

// SearchesNewHandler renders the create form.
func SearchesNewHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		searchFormTmpl.Execute(w, map[string]any{
			"IsEdit":     false,
			"ActionURL":  "/searches",
			"Name":       "",
			"Enabled":    true,
			"Filter":     filterForm{},
		})
	}
}

// SearchesCreateHandler creates a new saved search.
func SearchesCreateHandler(sc SearchCore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sc == nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "name is required", http.StatusUnprocessableEntity)
			return
		}
		enabled := formCheckbox(r, "enabled")
		q := parseFilterFromForm(r)

		ss := search.SavedSearch{
			Name:    name,
			Query:   q,
			Enabled: enabled,
		}
		if _, err := sc.CreateSavedSearch(r.Context(), ss); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/searches", http.StatusSeeOther)
	}
}

// SearchesEditHandler renders the edit form for an existing saved search.
func SearchesEditHandler(sc SearchCore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sc == nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		id := chi.URLParam(r, "id")
		ss, err := sc.QuerySavedSearchByID(r.Context(), id)
		if err != nil {
			if strings.Contains(err.Error(), "no rows") {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		searchFormTmpl.Execute(w, map[string]any{
			"IsEdit":    true,
			"ActionURL": "/searches/" + ss.ID,
			"ID":        ss.ID,
			"Name":      ss.Name,
			"Enabled":   ss.Enabled,
			"Filter":    filterToFormData(ss.Query),
		})
	}
}

// SearchesUpdateHandler updates an existing saved search.
func SearchesUpdateHandler(sc SearchCore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sc == nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		id := chi.URLParam(r, "id")
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		name := strings.TrimSpace(r.FormValue("name"))
		if name == "" {
			http.Error(w, "name is required", http.StatusUnprocessableEntity)
			return
		}
		enabled := formCheckbox(r, "enabled")
		q := parseFilterFromForm(r)

		ss := search.SavedSearch{
			ID:      id,
			Name:    name,
			Query:   q,
			Enabled: enabled,
		}
		if _, err := sc.UpdateSavedSearch(r.Context(), ss); err != nil {
			if strings.Contains(err.Error(), "no rows") {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/searches", http.StatusSeeOther)
	}
}

// SearchesDeleteHandler deletes a saved search.
func SearchesDeleteHandler(sc SearchCore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sc == nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		id := chi.URLParam(r, "id")
		if err := sc.DeleteSavedSearch(r.Context(), id); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/searches", http.StatusSeeOther)
	}
}
