package web

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cbrophy/land_trakker/business/domain/listing"
	"github.com/go-chi/chi/v5"
)

// ListingsQuerier is the minimal interface the listing handlers require.
type ListingsQuerier interface {
	QueryListings(ctx context.Context, limit, offset int) ([]listing.Listing, error)
	QueryListingByID(ctx context.Context, id string) (listing.Listing, error)
	QuerySnapshotsByListing(ctx context.Context, listingID string) ([]listing.ListingSnapshot, error)
	QueryListingsFilter(ctx context.Context, f listing.ListingFilter, limit, offset int) ([]listing.Listing, error)
}

var listingsTmpl = template.Must(template.ParseFS(templateFS,
	"templates/listings.html",
	"templates/listings_results.html",
))
var listingsResultsTmpl = template.Must(template.ParseFS(templateFS, "templates/listings_results.html"))
var listingDetailTmpl = template.Must(template.ParseFS(templateFS, "templates/listing_detail.html"))

type listingRow struct {
	ID            string
	Title         string
	Status        string
	PricePerAcre  string
	Acres         string
	Location      string
	FirstSeenDate string
}

type snapRow struct {
	CapturedAt string
	Status     string
	Price      string
	Acres      string
	Diff       string
}

type timelineDataPoint struct {
	Date  string  `json:"date"`
	Price float64 `json:"price"`
	Acres float64 `json:"acres"`
}

type timelineData struct {
	Points []timelineDataPoint `json:"points"`
}

type mapMarker struct {
	Lat   float64 `json:"lat"`
	Lng   float64 `json:"lng"`
	Title string  `json:"title"`
	ID    string  `json:"id"`
}

type filterForm struct {
	AcresMin     string
	AcresMax     string
	PriceMin     string
	PriceMax     string
	Counties     string
	PropertyType string
	Query        string
	AttrWater    bool
	AttrOffGrid  bool
	AttrPower    bool
	AttrWell     bool
	AttrSeptic   bool
}

func parsePagination(r *http.Request) (limit, offset int) {
	limit = 50
	q := r.URL.Query()
	if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 200 {
				n = 200
			}
			limit = n
		}
	}
	if v := q.Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			offset = n
		}
	}
	return
}

func parseFilter(r *http.Request) listing.ListingFilter {
	q := r.URL.Query()
	var f listing.ListingFilter

	if v := q.Get("q"); v != "" {
		f.FullText = &v
	}
	if v := q.Get("acres_min"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			f.AcresMin = &n
		}
	}
	if v := q.Get("acres_max"); v != "" {
		if n, err := strconv.ParseFloat(v, 64); err == nil {
			f.AcresMax = &n
		}
	}
	if v := q.Get("price_min"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cents := n * 100
			f.PriceMin = &cents
		}
	}
	if v := q.Get("price_max"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			cents := n * 100
			f.PriceMax = &cents
		}
	}
	if v := q.Get("counties"); v != "" {
		for _, p := range strings.Split(v, ",") {
			if p = strings.TrimSpace(p); p != "" {
				f.Counties = append(f.Counties, p)
			}
		}
	}
	if v := q.Get("property_type"); v != "" {
		f.PropertyType = &v
	}
	if q.Get("attr_water") == "true" {
		b := true
		f.AttrWaterFrontage = &b
	}
	if q.Get("attr_off_grid") == "true" {
		b := true
		f.AttrOffGrid = &b
	}
	if q.Get("attr_power") == "true" {
		b := true
		f.AttrPower = &b
	}
	if q.Get("attr_well") == "true" {
		b := true
		f.AttrWell = &b
	}
	if q.Get("attr_septic") == "true" {
		b := true
		f.AttrSeptic = &b
	}
	return f
}

func isFilterEmpty(f listing.ListingFilter) bool {
	return f.AcresMin == nil && f.AcresMax == nil &&
		f.PriceMin == nil && f.PriceMax == nil &&
		len(f.Counties) == 0 &&
		f.PPAMin == nil && f.PPAMax == nil &&
		f.PropertyType == nil &&
		f.FullText == nil &&
		f.AttrWaterFrontage == nil &&
		f.AttrOffGrid == nil &&
		f.AttrPower == nil &&
		f.AttrWell == nil &&
		f.AttrSeptic == nil
}

func buildFilterForm(r *http.Request) filterForm {
	q := r.URL.Query()
	return filterForm{
		Query:        q.Get("q"),
		AcresMin:     q.Get("acres_min"),
		AcresMax:     q.Get("acres_max"),
		PriceMin:     q.Get("price_min"),
		PriceMax:     q.Get("price_max"),
		Counties:     q.Get("counties"),
		PropertyType: q.Get("property_type"),
		AttrWater:    q.Get("attr_water") == "true",
		AttrOffGrid:  q.Get("attr_off_grid") == "true",
		AttrPower:    q.Get("attr_power") == "true",
		AttrWell:     q.Get("attr_well") == "true",
		AttrSeptic:   q.Get("attr_septic") == "true",
	}
}

func buildRowsAndMarkers(listings []listing.Listing) ([]listingRow, []mapMarker) {
	rows := make([]listingRow, len(listings))
	var markers []mapMarker
	for i, l := range listings {
		title := "(untitled)"
		if l.Title != nil {
			title = *l.Title
		}
		ppa := "n/a"
		if l.PricePerAcreCents != nil {
			ppa = formatCents(*l.PricePerAcreCents) + "/acre"
		}
		acres := "n/a"
		if l.Acres != nil {
			acres = strconv.FormatFloat(*l.Acres, 'f', 2, 64)
		}
		var loc []string
		if l.City != nil && *l.City != "" {
			loc = append(loc, *l.City)
		}
		if l.State != nil && *l.State != "" {
			loc = append(loc, *l.State)
		}
		rows[i] = listingRow{
			ID:            l.ID,
			Title:         title,
			Status:        string(l.Status),
			PricePerAcre:  ppa,
			Acres:         acres,
			Location:      strings.Join(loc, ", "),
			FirstSeenDate: l.FirstSeenAt.Format("2006-01-02"),
		}
		if l.Geom != nil {
			markers = append(markers, mapMarker{
				Lat:   l.Geom.Lat,
				Lng:   l.Geom.Lng,
				Title: title,
				ID:    l.ID,
			})
		}
	}
	return rows, markers
}

func paginationURL(r *http.Request, limit, offset int) string {
	q := make(url.Values)
	for k, v := range r.URL.Query() {
		q[k] = v
	}
	q.Set("limit", strconv.Itoa(limit))
	q.Set("offset", strconv.Itoa(offset))
	return "/?" + q.Encode()
}

// ListingsHandler serves the search + map page, with HTMX partial support.
func ListingsHandler(q ListingsQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if q == nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		limit, offset := parsePagination(r)
		f := parseFilter(r)
		isHTMX := r.Header.Get("HX-Request") == "true"

		var listings []listing.Listing
		var err error
		if isFilterEmpty(f) {
			listings, err = q.QueryListings(r.Context(), limit, offset)
		} else {
			listings, err = q.QueryListingsFilter(r.Context(), f, limit, offset)
		}
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		rows, markers := buildRowsAndMarkers(listings)
		markersJSON, _ := json.Marshal(markers)
		if markersJSON == nil {
			markersJSON = []byte("[]")
		}

		hasMore := len(listings) == limit
		var prevURL, nextURL string
		if offset > 0 {
			prev := offset - limit
			if prev < 0 {
				prev = 0
			}
			prevURL = paginationURL(r, limit, prev)
		}
		if hasMore {
			nextURL = paginationURL(r, limit, offset+limit)
		}

		data := map[string]any{
			"Rows":    rows,
			"Markers": template.JS(markersJSON), //nolint:gosec // JSON from trusted internal data
			"Filter":  buildFilterForm(r),
			"Limit":   limit,
			"Offset":  offset,
			"HasMore": hasMore,
			"PrevURL": prevURL,
			"NextURL": nextURL,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if isHTMX {
			listingsResultsTmpl.ExecuteTemplate(w, "results_content", data)
		} else {
			listingsTmpl.Execute(w, data)
		}
	}
}

// ListingDetailHandler serves the detail view for a single listing.
func ListingDetailHandler(q ListingsQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if q == nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		id := chi.URLParam(r, "id")

		l, err := q.QueryListingByID(r.Context(), id)
		if err != nil {
			if strings.Contains(err.Error(), "no rows") {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		snaps, err := q.QuerySnapshotsByListing(r.Context(), id)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		srows := make([]snapRow, len(snaps))
		timeline := timelineData{Points: make([]timelineDataPoint, 0)}
		for i, s := range snaps {
			status := "n/a"
			if s.Status != nil {
				status = *s.Status
			}
			price := "n/a"
			if s.PriceCents != nil {
				price = formatCents(*s.PriceCents)
			}
			acres := "n/a"
			if s.Acres != nil {
				acres = strconv.FormatFloat(*s.Acres, 'f', 2, 64)
			}
			diff := ""
			if len(s.Diff) > 0 {
				var parts []string
				for k := range s.Diff {
					parts = append(parts, k)
				}
				diff = strings.Join(parts, ", ")
			}
			srows[i] = snapRow{
				CapturedAt: s.CapturedAt.Format("2006-01-02 15:04"),
				Status:     status,
				Price:      price,
				Acres:      acres,
				Diff:       diff,
			}

			// Build timeline data point
			if s.PriceCents != nil || s.Acres != nil {
				point := timelineDataPoint{
					Date: s.CapturedAt.Format("2006-01-02"),
				}
				if s.PriceCents != nil {
					point.Price = float64(*s.PriceCents) / 100
				}
				if s.Acres != nil {
					point.Acres = *s.Acres
				}
				timeline.Points = append(timeline.Points, point)
			}
		}

		title := "(untitled)"
		if l.Title != nil {
			title = *l.Title
		}
		price := "n/a"
		if l.PriceCents != nil {
			price = formatCents(*l.PriceCents)
		}
		acres := "n/a"
		if l.Acres != nil {
			acres = strconv.FormatFloat(*l.Acres, 'f', 2, 64)
		}
		var addrParts []string
		if l.AddressLine != nil && *l.AddressLine != "" {
			addrParts = append(addrParts, *l.AddressLine)
		}
		if l.City != nil && *l.City != "" {
			addrParts = append(addrParts, *l.City)
		}
		if l.State != nil && *l.State != "" {
			addrParts = append(addrParts, *l.State)
		}

		timelineJSON, _ := json.Marshal(timeline)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		listingDetailTmpl.Execute(w, map[string]any{
			"ID":       l.ID,
			"Title":    title,
			"URL":      l.URL,
			"Status":   string(l.Status),
			"Price":    price,
			"Acres":    acres,
			"Address":  strings.Join(addrParts, ", "),
			"Snaps":    srows,
			"Timeline": template.JS(timelineJSON), //nolint:gosec // JSON from trusted internal data
		})
	}
}

func formatCents(cents int64) string {
	dollars := cents / 100
	s := strconv.FormatInt(dollars, 10)
	return "$" + addCommas(s)
}

func addCommas(s string) string {
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	rem := len(s) % 3
	if rem > 0 {
		b.WriteString(s[:rem])
	}
	for i := rem; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
