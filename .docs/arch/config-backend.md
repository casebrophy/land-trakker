# Config Backend System

> Centralized bootstrap configuration for land_trakker. Loads TOML files with environment variable overrides across six sections: server, database, geocoding, LLM, scraper, and backup. Precedence: TOML defaults → environment variables.

## Core Types

```go
// Config represents the complete bootstrap configuration for land_trakker.
type Config struct {
    Server   ServerConfig   `toml:"server"`
    Database DatabaseConfig `toml:"database"`
    Geocoding GeocodingConfig `toml:"geocoding"`
    LLM      LLMConfig      `toml:"llm"`
    Scraper  ScraperConfig  `toml:"scraper"`
    Backup   BackupConfig   `toml:"backup"`
}

// ServerConfig represents server-related configuration.
type ServerConfig struct {
    Listen           string `toml:"listen"`
    AdminPasswordHash string `toml:"admin_password_hash"`
    SessionSecret    string `toml:"session_secret"`
}

// DatabaseConfig represents database-related configuration.
type DatabaseConfig struct {
    URL string `toml:"url"`
}

// GeocodingConfig represents geocoding provider configuration.
type GeocodingConfig struct {
    Provider          string `toml:"provider"`
    APIKey            string `toml:"api_key"`
    DailyRequestLimit int    `toml:"daily_request_limit"`
}

// LLMConfig represents LLM (Claude API) configuration.
type LLMConfig struct {
    Enabled          bool   `toml:"enabled"`
    APIKey           string `toml:"api_key"`
    Model            string `toml:"model"`
    DailyCallLimit   int    `toml:"daily_call_limit"`
    MonthlyCallLimit int    `toml:"monthly_call_limit"`
}

// ScraperConfig represents default scraper configuration.
type ScraperConfig struct {
    DefaultUserAgent   string `toml:"default_user_agent"`
    DefaultRateLimitMS int    `toml:"default_rate_limit_ms"`
    DefaultConcurrency int    `toml:"default_concurrency"`
}

// BackupConfig represents backup-related configuration.
type BackupConfig struct {
    DailyDir        string `toml:"daily_dir"`
    RetentionDaily  int    `toml:"retention_daily"`
    RetentionWeekly int    `toml:"retention_weekly"`
    RetentionMonthly int    `toml:"retention_monthly"`
}
```

## File Map

### Loaders
- `foundation/config/config.go` — **Load()** — reads TOML file, unmarshals to Config struct, applies env overrides

### Helpers
- `foundation/config/config.go` — **applyEnvOverrides()** — iterates sections, applies `SECTION_KEY` env vars
- `foundation/config/config.go` — **parseIntEnv()** — parses integer env values, returns -1 on failure (silently ignored)

### Tests
- `foundation/config/config_test.go` — validates Load(), env override mechanics, error cases

### Testdata
- `foundation/config/testdata/land_trakker.toml` — reference TOML with all six sections populated

## Impact Callouts

### ⚠ ServerConfig
Changing this struct affects:
- Server startup code — must read `.Listen`, `.AdminPasswordHash`, `.SessionSecret`
- Authentication handlers — must validate requests against `.AdminPasswordHash`
- Session middleware — must use `.SessionSecret` for HMAC/encryption

Environment override: `SERVER_LISTEN`, `SERVER_ADMIN_PASSWORD_HASH`, `SERVER_SESSION_SECRET`

### ⚠ DatabaseConfig
Changing URL field affects:
- Database connection pool initialization — must parse `.URL` as connection string
- Migration runner — passes `.URL` directly to database driver

Environment override: `DATABASE_URL`

### ⚠ GeocodingConfig
Changing this struct affects:
- Geocoding service initialization — selects provider by `.Provider`, uses `.APIKey`
- Request limiting logic — enforces `.DailyRequestLimit` via rate limiter
- Provider fallback routing — Provider field gates which API client is instantiated

Environment overrides: `GEOCODING_PROVIDER`, `GEOCODING_API_KEY`, `GEOCODING_DAILY_REQUEST_LIMIT`

### ⚠ LLMConfig
Changing this struct affects:
- LLM client initialization — gates creation on `.Enabled`, uses `.APIKey`, `.Model`
- Call limiting — enforces `.DailyCallLimit` and `.MonthlyCallLimit` at request time
- Model selection — `.Model` value is passed to Claude API client at runtime

Environment overrides: `LLM_ENABLED`, `LLM_API_KEY`, `LLM_MODEL`, `LLM_DAILY_CALL_LIMIT`, `LLM_MONTHLY_CALL_LIMIT`

### ⚠ ScraperConfig
Changing this struct affects:
- Scraper instance defaults — `.DefaultUserAgent`, `.DefaultRateLimitMS`, `.DefaultConcurrency` seed scraper factories
- HTTP client setup — User-Agent header set from `.DefaultUserAgent`
- Worker pool — concurrency defaults to `.DefaultConcurrency`

Environment overrides: `SCRAPER_DEFAULT_USER_AGENT`, `SCRAPER_DEFAULT_RATE_LIMIT_MS`, `SCRAPER_DEFAULT_CONCURRENCY`

### ⚠ BackupConfig
Changing this struct affects:
- Backup directory creation — `.DailyDir` path where backups are stored
- Retention policy enforcement — `.RetentionDaily`, `.RetentionWeekly`, `.RetentionMonthly` control deletion schedule

Environment overrides: `BACKUP_DAILY_DIR`, `BACKUP_RETENTION_DAILY`, `BACKUP_RETENTION_WEEKLY`, `BACKUP_RETENTION_MONTHLY`

## Environment Variable Precedence

1. TOML file loaded first (foundation/config/testdata/land_trakker.toml pattern)
2. Environment variables override TOML values if set (non-empty string check)
3. Integer fields: invalid env values silently ignored (parseIntEnv returns -1, check rejects)
4. Boolean fields (LLM.Enabled): truthy check (`"true"`, `"1"`, `"yes"`)
5. Empty env vars do not override — TOML values remain

## Cross-Domain Dependencies

- **Outbound:** Uses BurntSushi/toml for unmarshaling, no internal package dependencies
- **Inbound:** All foundation services (geocode, llm, scraper, storage, web, parser) consume config at startup via Load()

## Configuration Source

Default/example: `foundation/config/testdata/land_trakker.toml`
- Server: listen `:8080`, placeholder hashes/secrets
- Database: PostgreSQL connection string
- Geocoding: Mapbox provider, test API key, 5000 req/day limit
- LLM: Claude 3.5 Sonnet enabled, 100 daily / 5000 monthly calls
- Scraper: 1s rate limit, 4 concurrent workers, standard user agent
- Backup: /var/backups/daily, 14 day / 8 week / 12 month retention
