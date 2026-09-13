package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration for stability-mcp.
type Config struct {
	APIKey      string `json:"api_key" yaml:"api_key"`
	OutputDir   string `json:"output_dir" yaml:"output_dir"`
	Concurrency int    `json:"concurrency" yaml:"concurrency"`
	RateLimit   int    `json:"rate_limit" yaml:"rate_limit"`
	RatePeriod  int    `json:"rate_period" yaml:"rate_period"`
	MaxRetries  int    `json:"max_retries" yaml:"max_retries"`
	LogLevel    string `json:"log_level" yaml:"log_level"`
}

// defaultConfig returns the default configuration values.
func defaultConfig() *Config {
	return &Config{
		OutputDir:   "./stability-images",
		Concurrency: 10,
		RateLimit:   150,
		RatePeriod:  10,
		MaxRetries:  3,
		LogLevel:    "info",
	}
}

// Load reads configuration from a config file (if found) and environment
// variables. Config file values act as defaults; environment variables
// override them when present.
func Load() (*Config, error) {
	cfg := defaultConfig()

	path, err := findConfigFile()
	if err != nil {
		return nil, err
	}
	if path != "" {
		if err := loadFile(cfg, path); err != nil {
			return nil, err
		}
	}

	// Environment variables override config file values.
	if v := os.Getenv("STABILITY_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("OUTPUT_DIR"); v != "" {
		cfg.OutputDir = v
	}
	if v := os.Getenv("CONCURRENCY"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			cfg.Concurrency = n
		}
	}
	if v := os.Getenv("RATE_LIMIT"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			cfg.RateLimit = n
		}
	}
	if v := os.Getenv("RATE_PERIOD"); v != "" {
		if n, _ := strconv.Atoi(v); n > 0 {
			cfg.RatePeriod = n
		}
	}
	if v := os.Getenv("MAX_RETRIES"); v != "" {
		if n, _ := strconv.Atoi(v); n >= 0 {
			cfg.MaxRetries = n
		}
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	// Sanitize: zero or negative values fall back to safe defaults.
	if cfg.Concurrency < 1 {
		cfg.Concurrency = 10
	}
	if cfg.RateLimit < 1 {
		cfg.RateLimit = 150
	}
	if cfg.RatePeriod < 1 {
		cfg.RatePeriod = 10
	}
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 3
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = "./stability-images"
	}
	if cfg.LogLevel == "" {
		cfg.LogLevel = "info"
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required: set STABILITY_API_KEY or api_key in a config file")
	}

	return cfg, nil
}

// findConfigFile returns the path of the config file to use, if any.
// STABILITY_CONFIG wins; otherwise the first existing file from the
// default search order is returned.
func findConfigFile() (string, error) {
	if p := os.Getenv("STABILITY_CONFIG"); p != "" {
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("config file %s: %w", p, err)
		}
		return p, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	candidates := []string{
		filepath.Join(home, ".stability-mcp.yaml"),
		filepath.Join(home, ".stability-mcp.json"),
		"./stability-mcp.yaml",
		"./stability-mcp.json",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", nil
}

// loadFile parses a YAML or JSON config file into cfg.
func loadFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("reading config file %s: %w", path, err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil
	}
	switch filepath.Ext(path) {
	case ".json":
		if err := json.Unmarshal(data, cfg); err != nil {
			return fmt.Errorf("parsing config file %s: %w", path, err)
		}
	default: // .yaml, .yml
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return fmt.Errorf("parsing config file %s: %w", path, err)
		}
	}
	return nil
}