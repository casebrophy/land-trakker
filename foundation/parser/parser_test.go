package parser_test

import (
	"fmt"
	"testing"
	"testing/quick"

	"github.com/cbrophy/land_trakker/foundation/parser"
)

func TestParseAcres(t *testing.T) {
	tests := []struct {
		name  string
		input string
		value float64
		ok    bool
	}{
		{"basic acres", "40 acres", 40, true},
		{"ac abbreviation", "40 ac", 40, true},
		{"acre singular", "1 acre", 1, true},
		{"acre tract", "40-acre tract", 40, true},
		{"decimal acres", "40.5 acres", 40.5, true},
		{"comma thousands", "1,000 acres", 1000, true},
		{"with tilde", "~40 acres", 40, true},
		{"with plus-minus", "±40 acres", 40, true},
		{"case insensitive", "40 ACRES", 40, true},
		{"mixed case", "40 Acres", 40, true},
		{"no value", "no acreage here", 0, false},
		{"empty string", "", 0, false},
		{"just number", "40", 0, false},
		{"decimal with comma", "1,000.5 acres", 1000.5, true},
		{"spaces around tilde", "~ 40 acres", 40, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := parser.ParseAcres(tt.input)
			if ok != tt.ok {
				t.Errorf("ok=%v, want %v", ok, tt.ok)
			}
			if ok && val != tt.value {
				t.Errorf("value=%v, want %v", val, tt.value)
			}
		})
	}
}

func TestParsePrice(t *testing.T) {
	tests := []struct {
		name  string
		input string
		value int64
		ok    bool
	}{
		{"basic dollars", "$1,200,000", 1200000, true},
		{"k lowercase", "$500k", 500000, true},
		{"K uppercase", "$500K", 500000, true},
		{"m lowercase", "$1.2m", 1200000, true},
		{"M uppercase", "$1.2M", 1200000, true},
		{"with decimal", "$500,000.00", 500000, true},
		{"no price", "no price", 0, false},
		{"empty string", "", 0, false},
		{"just number", "500000", 0, false},
		{"no multiplier", "$500", 500, true},
		{"space after dollar", "$ 1,200,000", 1200000, true},
		{"space before multiplier", "$500 k", 500000, true},
		{"decimal k", "$1.5k", 1500, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, ok := parser.ParsePrice(tt.input)
			if ok != tt.ok {
				t.Errorf("ok=%v, want %v", ok, tt.ok)
			}
			if ok && val != tt.value {
				t.Errorf("value=%v, want %v", val, tt.value)
			}
		})
	}
}

func TestNormalizeAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"trim period", "123 Main St.", "123 Main St"},
		{"trim whitespace", "  123   Main   St  ", "123 Main St"},
		{"collapse spaces", "123    Main    St", "123 Main St"},
		{"full address", "123 Main St, Boise, ID 83701", "123 Main St, Boise, ID 83701"},
		{"empty string", "", ""},
		{"only spaces", "   ", ""},
		{"trailing period with comma", "123 Main St., Boise, ID 83701.", "123 Main St, Boise, ID 83701"},
		{"period before comma", "123 Main St., Boise.", "123 Main St, Boise"},
		{"single component", "Boise", "Boise"},
		{"period only component", ".", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.NormalizeAddress(tt.input)
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseAcresProperty(t *testing.T) {
	prop := func(f float64) bool {
		s := fmt.Sprintf("%v acres", f)
		val, ok := parser.ParseAcres(s)
		if ok {
			return val > 0
		}
		return true
	}
	if err := quick.Check(prop, nil); err != nil {
		t.Errorf("property failed: %v", err)
	}
}

func TestParsePriceProperty(t *testing.T) {
	prop := func(u uint32) bool {
		if u == 0 {
			return true
		}
		s := fmt.Sprintf("$%v", u)
		val, ok := parser.ParsePrice(s)
		if ok {
			return val > 0
		}
		return true
	}
	if err := quick.Check(prop, nil); err != nil {
		t.Errorf("property failed: %v", err)
	}
}

func TestNormalizeAddressIdempotent(t *testing.T) {
	inputs := []string{
		"123 Main St., Boise, ID 83701",
		"  456   Oak Ave  ,  Portland  ,  OR  ",
		"789 Elm St.",
		"downtown",
		"",
	}

	for _, s := range inputs {
		t.Run(fmt.Sprintf("idempotent %q", s), func(t *testing.T) {
			once := parser.NormalizeAddress(s)
			twice := parser.NormalizeAddress(once)
			if once != twice {
				t.Errorf("not idempotent: once=%q, twice=%q", once, twice)
			}
		})
	}

	t.Run("random idempotent", func(t *testing.T) {
		prop := func(s string) bool {
			once := parser.NormalizeAddress(s)
			twice := parser.NormalizeAddress(once)
			return once == twice
		}
		if err := quick.Check(prop, &quick.Config{MaxCount: 100}); err != nil {
			t.Errorf("property failed: %v", err)
		}
	})
}

func TestParseAcresEdgeCases(t *testing.T) {
	t.Run("large numbers", func(t *testing.T) {
		val, ok := parser.ParseAcres("10,000,000 acres")
		if !ok || val != 10000000 {
			t.Errorf("got %v, %v", val, ok)
		}
	})

	t.Run("very small decimal", func(t *testing.T) {
		val, ok := parser.ParseAcres("0.1 acres")
		if !ok || val != 0.1 {
			t.Errorf("got %v, %v", val, ok)
		}
	})

	t.Run("zero acres fails", func(t *testing.T) {
		_, ok := parser.ParseAcres("0 acres")
		if ok {
			t.Error("expected false for 0 acres")
		}
	})
}

func TestParsePriceEdgeCases(t *testing.T) {
	t.Run("mixed case multiplier", func(t *testing.T) {
		val, ok := parser.ParsePrice("$1.5K")
		if !ok || val != 1500 {
			t.Errorf("got %v, %v", val, ok)
		}
	})

	t.Run("zero price fails", func(t *testing.T) {
		_, ok := parser.ParsePrice("$0")
		if ok {
			t.Error("expected false for $0")
		}
	})

	t.Run("large millions", func(t *testing.T) {
		val, ok := parser.ParsePrice("$10.5m")
		if !ok || val != 10500000 {
			t.Errorf("got %v, %v", val, ok)
		}
	})
}
