package scraper

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

// FixtureResult represents the outcome of a single fixture test.
type FixtureResult struct {
	Name    string // fixture filename without extension (e.g., "fixture-1")
	Matched bool   // true if actual Parse output matched expected JSON
	Error   string // non-empty if an error occurred
}

// TestFixtures loads HTML fixtures and expected JSON from testdata/<sourceID>/,
// runs Parse() on each, and reports matches and mismatches.
//
// The function expects a directory structure like:
//   testdata/<sourceID>/fixture-1.html
//   testdata/<sourceID>/fixture-1.json
//   testdata/<sourceID>/fixture-2.html
//   testdata/<sourceID>/fixture-2.json
//   ...
//
// For each fixture, it:
// 1. Reads the .html file as the RawFetch body
// 2. Loads the corresponding .json as the expected ParsedListing
// 3. Calls scraper.Parse(RawFetch) with a minimal RawFetch
// 4. Compares the result to expected using JSON marshaling
// 5. Reports success or failure via the *testing.T interface
//
// Returns a slice of FixtureResult for programmatic inspection, in lexicographic order by fixture name.
func TestFixtures(t *testing.T, scraper Scraper) []FixtureResult {
	sourceID := scraper.Source().ID
	fixturesDir := filepath.Join("testdata", sourceID)

	// Check if the directory exists
	_, err := os.Stat(fixturesDir)
	if errors.Is(err, os.ErrNotExist) {
		t.Logf("testdata directory not found: %s (skipping fixture tests)", fixturesDir)
		return nil
	}
	if err != nil {
		t.Fatalf("failed to stat testdata directory: %v", err)
	}

	// Read all .html files in the directory
	entries, err := os.ReadDir(fixturesDir)
	if err != nil {
		t.Fatalf("failed to read testdata directory %s: %v", fixturesDir, err)
	}

	// Collect unique fixture names (without extension)
	fixtureNames := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".json") {
			base := strings.TrimSuffix(strings.TrimSuffix(name, ".html"), ".json")
			fixtureNames[base] = true
		}
	}

	// Sort fixtures for consistent test output
	var fixtures []string
	for name := range fixtureNames {
		fixtures = append(fixtures, name)
	}
	sort.Strings(fixtures)

	// Run fixture tests
	var results []FixtureResult
	for _, fixture := range fixtures {
		result := testFixture(t, scraper, fixturesDir, fixture)
		results = append(results, result)
	}

	return results
}

func testFixture(t *testing.T, scraper Scraper, fixturesDir, fixtureName string) FixtureResult {
	sourceID := scraper.Source().ID

	htmlPath := filepath.Join(fixturesDir, fixtureName+".html")
	jsonPath := filepath.Join(fixturesDir, fixtureName+".json")

	// Read HTML fixture
	htmlBody, err := os.ReadFile(htmlPath)
	if err != nil {
		return FixtureResult{
			Name:  fixtureName,
			Error: fmt.Sprintf("failed to read HTML fixture: %v", err),
		}
	}

	// Read expected JSON
	expectedJSON, err := os.ReadFile(jsonPath)
	if err != nil {
		return FixtureResult{
			Name:  fixtureName,
			Error: fmt.Sprintf("failed to read JSON fixture: %v", err),
		}
	}

	// Construct a minimal RawFetch from the HTML body
	raw := RawFetch{
		SourceID:        sourceID,
		SourceListingID: fixtureName,
		URL:             fmt.Sprintf("https://%s.local/listing/%s", sourceID, fixtureName),
		StatusCode:      200,
		ContentType:     "text/html; charset=utf-8",
		Body:            htmlBody,
	}

	// Parse the raw fetch
	parsed, err := scraper.Parse(raw)
	if err != nil {
		return FixtureResult{
			Name:  fixtureName,
			Error: fmt.Sprintf("Parse() failed: %v", err),
		}
	}

	// Marshal the parsed result to JSON
	actualJSON, err := json.MarshalIndent(parsed, "", "  ")
	if err != nil {
		return FixtureResult{
			Name:  fixtureName,
			Error: fmt.Sprintf("failed to marshal parsed result: %v", err),
		}
	}

	// Unmarshal both to compare as objects (not strings)
	var expected, actual ParsedListing
	if err := json.Unmarshal(expectedJSON, &expected); err != nil {
		return FixtureResult{
			Name:  fixtureName,
			Error: fmt.Sprintf("failed to unmarshal expected JSON: %v", err),
		}
	}
	if err := json.Unmarshal(actualJSON, &actual); err != nil {
		return FixtureResult{
			Name:  fixtureName,
			Error: fmt.Sprintf("failed to unmarshal actual JSON: %v", err),
		}
	}

	// Compare the parsed results
	matched := parsedListingsEqual(&expected, &actual)
	if !matched {
		t.Errorf("fixture %s: parsed output does not match expected\nExpected:\n%s\n\nActual:\n%s",
			fixtureName, string(expectedJSON), string(actualJSON))
	}

	return FixtureResult{
		Name:    fixtureName,
		Matched: matched,
	}
}

// parsedListingsEqual compares two ParsedListing values for equality.
func parsedListingsEqual(a, b *ParsedListing) bool {
	if a.SourceID != b.SourceID {
		return false
	}
	if a.SourceListingID != b.SourceListingID {
		return false
	}
	if a.URL != b.URL {
		return false
	}
	if a.Title != b.Title {
		return false
	}
	if a.Description != b.Description {
		return false
	}
	if !optionalInt64Equal(a.PriceCents, b.PriceCents) {
		return false
	}
	if !optionalFloat64Equal(a.Acres, b.Acres) {
		return false
	}
	if !addressesEqual(a.Address, b.Address) {
		return false
	}
	if !optionalStringEqual(a.County, b.County) {
		return false
	}
	if !optionalStringEqual(a.State, b.State) {
		return false
	}
	if !stringSliceEqual(a.Photos, b.Photos) {
		return false
	}
	if !brokersEqual(a.Broker, b.Broker) {
		return false
	}
	if !mapEqual(a.StructuredAttrs, b.StructuredAttrs) {
		return false
	}
	if !optionalTimeEqual(a.PostedAt, b.PostedAt) {
		return false
	}
	if !optionalTimeEqual(a.UpdatedAt, b.UpdatedAt) {
		return false
	}
	if a.SourceStatus != b.SourceStatus {
		return false
	}
	return true
}

func optionalInt64Equal(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func optionalFloat64Equal(a, b *float64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func optionalStringEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func optionalTimeEqual(a, b *time.Time) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Equal(*b)
}

func addressesEqual(a, b *Address) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Street == b.Street &&
		a.City == b.City &&
		a.County == b.County &&
		a.State == b.State &&
		a.Zip == b.Zip
}

func brokersEqual(a, b *Broker) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return a.Name == b.Name &&
		a.Phone == b.Phone &&
		a.Email == b.Email
}

func stringSliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mapEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || !deepEqual(v, bv) {
			return false
		}
	}
	return true
}

func deepEqual(a, b any) bool {
	// Simple deep equality check for any type
	aJSON, _ := json.Marshal(a)
	bJSON, _ := json.Marshal(b)
	return string(aJSON) == string(bJSON)
}
