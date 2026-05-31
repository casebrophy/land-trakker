# Parser Foundation System

Foundation utility layer for normalizing and parsing land property data. Provides regex-based extraction and validation for acreage, pricing, and address fields. Consumed by the scraper orchestrator during property record normalization.

## Core Functions

### ParseAcres(s string) (float64, bool)
**Input:** Raw string from property listing (e.g., "~40 acres", "1,000.5 ac")  
**Output:** Positive float64 acres or false if parsing fails

**Normalization rules:**
- Regex pattern: `(?i)~?±?[\s]*([0-9,]+(?:\.[0-9]+)?)\s*-?(?:acres?|ac\b)`
- Case-insensitive; accepts "acres", "acre", "ac" suffix
- Strips commas from number strings before parsing
- Accepts optional tilde (~) and plus-minus (±) prefixes
- Rejects zero or negative values
- Supports decimals and comma-separated thousands (1,000.5 acres → 1000.5)

**Edge cases covered by tests:**
- Zero value rejection (fails gracefully)
- Very small decimals (0.1 acres)
- Large numbers (10,000,000 acres)
- Mixed case variants

---

### ParsePrice(s string) (int64, bool)
**Input:** Raw string from property listing (e.g., "$1,200,000", "$1.5m")  
**Output:** Price in dollars (int64) or false if parsing fails

**Normalization rules:**
- Regex pattern: `\$\s*([0-9,]+(?:\.[0-9]+)?)\s*([kmKM])?`
- Requires dollar sign prefix
- Strips commas before parsing
- Optional multiplier: "k" (×1000) or "m" (×1000000), case-insensitive
- Converts to int64 after applying multiplier
- Rejects zero or negative values

**Edge cases covered by tests:**
- Zero price rejection (fails gracefully)
- Large millions (10.5m → 10,500,000)
- Multiplier spacing and case variants
- Decimal multipliers (1.5k → 1500)

---

### NormalizeAddress(s string) string
**Input:** Raw address string (e.g., "  123 Main St., Boise, ID 83701  ")  
**Output:** Normalized address string

**Normalization rules:**
- Trim leading/trailing whitespace
- Collapse multiple consecutive spaces to single space
- Split by comma, trim each component, remove trailing periods
- Rejoin components with ", " separator
- Idempotent: calling twice yields same result

**Edge cases covered by tests:**
- Empty/whitespace-only strings
- Trailing periods (before and after commas)
- Multiple internal spaces
- Single components without commas
- Large multi-component addresses

---

## File Map

- `foundation/parser/parser.go` — Core parsing functions (ParseAcres, ParsePrice, NormalizeAddress)
- `foundation/parser/parser_test.go` — Unit tests + property-based tests + idempotency tests

## Impact Callouts

### ⚠ ParseAcres Regex Pattern
Changing the acresPattern will affect:
- All property records flowing through scraper normalization
- Tests in `parser_test.go:11-45` (basic cases, decimals, thousands, case variants)
- Edge case tests `parser_test.go:173-194` (zero rejection, large numbers, tiny decimals)
- Property-based test `parser_test.go:111-123` validates parsed values are positive

### ⚠ ParsePrice Regex & Multiplier Logic
Changing the pricePattern or multiplier (k/m) will affect:
- Price normalization in scraper records
- Tests in `parser_test.go:48-81` (multiplier case variants, spacing, decimals)
- Edge case tests `parser_test.go:196-217` (zero rejection, large millions)
- Property-based test `parser_test.go:125-140` validates parsed values are positive

### ⚠ NormalizeAddress Idempotency
Changing whitespace/punctuation rules will break:
- Idempotency property-based test `parser_test.go:142-171` (expects f(f(x)) == f(x))
- Tests in `parser_test.go:83-109` (period/space collapsing, comma normalization)
- Address storage stability (repeated normalizations should not modify stored values)

## Consumer Integration

These functions are called by the **scraper orchestrator** during property record normalization:
- ParseAcres extracts land area from listing descriptions
- ParsePrice extracts offer/asking price from listing data
- NormalizeAddress standardizes property addresses for deduplication and comparison
