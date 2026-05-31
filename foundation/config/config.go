package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

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
	Provider           string `toml:"provider"`
	APIKey             string `toml:"api_key"`
	DailyRequestLimit  int    `toml:"daily_request_limit"`
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
	DefaultUserAgent      string `toml:"default_user_agent"`
	DefaultRateLimitMS    int    `toml:"default_rate_limit_ms"`
	DefaultConcurrency    int    `toml:"default_concurrency"`
}

// BackupConfig represents backup-related configuration.
type BackupConfig struct {
	DailyDir          string `toml:"daily_dir"`
	RetentionDaily    int    `toml:"retention_daily"`
	RetentionWeekly   int    `toml:"retention_weekly"`
	RetentionMonthly  int    `toml:"retention_monthly"`
}

// Load reads and parses a TOML configuration file, applying environment variable overrides.
// Environment variable names follow the pattern: SECTION_KEY (e.g., DATABASE_URL, GEOCODING_API_KEY).
func Load(path string) (*Config, error) {
	cfg := &Config{}

	// Read and unmarshal TOML file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	// Apply environment variable overrides
	applyEnvOverrides(cfg)

	return cfg, nil
}

// applyEnvOverrides updates config values from environment variables.
func applyEnvOverrides(cfg *Config) {
	// Server overrides
	if v := os.Getenv("SERVER_LISTEN"); v != "" {
		cfg.Server.Listen = v
	}
	if v := os.Getenv("SERVER_ADMIN_PASSWORD_HASH"); v != "" {
		cfg.Server.AdminPasswordHash = v
	}
	if v := os.Getenv("SERVER_SESSION_SECRET"); v != "" {
		cfg.Server.SessionSecret = v
	}

	// Database overrides
	if v := os.Getenv("DATABASE_URL"); v != "" {
		cfg.Database.URL = v
	}

	// Geocoding overrides
	if v := os.Getenv("GEOCODING_PROVIDER"); v != "" {
		cfg.Geocoding.Provider = v
	}
	if v := os.Getenv("GEOCODING_API_KEY"); v != "" {
		cfg.Geocoding.APIKey = v
	}
	if v := os.Getenv("GEOCODING_DAILY_REQUEST_LIMIT"); v != "" {
		if val := parseIntEnv(v); val >= 0 {
			cfg.Geocoding.DailyRequestLimit = val
		}
	}

	// LLM overrides
	if v := os.Getenv("LLM_ENABLED"); v != "" {
		cfg.LLM.Enabled = v == "true" || v == "1" || v == "yes"
	}
	if v := os.Getenv("LLM_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		cfg.LLM.Model = v
	}
	if v := os.Getenv("LLM_DAILY_CALL_LIMIT"); v != "" {
		if val := parseIntEnv(v); val >= 0 {
			cfg.LLM.DailyCallLimit = val
		}
	}
	if v := os.Getenv("LLM_MONTHLY_CALL_LIMIT"); v != "" {
		if val := parseIntEnv(v); val >= 0 {
			cfg.LLM.MonthlyCallLimit = val
		}
	}

	// Scraper overrides
	if v := os.Getenv("SCRAPER_DEFAULT_USER_AGENT"); v != "" {
		cfg.Scraper.DefaultUserAgent = v
	}
	if v := os.Getenv("SCRAPER_DEFAULT_RATE_LIMIT_MS"); v != "" {
		if val := parseIntEnv(v); val >= 0 {
			cfg.Scraper.DefaultRateLimitMS = val
		}
	}
	if v := os.Getenv("SCRAPER_DEFAULT_CONCURRENCY"); v != "" {
		if val := parseIntEnv(v); val >= 0 {
			cfg.Scraper.DefaultConcurrency = val
		}
	}

	// Backup overrides
	if v := os.Getenv("BACKUP_DAILY_DIR"); v != "" {
		cfg.Backup.DailyDir = v
	}
	if v := os.Getenv("BACKUP_RETENTION_DAILY"); v != "" {
		if val := parseIntEnv(v); val >= 0 {
			cfg.Backup.RetentionDaily = val
		}
	}
	if v := os.Getenv("BACKUP_RETENTION_WEEKLY"); v != "" {
		if val := parseIntEnv(v); val >= 0 {
			cfg.Backup.RetentionWeekly = val
		}
	}
	if v := os.Getenv("BACKUP_RETENTION_MONTHLY"); v != "" {
		if val := parseIntEnv(v); val >= 0 {
			cfg.Backup.RetentionMonthly = val
		}
	}
}

// parseIntEnv parses an integer from a string, returning -1 if parsing fails.
func parseIntEnv(s string) int {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil {
		return -1
	}
	return v
}
