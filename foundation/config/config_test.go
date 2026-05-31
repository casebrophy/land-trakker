package config

import (
	"path/filepath"
	"testing"
)

func TestLoad_ValidTOML(t *testing.T) {
	configPath := filepath.Join("testdata", "land_trakker.toml")
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.Server.Listen != ":8080" {
		t.Errorf("Server.Listen = %q, want %q", cfg.Server.Listen, ":8080")
	}
	if cfg.Server.AdminPasswordHash == "" {
		t.Error("Server.AdminPasswordHash is empty")
	}
	if cfg.Database.URL == "" {
		t.Error("Database.URL is empty")
	}
	if cfg.Geocoding.Provider != "mapbox" {
		t.Errorf("Geocoding.Provider = %q, want %q", cfg.Geocoding.Provider, "mapbox")
	}
	if !cfg.LLM.Enabled {
		t.Error("LLM.Enabled is false, want true")
	}
	if cfg.Scraper.DefaultRateLimitMS != 1000 {
		t.Errorf("Scraper.DefaultRateLimitMS = %d, want %d", cfg.Scraper.DefaultRateLimitMS, 1000)
	}
	if cfg.Backup.RetentionDaily != 14 {
		t.Errorf("Backup.RetentionDaily = %d, want %d", cfg.Backup.RetentionDaily, 14)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/config.toml")
	if err == nil {
		t.Error("Load() succeeded for missing file, want error")
	}
}

func TestLoad_EnvOverridesString(t *testing.T) {
	configPath := filepath.Join("testdata", "land_trakker.toml")
	t.Setenv("DATABASE_URL", "postgres://override:pass@host:5432/override_db?sslmode=disable")
	t.Setenv("GEOCODING_API_KEY", "pk.override.token")
	t.Setenv("LLM_API_KEY", "sk-ant-override-key")
	t.Setenv("SCRAPER_DEFAULT_USER_AGENT", "custom-bot/1.0")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Database.URL != "postgres://override:pass@host:5432/override_db?sslmode=disable" {
		t.Errorf("Database.URL override failed")
	}
	if cfg.Geocoding.APIKey != "pk.override.token" {
		t.Errorf("Geocoding.APIKey override failed")
	}
	if cfg.LLM.APIKey != "sk-ant-override-key" {
		t.Errorf("LLM.APIKey override failed")
	}
	if cfg.Scraper.DefaultUserAgent != "custom-bot/1.0" {
		t.Errorf("Scraper.DefaultUserAgent override failed")
	}
}

func TestLoad_EnvOverridesInt(t *testing.T) {
	configPath := filepath.Join("testdata", "land_trakker.toml")
	t.Setenv("GEOCODING_DAILY_REQUEST_LIMIT", "10000")
	t.Setenv("LLM_DAILY_CALL_LIMIT", "500")
	t.Setenv("SCRAPER_DEFAULT_RATE_LIMIT_MS", "2000")
	t.Setenv("BACKUP_RETENTION_DAILY", "30")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Geocoding.DailyRequestLimit != 10000 {
		t.Errorf("Geocoding.DailyRequestLimit = %d, want %d", cfg.Geocoding.DailyRequestLimit, 10000)
	}
	if cfg.LLM.DailyCallLimit != 500 {
		t.Errorf("LLM.DailyCallLimit = %d, want %d", cfg.LLM.DailyCallLimit, 500)
	}
	if cfg.Scraper.DefaultRateLimitMS != 2000 {
		t.Errorf("Scraper.DefaultRateLimitMS = %d, want %d", cfg.Scraper.DefaultRateLimitMS, 2000)
	}
	if cfg.Backup.RetentionDaily != 30 {
		t.Errorf("Backup.RetentionDaily = %d, want %d", cfg.Backup.RetentionDaily, 30)
	}
}

func TestLoad_EnvOverridesBool(t *testing.T) {
	configPath := filepath.Join("testdata", "land_trakker.toml")
	t.Setenv("LLM_ENABLED", "false")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.LLM.Enabled {
		t.Error("LLM.Enabled = true, want false after env override")
	}
}

func TestLoad_EnvOverridesInvalidInt_IgnoresInvalidValue(t *testing.T) {
	configPath := filepath.Join("testdata", "land_trakker.toml")
	originalValue := 1000

	t.Setenv("SCRAPER_DEFAULT_RATE_LIMIT_MS", "not_a_number")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Scraper.DefaultRateLimitMS != originalValue {
		t.Errorf("Scraper.DefaultRateLimitMS = %d, want %d", cfg.Scraper.DefaultRateLimitMS, originalValue)
	}
}

func TestLoad_ServerConfigOverrides(t *testing.T) {
	configPath := filepath.Join("testdata", "land_trakker.toml")
	t.Setenv("SERVER_LISTEN", ":9000")
	t.Setenv("SERVER_ADMIN_PASSWORD_HASH", "$2a$12$new_hash_here")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Server.Listen != ":9000" {
		t.Errorf("Server.Listen = %q, want %q", cfg.Server.Listen, ":9000")
	}
}

func TestLoad_MultipleOverrides(t *testing.T) {
	configPath := filepath.Join("testdata", "land_trakker.toml")

	t.Setenv("DATABASE_URL", "postgres://new:pass@newhost:5432/newdb?sslmode=disable")
	t.Setenv("GEOCODING_PROVIDER", "google")
	t.Setenv("LLM_ENABLED", "false")
	t.Setenv("SCRAPER_DEFAULT_CONCURRENCY", "8")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Database.URL != "postgres://new:pass@newhost:5432/newdb?sslmode=disable" {
		t.Error("DATABASE_URL override failed")
	}
	if cfg.Geocoding.Provider != "google" {
		t.Error("GEOCODING_PROVIDER override failed")
	}
	if cfg.LLM.Enabled {
		t.Error("LLM_ENABLED override failed")
	}
	if cfg.Scraper.DefaultConcurrency != 8 {
		t.Error("SCRAPER_DEFAULT_CONCURRENCY override failed")
	}
}

func isValidPostgresURL(url string) bool {
	return len(url) > 0 && url[:11] == "postgres://"
}
