package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for stability-mcp.
type Config struct {
	APIKey      string
	OutputDir   string
	Concurrency int
	RateLimit   int // max requests per 10s window
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	apiKey := os.Getenv("STABILITY_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("STABILITY_API_KEY environment variable is required")
	}
	outputDir := os.Getenv("STABILITY_OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "./stability-images"
	}
	concurrency := 10
	if v := os.Getenv("STABILITY_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			concurrency = n
		}
	}
	rateLimit := 150
	if v := os.Getenv("STABILITY_RATE_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rateLimit = n
		}
	}
	return &Config{
		APIKey:      apiKey,
		OutputDir:   outputDir,
		Concurrency: concurrency,
		RateLimit:   rateLimit,
	}, nil
}