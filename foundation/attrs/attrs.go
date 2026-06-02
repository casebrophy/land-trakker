package attrs

import (
	"regexp"
	"strings"
)

// Result holds the outcome of a single attribute extraction.
type Result struct {
	Value      string  // "true"/"false" for booleans; enum value for text attrs
	Confidence float64 // 0..1
	Evidence   string  // the matched snippet
}

// --------------------------------------------------------------------------
// Package-level compiled regexps (no (?i) — we lower-case input once).
// --------------------------------------------------------------------------

var (
	// WaterFrontage
	reWaterNeg = regexp.MustCompile(`no\s+(creek|river|stream|pond|lake|water\s*front|riparian)|without\s+(creek|river|stream|water\s*front)`)
	reWaterPos = regexp.MustCompile(`creek|river|stream|pond|lake\s*(front(age)?)?|riparian|water\s*front(age)?`)

	// OffGrid
	reOffGridPos = regexp.MustCompile(`off[- ]?grid|no\s+power|no\s+utilities|off\s+the\s+grid`)
	reOffGridNeg = regexp.MustCompile(`power\s+(available|to\s+site|connected)|electricity\s+(available|connected)|on[- ]?grid`)

	// RoadAccess
	reRoadPaved    = regexp.MustCompile(`paved\s*road|asphalt\s*road|county\s*road|state\s*road|highway\s*frontage`)
	reRoadGravel   = regexp.MustCompile(`gravel\s*road|gravel\s*access|gravel\s*drive`)
	reRoadDirt     = regexp.MustCompile(`dirt\s*road|unimproved\s*road|two[- ]?track`)
	reRoadSeasonal = regexp.MustCompile(`seasonal\s*(road|access)|forest\s*service\s*road`)
	reRoadNone     = regexp.MustCompile(`landlocked|no\s*road\s*access|no\s*legal\s*access`)

	// Power
	rePowerPos = regexp.MustCompile(`power\s*(available|to\s+site|connected|on\s+site)|electricity\s+(available|connected|on\s+site)|electric\s+(service|available|hookup)`)
	rePowerNeg = regexp.MustCompile(`no\s+power|no\s+electricity|off[- ]?grid`)

	// Well
	reWellNeg = regexp.MustCompile(`no\s+well|without\s+(a\s+)?well|no\s+domestic\s+water`)
	reWellPos = regexp.MustCompile(`\bwell\b|domestic\s+water\s+well|drilled\s+well|artesian\s+well|water\s+well`)

	// Septic
	reSepticNeg = regexp.MustCompile(`no\s+septic|without\s+septic|no\s+sewer`)
	reSepticPos = regexp.MustCompile(`\bseptic\b|mound\s+system|leach\s+field|public\s+sewer`)

	// PropertyType
	rePropTimber  = regexp.MustCompile(`\btimber\b|\bforest\b|\bwoodland\b|\btree\s+farm\b`)
	rePropAg      = regexp.MustCompile(`\bfarm(land|ing)?\b|\bag(ricultural)?\s+(land|ground)\b|\bhay\b|\bcrop\b|\bpasture\b|\birrigated\b|\branch\b`)
	rePropHunting = regexp.MustCompile(`\bhunting\b|\brecreational\b|\bwildlife\b|\bdeer\b|\belk\b|\bwaterfowl\b`)
	rePropHome    = regexp.MustCompile(`\blo[ts]\b|\bhome\s*site\b|\bresidential\b|\bbuild\s+site\b|\bcabin\s+(site|lot)\b`)
	rePropComm    = regexp.MustCompile(`\bcommercial\b`)
)

// --------------------------------------------------------------------------
// Boolean helpers
// --------------------------------------------------------------------------

func firstMatch(re *regexp.Regexp, text string) string {
	m := re.FindString(text)
	return m
}

// --------------------------------------------------------------------------
// Extractors
// --------------------------------------------------------------------------

// ExtractWaterFrontage detects water-frontage mentions. Negation is checked first.
func ExtractWaterFrontage(text string) (Result, bool) {
	low := strings.ToLower(text)
	if m := firstMatch(reWaterNeg, low); m != "" {
		return Result{Value: "false", Confidence: 0.8, Evidence: m}, true
	}
	if m := firstMatch(reWaterPos, low); m != "" {
		return Result{Value: "true", Confidence: 0.9, Evidence: m}, true
	}
	return Result{}, false
}

// ExtractOffGrid detects off-grid / no-utilities mentions. Positive checked first.
func ExtractOffGrid(text string) (Result, bool) {
	low := strings.ToLower(text)
	if m := firstMatch(reOffGridPos, low); m != "" {
		return Result{Value: "true", Confidence: 0.9, Evidence: m}, true
	}
	if m := firstMatch(reOffGridNeg, low); m != "" {
		return Result{Value: "false", Confidence: 0.8, Evidence: m}, true
	}
	return Result{}, false
}

// ExtractRoadAccess detects road-access type using a confidence cascade.
func ExtractRoadAccess(text string) (Result, bool) {
	low := strings.ToLower(text)

	type candidate struct {
		re         *regexp.Regexp
		value      string
		confidence float64
	}
	candidates := []candidate{
		{reRoadPaved, "paved", 0.95},
		{reRoadNone, "none", 0.90},
		{reRoadGravel, "gravel", 0.90},
		{reRoadDirt, "dirt", 0.85},
		{reRoadSeasonal, "seasonal", 0.80},
	}

	var best Result
	found := false
	for _, c := range candidates {
		m := firstMatch(c.re, low)
		if m == "" {
			continue
		}
		if !found || c.confidence > best.Confidence {
			best = Result{Value: c.value, Confidence: c.confidence, Evidence: m}
			found = true
		}
	}
	return best, found
}

// ExtractPower detects whether power/electricity is available.
// Negative is checked first because "no power available" contains both signals
// and the negative is more specific; positive is returned only when negation is absent.
func ExtractPower(text string) (Result, bool) {
	low := strings.ToLower(text)
	if m := firstMatch(rePowerNeg, low); m != "" {
		return Result{Value: "false", Confidence: 0.9, Evidence: m}, true
	}
	if m := firstMatch(rePowerPos, low); m != "" {
		return Result{Value: "true", Confidence: 0.9, Evidence: m}, true
	}
	return Result{}, false
}

// ExtractWell detects well mentions. Negation checked first.
func ExtractWell(text string) (Result, bool) {
	low := strings.ToLower(text)
	if m := firstMatch(reWellNeg, low); m != "" {
		return Result{Value: "false", Confidence: 0.85, Evidence: m}, true
	}
	if m := firstMatch(reWellPos, low); m != "" {
		return Result{Value: "true", Confidence: 0.9, Evidence: m}, true
	}
	return Result{}, false
}

// ExtractSeptic detects septic/sewer mentions. Negation checked first.
func ExtractSeptic(text string) (Result, bool) {
	low := strings.ToLower(text)
	if m := firstMatch(reSepticNeg, low); m != "" {
		return Result{Value: "false", Confidence: 0.85, Evidence: m}, true
	}
	if m := firstMatch(reSepticPos, low); m != "" {
		return Result{Value: "true", Confidence: 0.9, Evidence: m}, true
	}
	return Result{}, false
}

// ExtractPropertyType determines the dominant property type via match counting.
func ExtractPropertyType(text string) (Result, bool) {
	low := strings.ToLower(text)

	type category struct {
		re         *regexp.Regexp
		value      string
		confidence float64
	}
	// Order encodes tie-break priority (first wins on tie).
	categories := []category{
		{rePropTimber, "timber", 0.9},
		{rePropAg, "agricultural", 0.9},
		{rePropHunting, "hunting", 0.85},
		{rePropHome, "homesite", 0.85},
		{rePropComm, "commercial", 0.80},
	}

	type scored struct {
		count      int
		value      string
		confidence float64
		evidence   string
		priority   int
	}

	var best scored
	found := false

	for i, c := range categories {
		matches := c.re.FindAllString(low, -1)
		if len(matches) == 0 {
			continue
		}
		s := scored{
			count:      len(matches),
			value:      c.value,
			confidence: c.confidence,
			evidence:   matches[0],
			priority:   i,
		}
		if !found || s.count > best.count || (s.count == best.count && s.priority < best.priority) {
			best = s
			found = true
		}
	}

	if !found {
		return Result{}, false
	}
	return Result{Value: best.value, Confidence: best.confidence, Evidence: best.evidence}, true
}

// ExtractAll runs all 7 extractors and returns matched results keyed by attribute name.
func ExtractAll(text string) map[string]Result {
	out := make(map[string]Result)

	if r, ok := ExtractWaterFrontage(text); ok {
		out["water_frontage"] = r
	}
	if r, ok := ExtractOffGrid(text); ok {
		out["off_grid"] = r
	}
	if r, ok := ExtractRoadAccess(text); ok {
		out["road_access"] = r
	}
	if r, ok := ExtractPower(text); ok {
		out["power"] = r
	}
	if r, ok := ExtractWell(text); ok {
		out["well"] = r
	}
	if r, ok := ExtractSeptic(text); ok {
		out["septic"] = r
	}
	if r, ok := ExtractPropertyType(text); ok {
		out["property_type"] = r
	}
	return out
}
