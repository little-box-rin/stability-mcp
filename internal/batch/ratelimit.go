package batch

import (
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
// It replenishes tokens at a configurable rate and blocks when tokens are
// exhausted.
type RateLimiter struct {
	tokens    int
	maxTokens int
	interval  time.Duration
	mu        sync.Mutex
	cond      *sync.Cond
	stopCh    chan struct{}
}

// NewRateLimiter creates a rate limiter that allows up to maxTokens requests
// per interval (e.g. 150 requests per 10 seconds). Tokens are replenished
// evenly over the interval.
func NewRateLimiter(maxTokens int, interval time.Duration) *RateLimiter {
	r := &RateLimiter{
		tokens:    maxTokens,
		maxTokens: maxTokens,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
	r.cond = sync.NewCond(&r.mu)
	go r.run()
	return r
}

// Stop stops the background replenishment goroutine.
func (r *RateLimiter) Stop() {
	close(r.stopCh)
}

// Wait blocks until a token is available, then consumes one.
func (r *RateLimiter) Wait() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for r.tokens <= 0 {
		r.cond.Wait()
	}
	r.tokens--
}

func (r *RateLimiter) run() {
	ticker := time.NewTicker(r.interval / time.Duration(r.maxTokens))
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.mu.Lock()
			if r.tokens < r.maxTokens {
				r.tokens++
			}
			r.cond.Broadcast()
			r.mu.Unlock()
		case <-r.stopCh:
			return
		}
	}
}