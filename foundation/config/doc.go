// Package config provides TOML configuration loading for land_trakker.
//
// The Config struct represents the complete bootstrap configuration,
// with sections for server, database, geocoding, LLM, scraper, and backup.
// Configuration is loaded from a TOML file and can be overridden via
// environment variables.
package config
