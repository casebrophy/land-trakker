package listingbus

import (
	"context"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/cbrophy/land_trakker/business/domain/listing"
)

// DedupConfig holds thresholds for duplicate detection.
type DedupConfig struct {
	GeoMaxKM       float64
	AcresMaxDelta  float64
	PriceMaxDelta  float64
	ScoreThreshold float64
}

// DefaultDedupConfig returns the default dedup configuration.
func DefaultDedupConfig() DedupConfig {
	return DedupConfig{
		GeoMaxKM:       10.0,
		AcresMaxDelta:  0.20,
		PriceMaxDelta:  0.20,
		ScoreThreshold: 0.40,
	}
}

// ScorePair computes a similarity score (0.0–1.0) and the matching reason
// strings for two listings.
func ScorePair(a, b listing.Listing, cfg DedupConfig) (float64, []string) {
	var reasons []string

	// geo
	if a.Geom != nil && b.Geom != nil {
		if haversineKM(*a.Geom, *b.Geom) <= cfg.GeoMaxKM {
			reasons = append(reasons, listing.DedupReasonGeo)
		}
	}

	// acres
	if a.Acres != nil && b.Acres != nil {
		aA, bA := *a.Acres, *b.Acres
		if aA != 0 && math.Abs(aA-bA)/aA <= cfg.AcresMaxDelta {
			reasons = append(reasons, listing.DedupReasonAcres)
		}
	}

	// price
	if a.PriceCents != nil && b.PriceCents != nil {
		aP, bP := float64(*a.PriceCents), float64(*b.PriceCents)
		if aP != 0 && math.Abs(aP-bP)/aP <= cfg.PriceMaxDelta {
			reasons = append(reasons, listing.DedupReasonPrice)
		}
	}

	// broker
	if a.BrokerName != nil && b.BrokerName != nil {
		if brokerSimilar(*a.BrokerName, *b.BrokerName) {
			reasons = append(reasons, listing.DedupReasonBroker)
		}
	}

	// title
	if a.Title != nil && b.Title != nil {
		if titleJaccard(*a.Title, *b.Title) >= 0.30 {
			reasons = append(reasons, listing.DedupReasonTitle)
		}
	}

	score := float64(len(reasons)) / 5.0
	return score, reasons
}

// RunDedup fetches all active/stale listings, groups them by county, and
// upserts any pairs whose score meets the configured threshold.
func (c *Core) RunDedup(ctx context.Context, cfg DedupConfig, now time.Time) error {
	listings, err := c.storer.QueryListingsForDedup(ctx)
	if err != nil {
		return err
	}

	// group by county (empty string when nil)
	byCounty := make(map[string][]listing.Listing)
	for _, l := range listings {
		county := ""
		if l.County != nil {
			county = *l.County
		}
		byCounty[county] = append(byCounty[county], l)
	}

	for _, group := range byCounty {
		for i := 0; i < len(group); i++ {
			for j := i + 1; j < len(group); j++ {
				a, b := group[i], group[j]
				if a.SourceID == b.SourceID {
					continue
				}

				score, reasons := ScorePair(a, b, cfg)
				if score < cfg.ScoreThreshold {
					continue
				}

				// enforce canonical ordering: a.ID < b.ID
				if a.ID > b.ID {
					a, b = b, a
				}

				pd := listing.PossibleDuplicate{
					ListingAID: a.ID,
					ListingBID: b.ID,
					Score:      score,
					Reasons:    reasons,
					DetectedAt: now,
				}
				if uErr := c.storer.UpsertPossibleDuplicate(ctx, pd); uErr != nil {
					c.log.Warn("dedup upsert failed", "a_id", pd.ListingAID, "b_id", pd.ListingBID, "err", uErr)
				}
			}
		}
	}

	return nil
}

// haversineKM computes the great-circle distance in kilometres between two
// WGS-84 points using the haversine formula.
func haversineKM(a, b listing.Point) float64 {
	const R = 6371.0 // Earth radius in km
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := (b.Lat - a.Lat) * math.Pi / 180
	dLng := (b.Lng - a.Lng) * math.Pi / 180

	sinLat := math.Sin(dLat / 2)
	sinLng := math.Sin(dLng / 2)
	h := sinLat*sinLat + math.Cos(lat1)*math.Cos(lat2)*sinLng*sinLng
	return 2 * R * math.Asin(math.Sqrt(h))
}

// normalizeAlpha lowercases s and strips all non-alphabetic characters.
func normalizeAlpha(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// brokerSimilar returns true when the two broker name strings are sufficiently
// similar: either one contains the other (after normalisation) or their token
// intersection / union is ≥ 0.5.
func brokerSimilar(a, b string) bool {
	na, nb := normalizeAlpha(a), normalizeAlpha(b)
	if na == "" || nb == "" {
		return false
	}
	if strings.Contains(na, nb) || strings.Contains(nb, na) {
		return true
	}
	// token overlap
	ta := tokenSet(strings.ToLower(a))
	tb := tokenSet(strings.ToLower(b))
	inter := 0
	for k := range ta {
		if _, ok := tb[k]; ok {
			inter++
		}
	}
	union := len(ta) + len(tb) - inter
	if union == 0 {
		return false
	}
	return float64(inter)/float64(union) >= 0.5
}

// titleJaccard computes the Jaccard similarity of the token sets of two title
// strings.
func titleJaccard(a, b string) float64 {
	ta := tokenSet(strings.ToLower(a))
	tb := tokenSet(strings.ToLower(b))
	if len(ta) == 0 && len(tb) == 0 {
		return 0
	}
	inter := 0
	for k := range ta {
		if _, ok := tb[k]; ok {
			inter++
		}
	}
	union := len(ta) + len(tb) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// tokenSet splits a lowercased string on whitespace and returns a set.
func tokenSet(s string) map[string]struct{} {
	tokens := strings.Fields(s)
	m := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		m[t] = struct{}{}
	}
	return m
}
