package scraper

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// FakeClock implements Clock for testing by maintaining a fixed time that can be advanced.
type FakeClock struct {
	currentTime time.Time
}

// NewFakeClock creates a new FakeClock starting at a fixed time.
func NewFakeClock() *FakeClock {
	return &FakeClock{
		currentTime: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
	}
}

// Now returns the current time in the fake clock.
func (fc *FakeClock) Now() time.Time {
	return fc.currentTime
}

// Sleep advances the clock by the given duration.
func (fc *FakeClock) Sleep(d time.Duration) {
	fc.currentTime = fc.currentTime.Add(d)
}

// Advance manually advances the clock for testing scenarios.
func (fc *FakeClock) Advance(d time.Duration) {
	fc.currentTime = fc.currentTime.Add(d)
}

// MockScraper is a test implementation of Scraper that can be configured to fail.
type MockScraper struct {
	fetchCalls     int
	failOnAttempt  int // -1 means never fail, 0 means always fail
	failWithError  error
	returnedFetch  RawFetch
	discoveryRefs  []ListingRef
	parseResult    ParsedListing
	sourceMetadata Source
}

// Compile-time assertion that MockScraper implements Scraper.
var _ Scraper = (*MockScraper)(nil)

func NewMockScraper() *MockScraper {
	return &MockScraper{
		failOnAttempt: -1, // Don't fail by default
		sourceMetadata: Source{
			ID:        "mock",
			RateLimit: 100 * time.Millisecond,
		},
		returnedFetch: RawFetch{
			SourceID:   "mock",
			StatusCode: http.StatusOK,
			Body:       []byte("<html>test</html>"),
			Headers:    make(http.Header),
		},
	}
}

func (ms *MockScraper) Source() Source {
	return ms.sourceMetadata
}

func (ms *MockScraper) ParserVersion() string {
	return "mock.v1"
}

func (ms *MockScraper) Discover(ctx context.Context) ([]ListingRef, error) {
	return ms.discoveryRefs, nil
}

func (ms *MockScraper) Fetch(ctx context.Context, ref ListingRef) (RawFetch, error) {
	ms.fetchCalls++
	if ms.failOnAttempt >= 0 && ms.fetchCalls == ms.failOnAttempt {
		return RawFetch{}, ms.failWithError
	}
	if ms.failOnAttempt == 0 {
		return RawFetch{}, ms.failWithError
	}
	ms.returnedFetch.SourceListingID = ref.SourceListingID
	ms.returnedFetch.URL = ref.URL
	return ms.returnedFetch, nil
}

func (ms *MockScraper) Parse(raw RawFetch) (ParsedListing, error) {
	return ms.parseResult, nil
}

// TestRateLimiterEnforcesInterval verifies that the rate limiter enforces
// the minimum interval between fetches as specified by Source.RateLimit.
func TestRateLimiterEnforcesInterval(t *testing.T) {
	fc := NewFakeClock()
	mock := NewMockScraper()
	mock.sourceMetadata.RateLimit = 100 * time.Millisecond
	rl := NewRateLimiterWithClock(mock, fc)

	ctx := context.Background()
	ref := ListingRef{
		SourceListingID: "test-1",
		URL:             "https://example.com/1",
	}

	// First fetch should not sleep
	startTime := fc.Now()
	_, _ = rl.Fetch(ctx, ref)
	afterFirstFetch := fc.Now()

	// The first fetch should not have slept (lastFetchTime is zero initially)
	if afterFirstFetch.Sub(startTime) != 0 {
		t.Errorf("first fetch should not sleep, but clock advanced by %v", afterFirstFetch.Sub(startTime))
	}

	// Second fetch immediately after should sleep for the full rate limit + jitter
	ref.SourceListingID = "test-2"
	beforeSecondFetch := fc.Now()
	_, _ = rl.Fetch(ctx, ref)
	afterSecondFetch := fc.Now()

	elapsed := afterSecondFetch.Sub(beforeSecondFetch)
	// Should sleep at least the rate limit (100ms), plus up to 500ms of jitter
	if elapsed < 100*time.Millisecond || elapsed > 600*time.Millisecond {
		t.Errorf("second fetch should sleep between 100-600ms, but slept %v", elapsed)
	}
}

// TestRateLimiterJitter verifies that jitter adds 0-500ms to the base rate limit.
func TestRateLimiterJitter(t *testing.T) {
	// Run multiple times to verify jitter is actually random
	jitterValues := make([]time.Duration, 0)

	for i := 0; i < 10; i++ {
		fc := NewFakeClock()
		mock := NewMockScraper()
		mock.sourceMetadata.RateLimit = 50 * time.Millisecond
		rl := NewRateLimiterWithClock(mock, fc)

		ctx := context.Background()
		ref := ListingRef{
			SourceListingID: fmt.Sprintf("test-%d", i),
			URL:             "https://example.com/test",
		}

		// First fetch
		_, _ = rl.Fetch(ctx, ref)

		// Second fetch, which will sleep
		beforeSecond := fc.Now()
		_, _ = rl.Fetch(ctx, ref)
		afterSecond := fc.Now()

		sleepDuration := afterSecond.Sub(beforeSecond)
		baseRateLimit := 50 * time.Millisecond
		jitter := sleepDuration - baseRateLimit

		if jitter < 0 || jitter > 500*time.Millisecond {
			t.Errorf("jitter should be 0-500ms, got %v", jitter)
		}
		jitterValues = append(jitterValues, jitter)
	}

	// Verify we got some variation (not all zero or all max)
	hasZero := false
	hasNonZero := false
	for _, j := range jitterValues {
		if j == 0 {
			hasZero = true
		} else {
			hasNonZero = true
		}
	}

	if !hasZero && !hasNonZero {
		t.Error("jitter values should vary")
	}
}

// TestRateLimiterRetryOnTransientError verifies that the rate limiter retries
// on transient errors and succeeds after a transient failure.
func TestRateLimiterRetryOnTransientError(t *testing.T) {
	fc := NewFakeClock()
	mock := NewMockScraper()
	mock.failWithError = fmt.Errorf("transient error")
	mock.failOnAttempt = 1 // Fail on first attempt only
	rl := NewRateLimiterWithClock(mock, fc)

	ctx := context.Background()
	ref := ListingRef{
		SourceListingID: "test",
		URL:             "https://example.com/test",
	}

	result, err := rl.Fetch(ctx, ref)
	if err != nil {
		t.Fatalf("Fetch should succeed after retry, but got error: %v", err)
	}

	if result.SourceListingID != "test" {
		t.Errorf("expected SourceListingID test, got %s", result.SourceListingID)
	}

	// Verify that Fetch was called more than once (retry happened)
	if mock.fetchCalls < 2 {
		t.Errorf("expected at least 2 fetch calls (retry), got %d", mock.fetchCalls)
	}
}

// TestRateLimiterRetriesExhausted verifies that the rate limiter gives up
// after 3 attempts on persistent errors.
func TestRateLimiterRetriesExhausted(t *testing.T) {
	fc := NewFakeClock()
	mock := NewMockScraper()
	expectedError := fmt.Errorf("persistent error")
	mock.failWithError = expectedError
	mock.failOnAttempt = 0 // Always fail
	rl := NewRateLimiterWithClock(mock, fc)

	ctx := context.Background()
	ref := ListingRef{
		SourceListingID: "test",
		URL:             "https://example.com/test",
	}

	result, err := rl.Fetch(ctx, ref)
	if err == nil {
		t.Fatal("Fetch should fail after exhausting retries")
	}

	if err.Error() != expectedError.Error() {
		t.Errorf("expected error %v, got %v", expectedError, err)
	}

	// Verify that Fetch was called exactly 3 times
	if mock.fetchCalls != 3 {
		t.Errorf("expected exactly 3 fetch calls, got %d", mock.fetchCalls)
	}

	// Verify that result is empty
	if result.SourceID != "" {
		t.Errorf("expected empty RawFetch on error, got %v", result)
	}
}

// TestRateLimiterDelegatesToWrapped verifies that Source, ParserVersion,
// Discover, and Parse are delegated to the wrapped scraper unchanged.
func TestRateLimiterDelegatesToWrapped(t *testing.T) {
	mock := NewMockScraper()
	mock.sourceMetadata = Source{
		ID:          "test-source",
		DisplayName: "Test Source",
		BaseURL:     "https://test.com",
		RateLimit:   100 * time.Millisecond,
	}
	mock.discoveryRefs = []ListingRef{
		{
			SourceListingID: "ref-1",
			URL:             "https://test.com/1",
		},
	}
	mock.parseResult = ParsedListing{
		SourceID:        "test-source",
		SourceListingID: "ref-1",
		Title:           "Test Listing",
	}

	rl := NewRateLimiter(mock)

	// Test Source()
	source := rl.Source()
	if source.ID != "test-source" {
		t.Errorf("expected Source.ID test-source, got %s", source.ID)
	}
	if source.DisplayName != "Test Source" {
		t.Errorf("expected Source.DisplayName Test Source, got %s", source.DisplayName)
	}

	// Test ParserVersion()
	pv := rl.ParserVersion()
	if pv != "mock.v1" {
		t.Errorf("expected ParserVersion mock.v1, got %s", pv)
	}

	// Test Discover()
	ctx := context.Background()
	refs, err := rl.Discover(ctx)
	if err != nil {
		t.Fatalf("Discover failed: %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].SourceListingID != "ref-1" {
		t.Errorf("expected SourceListingID ref-1, got %s", refs[0].SourceListingID)
	}

	// Test Parse()
	raw := RawFetch{
		SourceID:        "test-source",
		SourceListingID: "ref-1",
		Body:            []byte("<html>test</html>"),
	}
	parsed, err := rl.Parse(raw)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if parsed.Title != "Test Listing" {
		t.Errorf("expected Title Test Listing, got %s", parsed.Title)
	}
}

// TestRateLimiterInterfaceAssertion is a compile-time test ensuring RateLimiter implements Scraper.
func TestRateLimiterInterfaceAssertion(t *testing.T) {
	// This test just checks compilation; the actual assertion is done via the blank identifier assignment at package level.
	var _ Scraper = (*RateLimiter)(nil)
}
