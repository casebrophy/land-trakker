package scraper

import (
	"testing"
)

func TestFixturesWithFakeBroker(t *testing.T) {
	fb := NewFakeBroker()
	results := TestFixtures(t, fb)

	// Verify all fixtures were found and matched
	if len(results) == 0 {
		t.Fatal("expected to find fixtures, but got none")
	}

	expectedFixtures := map[string]bool{
		"fixture-1": false,
		"fixture-2": false,
		"fixture-3": false,
	}

	for _, result := range results {
		if result.Error != "" {
			t.Errorf("fixture %s: %s", result.Name, result.Error)
		}
		if !result.Matched {
			t.Errorf("fixture %s: output did not match expected", result.Name)
		}
		if _, ok := expectedFixtures[result.Name]; ok {
			expectedFixtures[result.Name] = true
		} else {
			t.Errorf("unexpected fixture: %s", result.Name)
		}
	}

	// Verify all expected fixtures were found
	for fixture, found := range expectedFixtures {
		if !found {
			t.Errorf("missing fixture: %s", fixture)
		}
	}

	// Verify results are in sorted order
	for i := 1; i < len(results); i++ {
		if results[i].Name < results[i-1].Name {
			t.Errorf("results not in sorted order: %s > %s", results[i-1].Name, results[i].Name)
		}
	}
}

func TestFixtureCounting(t *testing.T) {
	fb := NewFakeBroker()
	results := TestFixtures(t, fb)

	// Should have exactly 3 results
	if len(results) != 3 {
		t.Fatalf("expected 3 fixtures, got %d", len(results))
	}

	// Count successes
	var matched, failed int
	for _, result := range results {
		if result.Error != "" {
			failed++
		} else if result.Matched {
			matched++
		}
	}

	if matched != 3 {
		t.Errorf("expected 3 matched fixtures, got %d", matched)
	}
	if failed != 0 {
		t.Errorf("expected 0 failed fixtures, got %d", failed)
	}
}
