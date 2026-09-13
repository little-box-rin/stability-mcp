package batch

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/little-box-rin/stability-mcp/internal/client"
)

// Engine manages the worker pool and rate limiter for batch generation.
type Engine struct {
	concurrency int
	client      *client.Client
	rateLimiter *RateLimiter
}

// NewEngine creates a new batch engine with the given concurrency, HTTP client,
// and rate limit (requests per rate period).
func NewEngine(concurrency int, cli *client.Client, rateLimit int, ratePeriod time.Duration) *Engine {
	if ratePeriod <= 0 {
		ratePeriod = 10 * time.Second
	}
	return &Engine{
		concurrency: concurrency,
		client:      cli,
		rateLimiter: NewRateLimiter(rateLimit, ratePeriod),
	}
}

// Stop shuts down the rate limiter's background goroutine.
func (e *Engine) Stop() {
	e.rateLimiter.Stop()
}

// GenerateBatch generates images for an array of prompts using the worker pool.
// The base seed is incremented for each prompt. If params.Seed is 0, each
// prompt gets a random seed.
func (e *Engine) GenerateBatch(ctx context.Context, prompts []string, params client.GenerateParams) ([]Result, time.Duration, error) {
	start := time.Now()
	jobs := make(chan Job, len(prompts))
	results := make(chan Result, len(prompts))

	// Feed jobs
	go func() {
		defer close(jobs)
		for i, prompt := range prompts {
			seed := params.Seed
			if seed == 0 {
				seed = rand.Intn(1000000)
			} else {
				seed = seed + i
			}
			job := Job{
				Index:  i,
				Prompt: prompt,
				Seed:   seed,
				Params: params,
			}
			job.Params.Seed = seed

			select {
			case jobs <- job:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Collect results
	collectorDone := make(chan struct{})
	var allResults []Result
	go func() {
		defer close(collectorDone)
		for r := range results {
			allResults = append(allResults, r)
		}
	}()

	// Start workers
	var wg sync.WaitGroup
	workerCount := e.concurrency
	if workerCount > len(prompts) {
		workerCount = len(prompts)
	}
	if workerCount < 1 {
		workerCount = 1
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				// Wait for rate limiter
				e.rateLimiter.Wait()

				// Make the API call
				data, genResult, err := e.client.Generate(job.Params)
				res := Result{
					Index:  job.Index,
					Seed:   job.Seed,
					Data:   data,
					Err:    err,
				}
				if genResult != nil {
					res.Seed = genResult.Seed
					res.FinishReason = genResult.FinishReason
				}
				results <- res
			}
		}()
	}

	// Wait for all workers to finish
	wg.Wait()
	close(results)
	<-collectorDone

	elapsed := time.Since(start)
	return allResults, elapsed, nil
}