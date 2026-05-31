package scraper

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// Clock is an interface for dependency injection of time functions (enables testing with a fake clock).
type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

// RealClock wraps the standard time package.
type RealClock struct{}

func (rc *RealClock) Now() time.Time {
	return time.Now()
}

func (rc *RealClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

// RateLimiter wraps a Scraper and enforces rate limits on Fetch() calls.
// It tracks the last fetch time per Scraper instance and sleeps as needed
// to respect RateLimit + jitter. It also implements exponential backoff retry logic.
type RateLimiter struct {
	wrapped       Scraper
	clock         Clock
	mu            sync.Mutex
	lastFetchTime time.Time
}

// Compile-time assertion that RateLimiter implements Scraper.
var _ Scraper = (*RateLimiter)(nil)

// NewRateLimiter creates a new RateLimiter wrapping the given Scraper.
// Uses the real system clock.
func NewRateLimiter(wrapped Scraper) *RateLimiter {
	return &RateLimiter{
		wrapped: wrapped,
		clock:   &RealClock{},
	}
}

// NewRateLimiterWithClock creates a new RateLimiter with a custom Clock for testing.
func NewRateLimiterWithClock(wrapped Scraper, clock Clock) *RateLimiter {
	return &RateLimiter{
		wrapped: wrapped,
		clock:   clock,
	}
}

// Source delegates to the wrapped scraper.
func (rl *RateLimiter) Source() Source {
	return rl.wrapped.Source()
}

// ParserVersion delegates to the wrapped scraper.
func (rl *RateLimiter) ParserVersion() string {
	return rl.wrapped.ParserVersion()
}

// Discover delegates to the wrapped scraper.
func (rl *RateLimiter) Discover(ctx context.Context) ([]ListingRef, error) {
	return rl.wrapped.Discover(ctx)
}

// Parse delegates to the wrapped scraper.
func (rl *RateLimiter) Parse(raw RawFetch) (ParsedListing, error) {
	return rl.wrapped.Parse(raw)
}

// Fetch enforces the rate limit from Source.RateLimit with jitter (0-500ms),
// and implements exponential backoff retry on transient errors (up to 3 attempts).
func (rl *RateLimiter) Fetch(ctx context.Context, ref ListingRef) (RawFetch, error) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	source := rl.wrapped.Source()
	rateLimitDuration := source.RateLimit

	// Calculate the time to sleep based on the rate limit and when the last fetch occurred
	now := rl.clock.Now()
	timeSinceLastFetch := now.Sub(rl.lastFetchTime)

	// Add jitter (0-500ms) to the rate limit
	jitterMs := rand.Intn(501) // 0-500 inclusive
	jitter := time.Duration(jitterMs) * time.Millisecond
	requiredInterval := rateLimitDuration + jitter

	if timeSinceLastFetch < requiredInterval {
		sleepDuration := requiredInterval - timeSinceLastFetch
		rl.clock.Sleep(sleepDuration)
	}

	// Update the last fetch time before attempting the fetch
	rl.lastFetchTime = rl.clock.Now()

	// Implement exponential backoff retry on transient errors (up to 3 attempts)
	const maxAttempts = 3
	var lastErr error
	backoffBase := 100 * time.Millisecond

	for attempt := 0; attempt < maxAttempts; attempt++ {
		result, err := rl.wrapped.Fetch(ctx, ref)
		if err == nil {
			return result, nil
		}

		// Check if the error is transient
		if !isTransientError(err) {
			return RawFetch{}, err
		}

		lastErr = err
		if attempt < maxAttempts-1 {
			// Calculate exponential backoff: 100ms, 200ms, 400ms
			backoffDuration := backoffBase * time.Duration(1<<uint(attempt))
			rl.clock.Sleep(backoffDuration)
		}
	}

	return RawFetch{}, lastErr
}

// isTransientError checks if an error is transient and should be retried.
// For now, we consider all errors as transient, but this can be refined
// to check for specific error types like temporary network errors.
func isTransientError(err error) bool {
	// In a production system, we would check:
	// - net.Error with Timeout() or Temporary() methods
	// - Specific HTTP status codes (5xx, 429, etc.)
	// For this implementation, we treat all errors as transient
	// to keep it simple and let the caller decide retry strategy.
	return true
}
