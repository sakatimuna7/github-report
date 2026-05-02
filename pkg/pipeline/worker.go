package pipeline

import (
	"context"
	"fmt"
	"sync"
)

// MapFn is the function signature for summarizing a single chunk.
// It receives the chunk text and returns the summary or an error.
type MapFn func(ctx context.Context, chunkData string) (string, error)

// MapResult holds the output of one MAP worker.
type MapResult struct {
	Idx  int
	Text string
	Err  error
}

// WorkerPool runs MapFn across chunks with bounded concurrency.
// It supports context cancellation and per-chunk caching.
type WorkerPool struct {
	Concurrency int          // max goroutines (0 = len(chunks))
	Cache       SummaryCache // optional: skip LLM if cached
}

// NewWorkerPool creates a WorkerPool with the given concurrency limit.
// If concurrency <= 0, it defaults to the number of chunks (unbounded).
func NewWorkerPool(concurrency int, cache SummaryCache) *WorkerPool {
	return &WorkerPool{
		Concurrency: concurrency,
		Cache:       cache,
	}
}

// Run executes fn for each chunk using a bounded worker pool.
// Results are returned in the original chunk order.
// If a cache is provided, cached summaries skip the LLM call entirely.
// The function respects context cancellation.
func (wp *WorkerPool) Run(ctx context.Context, chunks []string, fn MapFn) []MapResult {
	n := len(chunks)
	if n == 0 {
		return nil
	}

	results := make([]MapResult, n)
	sem := make(chan struct{}, wp.concurrency(n))
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		// Check context before launching
		select {
		case <-ctx.Done():
			// Fill remaining with context error
			for j := i; j < n; j++ {
				results[j] = MapResult{Idx: j, Err: ctx.Err()}
			}
			return results
		default:
		}

		wg.Add(1)
		sem <- struct{}{} // acquire slot

		go func(idx int, data string) {
			defer wg.Done()
			defer func() { <-sem }() // release slot

			// Check cache first
			if wp.Cache != nil {
				key := ContentHash(data)
				if cached, ok := wp.Cache.Get(key); ok {
					results[idx] = MapResult{Idx: idx, Text: cached}
					return
				}
			}

			// Check context again inside goroutine
			select {
			case <-ctx.Done():
				results[idx] = MapResult{Idx: idx, Err: ctx.Err()}
				return
			default:
			}

			res, err := fn(ctx, data)
			if err != nil {
				results[idx] = MapResult{Idx: idx, Err: fmt.Errorf("chunk %d: %w", idx+1, err)}
				return
			}

			results[idx] = MapResult{Idx: idx, Text: res}

			// Store in cache
			if wp.Cache != nil {
				wp.Cache.Set(ContentHash(data), res)
			}
		}(i, chunk)
	}

	wg.Wait()
	return results
}

// concurrency returns the effective concurrency limit.
func (wp *WorkerPool) concurrency(total int) int {
	if wp.Concurrency <= 0 || wp.Concurrency > total {
		return total
	}
	return wp.Concurrency
}

// CollectSuccessful extracts successful summaries in order, and returns aggregated errors.
func CollectSuccessful(results []MapResult) (summaries []string, errors []error) {
	for _, r := range results {
		if r.Err != nil {
			errors = append(errors, r.Err)
			continue
		}
		if r.Text != "" {
			summaries = append(summaries, r.Text)
		}
	}
	return
}
