package sourcebus_test

import (
	"testing"

	"github.com/cbrophy/land_trakker/business/domain/source"
	"github.com/cbrophy/land_trakker/business/sdk/sourcebus"
)

func intPtr(n int) *int { return &n }

func makeSrc(ratio float64) source.Source {
	return source.Source{
		ID:                            "src1",
		MinResultRatioForInactivation: ratio,
	}
}

func makeRun(discovered *int) source.ScrapeRun {
	return source.ScrapeRun{
		SourceID:        "src1",
		DiscoveredCount: discovered,
		Status:          source.RunStatusOK,
	}
}

func TestIsRunHealthy_NilPriorRun(t *testing.T) {
	run := makeRun(intPtr(50))
	src := makeSrc(0.5)
	if !sourcebus.IsRunHealthy(run, src, nil) {
		t.Error("want true when priorRun is nil")
	}
}

func TestIsRunHealthy_NilPriorDiscoveredCount(t *testing.T) {
	run := makeRun(intPtr(50))
	src := makeSrc(0.5)
	prior := makeRun(nil)
	if !sourcebus.IsRunHealthy(run, src, &prior) {
		t.Error("want true when prior DiscoveredCount is nil")
	}
}

func TestIsRunHealthy_ZeroPriorCount(t *testing.T) {
	run := makeRun(intPtr(50))
	src := makeSrc(0.5)
	prior := makeRun(intPtr(0))
	if !sourcebus.IsRunHealthy(run, src, &prior) {
		t.Error("want true when prior DiscoveredCount is 0")
	}
}

func TestIsRunHealthy_AboveThreshold(t *testing.T) {
	// discovered=60, prior=100, ratio=0.5 → threshold=50, 60>=50 → true
	run := makeRun(intPtr(60))
	src := makeSrc(0.5)
	prior := makeRun(intPtr(100))
	if !sourcebus.IsRunHealthy(run, src, &prior) {
		t.Error("want true when discovered=60 >= threshold=50")
	}
}

func TestIsRunHealthy_AtThreshold(t *testing.T) {
	// discovered=50, prior=100, ratio=0.5 → threshold=50, 50>=50 → true
	run := makeRun(intPtr(50))
	src := makeSrc(0.5)
	prior := makeRun(intPtr(100))
	if !sourcebus.IsRunHealthy(run, src, &prior) {
		t.Error("want true when discovered=50 exactly at threshold=50")
	}
}

func TestIsRunHealthy_BelowThreshold(t *testing.T) {
	// discovered=49, prior=100, ratio=0.5 → threshold=50, 49<50 → false
	run := makeRun(intPtr(49))
	src := makeSrc(0.5)
	prior := makeRun(intPtr(100))
	if sourcebus.IsRunHealthy(run, src, &prior) {
		t.Error("want false when discovered=49 < threshold=50")
	}
}

func TestIsRunHealthy_NilCurrentDiscoveredCount(t *testing.T) {
	// current run DiscoveredCount=nil → false
	run := makeRun(nil)
	src := makeSrc(0.5)
	prior := makeRun(intPtr(100))
	if sourcebus.IsRunHealthy(run, src, &prior) {
		t.Error("want false when current DiscoveredCount is nil")
	}
}
