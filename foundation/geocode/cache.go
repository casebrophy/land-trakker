package geocode

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
)

// CacheStore abstracts geocode_cache persistence.
type CacheStore interface {
	Lookup(ctx context.Context, addressKey string) (Result, bool, error)
	Store(ctx context.Context, addressKey string, r Result) error
}

// CachingGeocoder wraps a Geocoder with persistent cache, daily limit, and county-centroid fallback.
// It also implements Geocoder.
type CachingGeocoder struct {
	inner     Geocoder
	store     CacheStore
	limit     int32
	dailyUsed atomic.Int32
}

// NewCachingGeocoder constructs a CachingGeocoder.
// limit is the daily request cap; 0 means unlimited.
func NewCachingGeocoder(inner Geocoder, store CacheStore, limit int32) *CachingGeocoder {
	return &CachingGeocoder{inner: inner, store: store, limit: limit}
}

// ResetDailyCount resets the in-memory daily usage counter. Call at midnight.
func (c *CachingGeocoder) ResetDailyCount() { c.dailyUsed.Store(0) }

// DailyUsed returns the number of real geocoding calls made today.
func (c *CachingGeocoder) DailyUsed() int32 { return c.dailyUsed.Load() }

// Geocode returns a cached result when available; otherwise calls the inner geocoder,
// falls back to county centroid on failure, and stores the result.
func (c *CachingGeocoder) Geocode(ctx context.Context, address, city, county, state string) (Result, error) {
	key := normalizeKey(address, city, county, state)

	if r, ok, err := c.store.Lookup(ctx, key); err == nil && ok {
		return r, nil
	}

	if c.limit > 0 && c.dailyUsed.Add(1) > c.limit {
		c.dailyUsed.Add(-1)
		return Result{}, ErrDailyLimitExceeded
	}

	r, err := c.inner.Geocode(ctx, address, city, county, state)
	if err != nil {
		if centroid, ok := CountyCentroid(county, state); ok {
			_ = c.store.Store(ctx, key, centroid)
			return centroid, nil
		}
		return Result{}, err
	}

	_ = c.store.Store(ctx, key, r)
	return r, nil
}

// normalizeKey returns a stable lowercase cache key from address components.
func normalizeKey(address, city, county, state string) string {
	parts := [4]string{
		strings.ToLower(strings.TrimSpace(address)),
		strings.ToLower(strings.TrimSpace(city)),
		strings.ToLower(strings.TrimSpace(county)),
		strings.ToLower(strings.TrimSpace(state)),
	}
	return strings.Join(parts[:], "|")
}

// MemStore is an in-memory CacheStore for tests.
type MemStore struct {
	mu    sync.RWMutex
	cache map[string]Result
}

// NewMemStore returns a new empty MemStore.
func NewMemStore() *MemStore {
	return &MemStore{cache: make(map[string]Result)}
}

func (m *MemStore) Lookup(_ context.Context, key string) (Result, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	r, ok := m.cache[key]
	return r, ok, nil
}

func (m *MemStore) Store(_ context.Context, key string, r Result) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cache[key] = r
	return nil
}
