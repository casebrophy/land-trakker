# Geocode Backend Architecture

## Foundation Layer: `foundation/geocode/`

Geocode provides structured address-to-coordinates resolution via pluggable Geocoder implementations, with persistent caching, daily request limiting, and county-centroid fallback for failed queries. Used by listing ingestion to convert raw parcel addresses into spatial data.

### Core Types

**Precision** — enum string for geocoding accuracy level:
- `"rooftop"` — precise address-level match
- `"street"` — street-level accuracy
- `"locality"` — city/town level
- `"county_centroid"` — fallback approximate centroid

**Result** — output of a geocoding operation:
```go
type Result struct {
    Lat        float64    // latitude
    Lng        float64    // longitude
    Precision  Precision  // accuracy level
    Provider   string     // "google", "fake", "builtin"
    Confidence float64    // 0.0–1.0 score (optional)
}
```

**Geocoder interface** — pluggable geocoding implementation:
```go
type Geocoder interface {
    Geocode(ctx context.Context, address, city, county, state string) (Result, error)
}
```
Callers pass structured address components; returns geocoded coordinates or error.

**ErrDailyLimitExceeded** — var error returned when daily request cap exhausted.

### Cache Layer

**CacheStore interface** — abstract persistence contract for geocode results:
```go
type CacheStore interface {
    Lookup(ctx context.Context, addressKey string) (Result, bool, error)
    Store(ctx context.Context, addressKey string, r Result) error
}
```
Key normalization: `normalizeKey()` lowercases and joins address components as `"address|city|county|state"`.

**CachingGeocoder** — wrapper that orchestrates caching, daily limits, and fallback:
```go
type CachingGeocoder struct {
    inner     Geocoder       // wrapped real geocoder (e.g., Google)
    store     CacheStore     // persistence layer
    limit     int32          // daily request cap (0 = unlimited)
    dailyUsed atomic.Int32   // in-memory counter
}
```

**NewCachingGeocoder(inner, store, limit)** — constructor.

**Methods:**
- **Geocode(ctx, address, city, county, state)** — checks cache; on miss, increments daily counter, calls inner geocoder, or falls back to CountyCentroid on error. Stores result and returns it.
- **ResetDailyCount()** — call at midnight to reset atomic counter.
- **DailyUsed()** — returns current daily usage counter.

**MemStore** — in-memory CacheStore for tests (mutex-protected map):
```go
type MemStore struct {
    mu    sync.RWMutex
    cache map[string]Result
}
```
**NewMemStore()** — constructor.

### Implementations

**FakeGeocoder** — deterministic test implementation:
```go
type FakeGeocoder struct{}

func (FakeGeocoder) Geocode(ctx context.Context, address, _, _, _ string) (Result, error)
```
Returns fixed Boise coordinates (Lat: 43.6150, Lng: -116.2023, Precision: "rooftop", Provider: "fake", Confidence: 0.99) for all addresses except those containing "unknown" (which trigger error to exercise county-centroid fallback).

**PGStore** — PostgreSQL-backed CacheStore using PostGIS:
```go
type PGStore struct {
    pool *pgxpool.Pool
}
```
**NewPGStore(pool)** — constructor.

**Methods:**
- **Lookup(ctx, key)** — queries geocode_cache table:
  ```sql
  SELECT ST_AsText(geom), precision, provider, COALESCE(confidence, 0)
  FROM geocode_cache
  WHERE address_key = $1
  ```
  Parses WKT "POINT(lng lat)" geometry to Result; returns false on no rows.
- **Store(ctx, key, result)** — upserts via:
  ```sql
  INSERT INTO geocode_cache (address_key, geom, precision, provider, confidence)
  VALUES ($1, ST_SetSRID(ST_MakePoint($2, $3), 4326), $4, $5, $6)
  ON CONFLICT (address_key) DO UPDATE
      SET geom=EXCLUDED.geom, precision=EXCLUDED.precision, provider=EXCLUDED.provider,
          confidence=EXCLUDED.confidence, cached_at=now()
  ```
  Stores longitude/latitude as PostGIS POINT with SRID 4326 (WGS84).

**CountyCentroid(county, state)** — utility returning approximate centroid for US county:
```go
func CountyCentroid(county, state string) (Result, bool)
```
Maps "county,state" (case-insensitive) to pre-baked centroids. Idaho counties fully covered; other states minimal. All returned as Precision: "county_centroid", Provider: "builtin", Confidence: 0.5.

---

## Database Schema: `geocode_cache`

**Table** — persists Geocoder results:
```
address_key TEXT PRIMARY KEY     -- normalized cache key
geom        GEOMETRY(Point, 4326) -- PostGIS point (longitude, latitude)
precision   TEXT                 -- rooftop|street|locality|county_centroid
provider    TEXT                 -- google|fake|builtin
confidence  FLOAT8              -- optional, NULL allowed
cached_at   TIMESTAMP           -- auto-updated on upsert
created_at  TIMESTAMP           -- auto-set on insert
```

**Key Design:** address_key is primary key for upsert efficiency. Geom stored as PostGIS Point for potential spatial queries (e.g., "nearest listings" searches).

---

## Impact Callouts

### ⚠ **Result struct** (`geocode.go:19–25`)
Changing Result field names, types, or tags will break:
- **PGStore.Lookup()** — must parse Scan result into matching fields (`geocode.go:30–42`)
- **PGStore.Store()** — must Exec with values in matching order (`pgstore.go:57`)
- **CachingGeocoder.Geocode()** — returns result downstream; callers expect field names
- **Listing ingestion** — receives Result and stores coordinates in listing tables
- All tests expecting Result shape (FakeGeocoder hardcodes values; tests may assert fields)

### ⚠ **Precision enum** (`geocode.go:8–16`)
Changing constant values or adding new precision levels requires:
- **PGStore.Store() SQL** — precision stored as TEXT; no validation; arbitrary string OK but convention matters
- **PGStore.Lookup() Scan** — parses TEXT → Precision; casting new values requires no DB change
- **CountyCentroid map** — uses PrecisionCountyCentroid constant; changing constant name breaks references (`county.go:20`)
- **FakeGeocoder** — hardcodes PrecisionRooftop; changing constant name breaks (`fake.go:23`)
- **Downstream** — any code checking `if r.Precision == geocode.PrecisionRooftop` must be updated

### ⚠ **CacheStore interface** (`cache.go:11–14`)
Adding or changing Lookup/Store signatures affects:
- **CachingGeocoder.Geocode()** — calls both methods; signature changes propagate immediately (`cache.go:42, 54, 60`)
- **MemStore** — must implement new signature (`cache.go:86–98`)
- **PGStore** — must implement new signature (`pgstore.go:25, 47`)
- Any other CacheStore implementation (e.g., Redis wrapper)

### ⚠ **Geocoder interface** (`geocode.go:28–30`)
Changing Geocode signature (parameters or return types) requires:
- **CachingGeocoder** — wraps and calls inner.Geocode; signature change propagates (`cache.go:51`)
- **FakeGeocoder** — implements interface; must match signature (`fake.go:16`)
- **All callers** — any code calling Geocode (listing ingestion handler) must adapt

### ⚠ **CachingGeocoder daily limit logic** (`cache.go:46–48`)
Daily limit is enforced via in-memory atomic counter:
- **ResetDailyCount()** called at midnight by orchestrator; forgetting breaks quota across restarts
- **dailyUsed.Add(1)** increments before calling inner geocoder; on limit exceeded, counter is decremented. Caller must not retry immediately (will keep incrementing/decrementing).
- **County fallback** skips inner geocoder entirely; does not consume daily quota

### ⚠ **geocode_cache table schema** (database)
Changing columns or constraints requires:
- **PGStore.Lookup() SQL** — hardcoded column list; missing columns cause Scan errors (`pgstore.go:26–28`)
- **PGStore.Store() SQL** — hardcoded INSERT/UPDATE column list (`pgstore.go:49–56`)
- **Migration script** — must create/alter table before PGStore first call

### ⚠ **WKT point parsing** (`pgstore.go:64–78`)
PGStore assumes PostGIS returns geometry as WKT "POINT(lng lat)" text. If PostGIS output format changes (e.g., to binary), parseWKTPoint will fail. Currently strict: fails on unexpected WKT format.

### ⚠ **CountyCentroid map** (`county.go:18–64`)
Statically defined centroid dataset:
- Used as last-resort fallback when inner Geocoder fails
- Changing constant values or adding counties requires code edit + recompile
- Missing counties will cause Geocode to return error even with fallback enabled
- All Idaho counties covered; other states minimal — consider expanding for production use

---

## Data Flow

1. **Handler** calls `CachingGeocoder.Geocode(ctx, address, city, county, state)`
2. **CachingGeocoder** normalizes key → `normalizeKey()`, queries CacheStore (e.g., PGStore)
3. **Cache hit** → returns Result immediately
4. **Cache miss** → increments daily counter; calls inner Geocoder (e.g., Google API)
5. **Geocoder success** → CachingGeocoder stores Result in CacheStore, returns to handler
6. **Geocoder error** → CachingGeocoder attempts CountyCentroid fallback:
   - If county+state found in map → stores centroid, returns it
   - If not found → propagates error to handler
7. **Handler** receives Result or error; on success, inserts coordinates into listing record

---

## Key Behaviors

1. **Stateless cache key** — normalized address components, no listing ID or context
2. **County centroid fallback** — provides graceful degradation when precise geocoding fails (e.g., rural addresses, API quota)
3. **Daily limit** — in-memory counter resets at midnight; quota applies only to real API calls, not cache hits or fallbacks
4. **PostGIS storage** — enables future spatial queries (e.g., "listings within X km of point")
5. **Upsert semantics** — repeated geocoding of same address overwrites cached_at timestamp but preserves historical precision/provider for audit
