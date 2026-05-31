package web

import (
	"context"
	"html/template"
	"net/http"
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
}

var listingsTmpl = template.Must(template.ParseFS(templateFS, "templates/listings.html"))
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
	CapturedAt  string
	Status      string
	Price       string
	Acres       string
	Diff        string
}

// ListingsHandler serves the paginated listings index.
func ListingsHandler(q ListingsQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if q == nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		limit := 50
		offset := 0

		if v := r.URL.Query().Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				if n > 200 {
					n = 200
				}
				limit = n
			}
		}
		if v := r.URL.Query().Get("offset"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				offset = n
			}
		}

		listings, err := q.QueryListings(r.Context(), limit, offset)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		rows := make([]listingRow, len(listings))
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
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		listingsTmpl.Execute(w, map[string]any{
			"Rows": rows,
		})
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

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		listingDetailTmpl.Execute(w, map[string]any{
			"ID":      l.ID,
			"Title":   title,
			"URL":     l.URL,
			"Status":  string(l.Status),
			"Price":   price,
			"Acres":   acres,
			"Address": strings.Join(addrParts, ", "),
			"Snaps":   srows,
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
