package web

import (
	"html/template"
	"net/http"
	"strconv"
	"strings"

	"github.com/cbrophy/land_trakker/business/domain/search"
)

var digestTmpl = template.Must(template.ParseFS(templateFS, "templates/digest.html"))

type digestHitRow struct {
	HitID        int64
	ListingID    string
	ListingTitle string
	ListingPrice string
	ListingAcres string
	ListingURL   string
	SearchName   string
	HitAt        string
	Reason       string
}

func hitReasonLabel(r search.HitReason) string {
	switch r {
	case search.ReasonNew:
		return "New Listing"
	case search.ReasonPriceDrop:
		return "Price Drop"
	case search.ReasonAttributeAdded:
		return "Attribute Added"
	default:
		return string(r)
	}
}

// DigestHandler shows today's unseen search hits.
func DigestHandler(sc SearchCore, lq ListingsQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sc == nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		hits, err := sc.QueryUnseen(r.Context(), 200)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// Build name lookup for saved searches referenced by hits.
		searchNames := map[string]string{}
		for _, h := range hits {
			if _, seen := searchNames[h.SavedSearchID]; seen {
				continue
			}
			ss, err := sc.QuerySavedSearchByID(r.Context(), h.SavedSearchID)
			if err == nil {
				searchNames[h.SavedSearchID] = ss.Name
			} else {
				searchNames[h.SavedSearchID] = h.SavedSearchID
			}
		}

		rows := make([]digestHitRow, 0, len(hits))
		for _, h := range hits {
			row := digestHitRow{
				HitID:      h.ID,
				ListingID:  h.ListingID,
				SearchName: searchNames[h.SavedSearchID],
				HitAt:      h.HitAt.Format("2006-01-02"),
				Reason:     hitReasonLabel(h.Reason),
			}

			if lq != nil {
				l, err := lq.QueryListingByID(r.Context(), h.ListingID)
				if err == nil {
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
					row.ListingTitle = title
					row.ListingPrice = price
					row.ListingAcres = acres
					row.ListingURL = l.URL
				}
			}

			if row.ListingTitle == "" {
				row.ListingTitle = "(untitled)"
			}

			rows = append(rows, row)
		}

		// Build a comma-separated list of hit IDs for the mark-seen form.
		hitIDs := make([]string, len(rows))
		for i, r := range rows {
			hitIDs[i] = strconv.FormatInt(r.HitID, 10)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		digestTmpl.Execute(w, map[string]any{
			"Hits":      rows,
			"HitIDs":    strings.Join(hitIDs, ","),
			"HitCount":  len(rows),
		})
	}
}

// DigestMarkSeenHandler marks hits as seen and redirects back to /digest.
func DigestMarkSeenHandler(sc SearchCore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sc == nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		raw := r.FormValue("hit_ids")
		var ids []int64
		for _, part := range strings.Split(raw, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				continue
			}
			ids = append(ids, id)
		}
		if len(ids) > 0 {
			if err := sc.MarkHitsSeen(r.Context(), ids); err != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
		}
		http.Redirect(w, r, "/digest", http.StatusSeeOther)
	}
}
