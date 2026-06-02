package attrs_test

import (
	"testing"

	"github.com/cbrophy/land_trakker/foundation/attrs"
)

// --------------------------------------------------------------------------
// WaterFrontage
// --------------------------------------------------------------------------

func TestExtractWaterFrontage(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		cases := []string{
			"Property borders a scenic creek with beautiful views.",
			"River frontage on both sides of the land.",
			"Riparian rights included with purchase.",
			"Small pond at the back of the parcel.",
			"Lakefront property with 200 ft of water frontage.",
		}
		for _, tc := range cases {
			r, found := attrs.ExtractWaterFrontage(tc)
			if !found {
				t.Errorf("expected found=true for %q", tc)
			}
			if r.Value != "true" {
				t.Errorf("expected value=true for %q, got %q", tc, r.Value)
			}
			if r.Confidence <= 0 {
				t.Errorf("expected confidence>0 for %q", tc)
			}
			if r.Evidence == "" {
				t.Errorf("expected non-empty evidence for %q", tc)
			}
		}
	})

	t.Run("negation", func(t *testing.T) {
		cases := []string{
			"No creek or stream on the property.",
			"Without water front access.",
			"No lake nearby.",
		}
		for _, tc := range cases {
			r, found := attrs.ExtractWaterFrontage(tc)
			if !found {
				t.Errorf("expected found=true for %q", tc)
			}
			if r.Value != "false" {
				t.Errorf("expected value=false for %q, got %q", tc, r.Value)
			}
			if r.Confidence <= 0 {
				t.Errorf("expected confidence>0 for %q", tc)
			}
			if r.Evidence == "" {
				t.Errorf("expected non-empty evidence for %q", tc)
			}
		}
	})

	t.Run("no_match", func(t *testing.T) {
		cases := []string{
			"Flat farmland with road access.",
			"Hunting property with mature timber.",
		}
		for _, tc := range cases {
			_, found := attrs.ExtractWaterFrontage(tc)
			if found {
				t.Errorf("expected found=false for %q", tc)
			}
		}
	})
}

// --------------------------------------------------------------------------
// OffGrid
// --------------------------------------------------------------------------

func TestExtractOffGrid(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		cases := []string{
			"This is a true off-grid retreat.",
			"Off grid living at its finest.",
			"No power lines reach this remote property.",
			"No utilities available at this location.",
			"Live off the grid in complete solitude.",
		}
		for _, tc := range cases {
			r, found := attrs.ExtractOffGrid(tc)
			if !found {
				t.Errorf("expected found=true for %q", tc)
			}
			if r.Value != "true" {
				t.Errorf("expected value=true for %q, got %q", tc, r.Value)
			}
			if r.Confidence <= 0 {
				t.Errorf("expected confidence>0 for %q", tc)
			}
			if r.Evidence == "" {
				t.Errorf("expected non-empty evidence for %q", tc)
			}
		}
	})

	t.Run("negation", func(t *testing.T) {
		cases := []string{
			"Power available at the road.",
			"Electricity connected to site.",
			"On-grid with all utilities.",
		}
		for _, tc := range cases {
			r, found := attrs.ExtractOffGrid(tc)
			if !found {
				t.Errorf("expected found=true for %q", tc)
			}
			if r.Value != "false" {
				t.Errorf("expected value=false for %q, got %q", tc, r.Value)
			}
			if r.Confidence <= 0 {
				t.Errorf("expected confidence>0 for %q", tc)
			}
			if r.Evidence == "" {
				t.Errorf("expected non-empty evidence for %q", tc)
			}
		}
	})

	t.Run("no_match", func(t *testing.T) {
		cases := []string{
			"Beautiful river property with mature trees.",
			"Agricultural land with good road access.",
		}
		for _, tc := range cases {
			_, found := attrs.ExtractOffGrid(tc)
			if found {
				t.Errorf("expected found=false for %q", tc)
			}
		}
	})
}

// --------------------------------------------------------------------------
// RoadAccess
// --------------------------------------------------------------------------

func TestExtractRoadAccess(t *testing.T) {
	t.Run("paved", func(t *testing.T) {
		r, found := attrs.ExtractRoadAccess("Property has paved road frontage.")
		if !found {
			t.Fatal("expected found=true")
		}
		if r.Value != "paved" {
			t.Errorf("expected paved, got %q", r.Value)
		}
		if r.Confidence <= 0 {
			t.Error("expected confidence>0")
		}
		if r.Evidence == "" {
			t.Error("expected non-empty evidence")
		}
	})

	t.Run("gravel", func(t *testing.T) {
		r, found := attrs.ExtractRoadAccess("Gravel road leads to the property.")
		if !found {
			t.Fatal("expected found=true")
		}
		if r.Value != "gravel" {
			t.Errorf("expected gravel, got %q", r.Value)
		}
		if r.Evidence == "" {
			t.Error("expected non-empty evidence")
		}
	})

	t.Run("dirt", func(t *testing.T) {
		r, found := attrs.ExtractRoadAccess("Accessed via a dirt road through the woods.")
		if !found {
			t.Fatal("expected found=true")
		}
		if r.Value != "dirt" {
			t.Errorf("expected dirt, got %q", r.Value)
		}
		if r.Evidence == "" {
			t.Error("expected non-empty evidence")
		}
	})

	t.Run("seasonal", func(t *testing.T) {
		r, found := attrs.ExtractRoadAccess("Seasonal road access only in dry months.")
		if !found {
			t.Fatal("expected found=true")
		}
		if r.Value != "seasonal" {
			t.Errorf("expected seasonal, got %q", r.Value)
		}
		if r.Evidence == "" {
			t.Error("expected non-empty evidence")
		}
	})

	t.Run("none", func(t *testing.T) {
		r, found := attrs.ExtractRoadAccess("This landlocked parcel has no road access.")
		if !found {
			t.Fatal("expected found=true")
		}
		if r.Value != "none" {
			t.Errorf("expected none, got %q", r.Value)
		}
		if r.Evidence == "" {
			t.Error("expected non-empty evidence")
		}
	})

	t.Run("no_match", func(t *testing.T) {
		cases := []string{
			"Beautiful wooded property with a creek.",
			"Hunting land with mature oak trees.",
		}
		for _, tc := range cases {
			_, found := attrs.ExtractRoadAccess(tc)
			if found {
				t.Errorf("expected found=false for %q", tc)
			}
		}
	})

	t.Run("evidence_non_empty", func(t *testing.T) {
		r, found := attrs.ExtractRoadAccess("County road runs along the south boundary.")
		if !found {
			t.Fatal("expected found=true")
		}
		if r.Evidence == "" {
			t.Error("expected non-empty evidence")
		}
	})
}

// --------------------------------------------------------------------------
// Power
// --------------------------------------------------------------------------

func TestExtractPower(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		cases := []string{
			"Power available at the road.",
			"Electricity connected to the site.",
			"Electric service runs along the front boundary.",
			"Power to site already installed.",
			"Electricity on site, ready to build.",
		}
		for _, tc := range cases {
			r, found := attrs.ExtractPower(tc)
			if !found {
				t.Errorf("expected found=true for %q", tc)
			}
			if r.Value != "true" {
				t.Errorf("expected value=true for %q, got %q", tc, r.Value)
			}
			if r.Confidence <= 0 {
				t.Errorf("expected confidence>0 for %q", tc)
			}
			if r.Evidence == "" {
				t.Errorf("expected non-empty evidence for %q", tc)
			}
		}
	})

	t.Run("negation", func(t *testing.T) {
		cases := []string{
			"No power available at this remote location.",
			"No electricity on this off-grid parcel.",
			"Off-grid property with no utilities.",
		}
		for _, tc := range cases {
			r, found := attrs.ExtractPower(tc)
			if !found {
				t.Errorf("expected found=true for %q", tc)
			}
			if r.Value != "false" {
				t.Errorf("expected value=false for %q, got %q", tc, r.Value)
			}
			if r.Confidence <= 0 {
				t.Errorf("expected confidence>0 for %q", tc)
			}
			if r.Evidence == "" {
				t.Errorf("expected non-empty evidence for %q", tc)
			}
		}
	})

	t.Run("no_match", func(t *testing.T) {
		cases := []string{
			"Wooded hunting parcel with creek.",
			"Flat farmland with good soils.",
		}
		for _, tc := range cases {
			_, found := attrs.ExtractPower(tc)
			if found {
				t.Errorf("expected found=false for %q", tc)
			}
		}
	})
}

// --------------------------------------------------------------------------
// Well
// --------------------------------------------------------------------------

func TestExtractWell(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		cases := []string{
			"Drilled well on the property.",
			"Artesian well provides abundant water.",
			"Property has a domestic water well.",
			"Water well already installed.",
			"There is a well at the homesite.",
		}
		for _, tc := range cases {
			r, found := attrs.ExtractWell(tc)
			if !found {
				t.Errorf("expected found=true for %q", tc)
			}
			if r.Value != "true" {
				t.Errorf("expected value=true for %q, got %q", tc, r.Value)
			}
			if r.Confidence <= 0 {
				t.Errorf("expected confidence>0 for %q", tc)
			}
			if r.Evidence == "" {
				t.Errorf("expected non-empty evidence for %q", tc)
			}
		}
	})

	t.Run("negation", func(t *testing.T) {
		cases := []string{
			"No well on this parcel.",
			"Without a well, buyer must install.",
			"No domestic water available.",
		}
		for _, tc := range cases {
			r, found := attrs.ExtractWell(tc)
			if !found {
				t.Errorf("expected found=true for %q", tc)
			}
			if r.Value != "false" {
				t.Errorf("expected value=false for %q, got %q", tc, r.Value)
			}
			if r.Confidence <= 0 {
				t.Errorf("expected confidence>0 for %q", tc)
			}
			if r.Evidence == "" {
				t.Errorf("expected non-empty evidence for %q", tc)
			}
		}
	})

	t.Run("no_match", func(t *testing.T) {
		cases := []string{
			"Hunting land bordering national forest.",
			"Commercial zoned parcel near highway.",
		}
		for _, tc := range cases {
			_, found := attrs.ExtractWell(tc)
			if found {
				t.Errorf("expected found=false for %q", tc)
			}
		}
	})
}

// --------------------------------------------------------------------------
// Septic
// --------------------------------------------------------------------------

func TestExtractSeptic(t *testing.T) {
	t.Run("positive", func(t *testing.T) {
		cases := []string{
			"Septic system already installed.",
			"Mound system in place.",
			"Leach field approved and ready.",
			"Connected to public sewer.",
			"Property has a septic permit.",
		}
		for _, tc := range cases {
			r, found := attrs.ExtractSeptic(tc)
			if !found {
				t.Errorf("expected found=true for %q", tc)
			}
			if r.Value != "true" {
				t.Errorf("expected value=true for %q, got %q", tc, r.Value)
			}
			if r.Confidence <= 0 {
				t.Errorf("expected confidence>0 for %q", tc)
			}
			if r.Evidence == "" {
				t.Errorf("expected non-empty evidence for %q", tc)
			}
		}
	})

	t.Run("negation", func(t *testing.T) {
		cases := []string{
			"No septic installed, buyer responsible.",
			"Without septic approval, hard to build.",
			"No sewer available in this area.",
		}
		for _, tc := range cases {
			r, found := attrs.ExtractSeptic(tc)
			if !found {
				t.Errorf("expected found=true for %q", tc)
			}
			if r.Value != "false" {
				t.Errorf("expected value=false for %q, got %q", tc, r.Value)
			}
			if r.Confidence <= 0 {
				t.Errorf("expected confidence>0 for %q", tc)
			}
			if r.Evidence == "" {
				t.Errorf("expected non-empty evidence for %q", tc)
			}
		}
	})

	t.Run("no_match", func(t *testing.T) {
		cases := []string{
			"River frontage with good timber.",
			"Agricultural land near town.",
		}
		for _, tc := range cases {
			_, found := attrs.ExtractSeptic(tc)
			if found {
				t.Errorf("expected found=false for %q", tc)
			}
		}
	})
}

// --------------------------------------------------------------------------
// PropertyType
// --------------------------------------------------------------------------

func TestExtractPropertyType(t *testing.T) {
	t.Run("timber", func(t *testing.T) {
		r, found := attrs.ExtractPropertyType("Mature timber stand with diverse woodland habitat.")
		if !found {
			t.Fatal("expected found=true")
		}
		if r.Value != "timber" {
			t.Errorf("expected timber, got %q", r.Value)
		}
		if r.Confidence <= 0 {
			t.Error("expected confidence>0")
		}
		if r.Evidence == "" {
			t.Error("expected non-empty evidence")
		}
	})

	t.Run("agricultural", func(t *testing.T) {
		r, found := attrs.ExtractPropertyType("Prime farmland with irrigated hay fields and rich crop soil.")
		if !found {
			t.Fatal("expected found=true")
		}
		if r.Value != "agricultural" {
			t.Errorf("expected agricultural, got %q", r.Value)
		}
		if r.Evidence == "" {
			t.Error("expected non-empty evidence")
		}
	})

	t.Run("hunting", func(t *testing.T) {
		r, found := attrs.ExtractPropertyType("Premier hunting land with trophy deer and elk habitat.")
		if !found {
			t.Fatal("expected found=true")
		}
		if r.Value != "hunting" {
			t.Errorf("expected hunting, got %q", r.Value)
		}
		if r.Evidence == "" {
			t.Error("expected non-empty evidence")
		}
	})

	t.Run("homesite", func(t *testing.T) {
		r, found := attrs.ExtractPropertyType("Residential lot ready to build your dream home site.")
		if !found {
			t.Fatal("expected found=true")
		}
		if r.Value != "homesite" {
			t.Errorf("expected homesite, got %q", r.Value)
		}
		if r.Evidence == "" {
			t.Error("expected non-empty evidence")
		}
	})

	t.Run("commercial", func(t *testing.T) {
		r, found := attrs.ExtractPropertyType("Commercial zoned parcel with high visibility.")
		if !found {
			t.Fatal("expected found=true")
		}
		if r.Value != "commercial" {
			t.Errorf("expected commercial, got %q", r.Value)
		}
		if r.Evidence == "" {
			t.Error("expected non-empty evidence")
		}
	})

	t.Run("multi_signal_highest_wins", func(t *testing.T) {
		// 3 agricultural signals vs 1 hunting signal
		r, found := attrs.ExtractPropertyType("Farmland with hay production and crop rotation on this working ranch, plus some deer hunting.")
		if !found {
			t.Fatal("expected found=true")
		}
		if r.Value != "agricultural" {
			t.Errorf("expected agricultural to win, got %q", r.Value)
		}
	})

	t.Run("no_match", func(t *testing.T) {
		_, found := attrs.ExtractPropertyType("Beautiful property with great views and road access.")
		if found {
			t.Error("expected found=false for generic text")
		}
	})
}

// --------------------------------------------------------------------------
// ExtractAll
// --------------------------------------------------------------------------

func TestExtractAll(t *testing.T) {
	t.Run("multi_attr_text", func(t *testing.T) {
		text := "Creek frontage farmland with drilled well, septic installed, power available, gravel road access."
		result := attrs.ExtractAll(text)

		expectedKeys := []string{"water_frontage", "well", "septic", "power", "road_access", "property_type"}
		for _, k := range expectedKeys {
			if _, ok := result[k]; !ok {
				t.Errorf("expected key %q to be present in result", k)
			}
		}
	})

	t.Run("empty_text", func(t *testing.T) {
		result := attrs.ExtractAll("")
		if len(result) != 0 {
			t.Errorf("expected empty map for empty text, got %d entries", len(result))
		}
	})
}
