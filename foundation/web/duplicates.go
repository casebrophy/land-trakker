package web

import (
	"context"
	"html/template"
	"net/http"
	"strconv"

	"github.com/cbrophy/land_trakker/business/domain/listing"
)

// DuplicatesQuerier is the minimal interface the duplicates handlers require.
type DuplicatesQuerier interface {
	QueryPossibleDuplicates(ctx context.Context, decision *string) ([]listing.PossibleDuplicate, error)
	UpdateDuplicateDecision(ctx context.Context, aID, bID string, decision string) error
	QueryListingByID(ctx context.Context, id string) (listing.Listing, error)
}

var duplicatesTmpl = template.Must(template.ParseFS(templateFS, "templates/duplicates.html"))

type duplicatePairRow struct {
	ListingAID    string
	ListingATitle string
	ListingAPrice string
	ListingAURL   string
	ListingAAddr  string

	ListingBID    string
	ListingBTitle string
	ListingBPrice string
	ListingBURL   string
	ListingBAddr  string

	ScorePercent string
	Reasons      string
}

// DuplicatesHandler shows the review queue of possible duplicates.
func DuplicatesHandler(dq DuplicatesQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if dq == nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}

		// Query for undecided duplicates only (decision is null)
		var decisionFilter *string
		possibleDups, err := dq.QueryPossibleDuplicates(r.Context(), decisionFilter)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		// Build rows, enriching with listing details
		rows := make([]duplicatePairRow, 0, len(possibleDups))
		for _, pd := range possibleDups {
			row := duplicatePairRow{
				ListingAID:   pd.ListingAID,
				ListingBID:   pd.ListingBID,
				ScorePercent: strconv.FormatFloat(pd.Score*100, 'f', 0, 64),
				Reasons:      formatReasons(pd.Reasons),
			}

			// Fetch listing A details
			la, err := dq.QueryListingByID(r.Context(), pd.ListingAID)
			if err == nil {
				if la.Title != nil {
					row.ListingATitle = *la.Title
				}
				if la.PriceCents != nil {
					row.ListingAPrice = formatCents(*la.PriceCents)
				}
				row.ListingAURL = la.URL
				row.ListingAAddr = formatAddress(&la)
			}

			// Fetch listing B details
			lb, err := dq.QueryListingByID(r.Context(), pd.ListingBID)
			if err == nil {
				if lb.Title != nil {
					row.ListingBTitle = *lb.Title
				}
				if lb.PriceCents != nil {
					row.ListingBPrice = formatCents(*lb.PriceCents)
				}
				row.ListingBURL = lb.URL
				row.ListingBAddr = formatAddress(&lb)
			}

			rows = append(rows, row)
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		duplicatesTmpl.Execute(w, map[string]any{
			"Pairs":     rows,
			"PairCount": len(rows),
		})
	}
}

// DuplicatesUpdateHandler handles form submissions to record a decision on a pair.
func DuplicatesUpdateHandler(dq DuplicatesQuerier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if dq == nil {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Expected form data:
		// action=<same|different|dismiss>
		// a_id=<uuid>
		// b_id=<uuid>
		action := r.FormValue("action")
		aID := r.FormValue("a_id")
		bID := r.FormValue("b_id")

		if aID == "" || bID == "" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Map action to decision value
		decision := ""
		switch action {
		case "same":
			decision = "same"
		case "different":
			decision = "different"
		case "dismiss":
			decision = "dismiss"
		default:
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		if err := dq.UpdateDuplicateDecision(r.Context(), aID, bID, decision); err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/duplicates", http.StatusSeeOther)
	}
}

func formatReasons(reasons []string) string {
	if len(reasons) == 0 {
		return ""
	}
	reasonLabels := make([]string, len(reasons))
	for i, r := range reasons {
		switch r {
		case listing.DedupReasonGeo:
			reasonLabels[i] = "Location"
		case listing.DedupReasonAcres:
			reasonLabels[i] = "Acres"
		case listing.DedupReasonPrice:
			reasonLabels[i] = "Price"
		case listing.DedupReasonBroker:
			reasonLabels[i] = "Broker"
		case listing.DedupReasonTitle:
			reasonLabels[i] = "Title"
		default:
			reasonLabels[i] = r
		}
	}
	// Simple join; we'll use a comma-separated list
	result := ""
	for i, label := range reasonLabels {
		if i > 0 {
			result += ", "
		}
		result += label
	}
	return result
}

func formatAddress(l *listing.Listing) string {
	if l == nil {
		return ""
	}
	parts := []string{}
	if l.AddressLine != nil && *l.AddressLine != "" {
		parts = append(parts, *l.AddressLine)
	}
	if l.City != nil && *l.City != "" {
		parts = append(parts, *l.City)
	}
	if l.County != nil && *l.County != "" {
		parts = append(parts, *l.County+" County")
	}
	if len(parts) == 0 {
		return "(address not available)"
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result
}
