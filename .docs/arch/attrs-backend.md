# Attrs Foundation System

Deterministic regex/keyword extractors for 8 land-listing attributes. Each extractor returns a Result with matched value, confidence score (0–1), and the evidence snippet that triggered the match. The package is consumed by scraper normalization pipelines and web handlers during listing ingestion and deduplication.

## Core Types

```go
// Result holds the outcome of a single attribute extraction.
type Result struct {
	Value      string  // "true"/"false" for booleans; enum value for text attrs
	Confidence float64 // 0..1
	Evidence   string  // the matched snippet
}
```

## Extractors

### ExtractWaterFrontage(text string) (Result, bool)
**Input:** Raw listing description  
**Output:** Result{Value: "true"/"false", Confidence: 0.8–0.9, Evidence: matched phrase}, ok bool

**Extraction logic:**
- Negation checked first: regex `no\s+(creek|river|stream|pond|lake|water\s*front|riparian)|without\s+(creek|river|stream|water\s*front)` → "false", 0.8
- Positive match: regex `creek|river|stream|pond|lake\s*(front(age)?)?|riparian|water\s*front(age)?` → "true", 0.9
- Case-insensitive (input lowercased once)
- Returns `(Result{}, false)` if no match found

---

### ExtractOffGrid(text string) (Result, bool)
**Input:** Raw listing description  
**Output:** Result{Value: "true"/"false", Confidence: 0.8–0.9, Evidence: matched phrase}, ok bool

**Extraction logic:**
- Positive checked first: regex `off[- ]?grid|no\s+power|no\s+utilities|off\s+the\s+grid` → "true", 0.9
- Negation: regex `power\s+(available|to\s+site|connected)|electricity\s+(available|connected)|on[- ]?grid` → "false", 0.8
- Positive prioritized to avoid shadowing by generic power language
- Case-insensitive

---

### ExtractRoadAccess(text string) (Result, bool)
**Input:** Raw listing description  
**Output:** Result{Value: "paved"/"gravel"/"dirt"/"seasonal"/"none", Confidence: 0.80–0.95, Evidence: matched phrase}, ok bool

**Extraction logic:**
- Confidence cascade (highest confidence wins on tie-break):
  - Paved: `paved\s*road|asphalt\s*road|county\s*road|state\s*road|highway\s*frontage` → 0.95
  - None: `landlocked|no\s*road\s*access|no\s*legal\s*access` → 0.90
  - Gravel: `gravel\s*road|gravel\s*access|gravel\s*drive` → 0.90
  - Dirt: `dirt\s*road|unimproved\s*road|two[- ]?track` → 0.85
  - Seasonal: `seasonal\s*(road|access)|forest\s*service\s*road` → 0.80
- First match with highest confidence is returned
- Returns `(Result{}, false)` if no road types detected

---

### ExtractPower(text string) (Result, bool)
**Input:** Raw listing description  
**Output:** Result{Value: "true"/"false", Confidence: 0.9, Evidence: matched phrase}, ok bool

**Extraction logic:**
- Negation checked first: regex `no\s+power|no\s+electricity|off[- ]?grid` → "false", 0.9
- Positive: regex `power\s*(available|to\s+site|connected|on\s+site)|electricity\s+(available|connected|on\s+site)|electric\s+(service|available|hookup)` → "true", 0.9
- Negation prioritized because "no power available" contains both signals and negative is more specific
- Case-insensitive

---

### ExtractWell(text string) (Result, bool)
**Input:** Raw listing description  
**Output:** Result{Value: "true"/"false", Confidence: 0.85–0.9, Evidence: matched phrase}, ok bool

**Extraction logic:**
- Negation checked first: regex `no\s+well|without\s+(a\s+)?well|no\s+domestic\s+water` → "false", 0.85
- Positive: regex `\bwell\b|domestic\s+water\s+well|drilled\s+well|artesian\s+well|water\s+well` → "true", 0.9
- Word boundary on `\bwell\b` prevents false matches in "dweller", "swell", etc.
- Case-insensitive

---

### ExtractSeptic(text string) (Result, bool)
**Input:** Raw listing description  
**Output:** Result{Value: "true"/"false", Confidence: 0.85–0.9, Evidence: matched phrase}, ok bool

**Extraction logic:**
- Negation checked first: regex `no\s+septic|without\s+septic|no\s+sewer` → "false", 0.85
- Positive: regex `\bseptic\b|mound\s+system|leach\s+field|public\s+sewer` → "true", 0.9
- Word boundary on `\bseptic\b` prevents false matches
- Case-insensitive

---

### ExtractPropertyType(text string) (Result, bool)
**Input:** Raw listing description  
**Output:** Result{Value: "timber"/"agricultural"/"hunting"/"homesite"/"commercial", Confidence: 0.80–0.9, Evidence: first matched phrase}, ok bool

**Extraction logic:**
- Match counting + tie-break priority (order in categories list):
  1. Timber: `\btimber\b|\bforest\b|\bwoodland\b|\btree\s+farm\b` → 0.9
  2. Agricultural: `\bfarm(land|ing)?\b|\bag(ricultural)?\s+(land|ground)\b|\bhay\b|\bcrop\b|\bpasture\b|\birrigated\b|\branch\b` → 0.9
  3. Hunting: `\bhunting\b|\brecreational\b|\bwildlife\b|\bdeer\b|\belk\b|\bwaterfowl\b` → 0.85
  4. Homesite: `\blo[ts]\b|\bhome\s*site\b|\bresidential\b|\bbuild\s+site\b|\bcabin\s+(site|lot)\b` → 0.85
  5. Commercial: `\bcommercial\b` → 0.80
- Category with highest match count wins
- On tie, category appearing first in order wins
- Evidence is the first matched phrase of the winning category
- Returns `(Result{}, false)` if no property type keywords found

---

### ExtractAll(text string) map[string]Result
**Input:** Raw listing description  
**Output:** Map of extracted attributes to Results, empty map if no matches

**Logic:**
- Calls all 7 extractors sequentially
- Keys matched results into map: "water_frontage", "off_grid", "road_access", "power", "well", "septic", "property_type"
- Only includes attributes where extraction succeeded (ok==true)
- Returns empty map for empty or generic text

---

## File Map

- `foundation/attrs/attrs.go` — Core extractors (ExtractWaterFrontage, ExtractOffGrid, ExtractRoadAccess, ExtractPower, ExtractWell, ExtractSeptic, ExtractPropertyType, ExtractAll) + compiled regexes + Result type
- `foundation/attrs/doc.go` — Package documentation
- `foundation/attrs/attrs_test.go` — Unit tests covering positive/negative/no-match cases for each extractor

## Impact Callouts

### ⚠ Regex Patterns (foundation/attrs/attrs.go:19–53)
Changing any compiled regex will affect:
- **reWaterNeg/reWaterPos** — waterfrontage extraction tests `attrs_test.go:13–73`; affects scraper normalization of water-adjacent listings
- **reOffGridPos/reOffGridNeg** — off-grid extraction tests `attrs_test.go:80–140`; affects property utility classification
- **reRoadPaved/reRoadGravel/reRoadDirt/reRoadSeasonal/reRoadNone** — road-access cascade tests `attrs_test.go:147–237`; confidence ordering at lines 102–107 encodes priority (paved > gravel/none > dirt > seasonal)
- **rePowerPos/rePowerNeg** — power extraction tests `attrs_test.go:244–304`; negation-first check at line 129 is critical (avoids "no power available" false positives)
- **reWellNeg/reWellPos** — well extraction tests `attrs_test.go:311–371`; word boundary `\bwell\b` at line 41 prevents false matches
- **reSepticNeg/reSepticPos** — septic extraction tests `attrs_test.go:378–438`; word boundary `\bseptic\b` at line 45 prevents false matches
- **rePropTimber/rePropAg/rePropHunting/rePropHome/rePropComm** — property-type cascade tests `attrs_test.go:445–530`; match counting at line 192 and tie-break priority at line 203 determine winner

### ⚠ ExtractRoadAccess Confidence Cascade (foundation/attrs/attrs.go:101–107)
Changing the confidence ordering or adding/removing road types affects:
- Road-access tie-breaking behavior (`attrs_test.go:229–237` verifies evidence is non-empty)
- Scraper logic that depends on paved > gravel/none hierarchy
- Any downstream filtering on confidence thresholds

### ⚠ ExtractPropertyType Match Counting & Tie-Break (foundation/attrs/attrs.go:191–207)
Changing the category order or match logic affects:
- Multi-signal detection (`attrs_test.go:514–523` validates highest count wins)
- Tie-break priority — first category in the list wins on equal match counts
- Listing classification in scraper normalization pipelines

### ⚠ Negation-First Check in ExtractOffGrid & ExtractPower (foundation/attrs/attrs.go:83, 129)
These extractors check negation *before* positive because:
- "no power available" matches both reOffGridNeg and reOffGridPos (negation is more specific)
- Same risk in ExtractPower: "no power" must be caught before generic "power" patterns
- Reversing the check order will silently flip results on ambiguous text
- Tests `attrs_test.go:106–127, 270–291` validate this ordering

### ⚠ Result Type (foundation/attrs/attrs.go:9–13)
The Result struct is the public API boundary:
- Evidence field must be non-empty when ok==true (tests enforce this)
- Confidence range 0–1 is soft contract (no validation, but tests assume 0.8–0.95 range)
- Value field semantics vary by extractor (boolean extractors use "true"/"false"; road_access/property_type use enum strings)
- Changing field names or types breaks all callers

### ⚠ ExtractAll Key Names (foundation/attrs/attrs.go:216–241)
The map keys must match:
- "water_frontage", "off_grid", "road_access", "power", "well", "septic", "property_type"
- Changing key names breaks downstream code expecting these exact strings
- Callers depend on the order being deterministic (though Go maps are unordered, the key names are stable)

## Consumer Integration

These functions are called by:
- **Scraper orchestrator** — normalizes listing text during property record ingestion
- **Web handlers** — deduplication review queue for manual confirmation of extracted attributes
- **Parser foundation** — chains with ParseAcres/ParsePrice/NormalizeAddress for full listing normalization
- **Property record schemas** — Result{value, confidence, evidence} tuple flows into attribute tables
