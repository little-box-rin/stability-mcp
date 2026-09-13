package client

import (
	"math"
	"math/rand"
	"net/http"
	"time"
)

const (
	DefaultMaxRetries = 3
	BaseDelay         = 500 * time.Millisecond
	MaxDelay          = 30 * time.Second
)

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxRetries int           // 0 = no retries
	BaseDelay  time.Duration // first backoff duration
	MaxDelay   time.Duration // cap for backoff
}

// DefaultRetryConfig returns sensible defaults: MaxRetries=3, BaseDelay=500ms, MaxDelay=30s
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: DefaultMaxRetries,
		BaseDelay:  BaseDelay,
		MaxDelay:   MaxDelay,
	}
}

// shouldRetry checks if an HTTP response or error should trigger a retry.
// Retry on connection errors, HTTP 429 (rate limit), and HTTP 5xx (server errors).
func shouldRetry(err error, resp *http.Response) bool {
	if err != nil {
		return true // connection errors
	}
	if resp == nil {
		return false
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}
	return resp.StatusCode >= 500 && resp.StatusCode <= 599
}

// backoff calculates delay for retry attempt (1-indexed) with full jitter.
// delay = min(MaxDelay, BaseDelay * 2^(attempt-1)) * random[0.5, 1.5)
func backoff(attempt int, cfg RetryConfig) time.Duration {
	base := cfg.BaseDelay
	if base <= 0 {
		base = BaseDelay
	}
	max := cfg.MaxDelay
	if max <= 0 {
		max = MaxDelay
	}
	exp := float64(base) * math.Pow(2, float64(attempt-1))
	if exp > float64(max) {
		exp = float64(max)
	}
	// jitter in [0.5, 1.5)
	jitter := 0.5 + rand.Float64()
	d := time.Duration(exp * jitter)
	if d > max {
		d = max
	}
	if d < 0 {
		d = 0
	}
	return d
}