package probe

import (
	"context"
	"sync"

	"probeHTTP/internal/output"
)

// ProcessURLs processes URLs concurrently using a worker pool with context support
func (p *Prober) ProcessURLs(ctx context.Context, urls []string, originalInputMap map[string]string, concurrency int) <-chan output.ProbeResult {
	results := make(chan output.ProbeResult, len(urls))
	urlChan := make(chan string, len(urls))

	// Create worker pool
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go p.worker(ctx, urlChan, results, originalInputMap, &wg)
	}

	// Send URLs to workers
	go func() {
		for _, url := range urls {
			select {
			case urlChan <- url:
			case <-ctx.Done():
				break
			}
		}
		close(urlChan)
	}()

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// worker processes URLs from the channel
func (p *Prober) worker(ctx context.Context, urls <-chan string, results chan<- output.ProbeResult, originalInputMap map[string]string, wg *sync.WaitGroup) {
	defer wg.Done()

	for expandedURL := range urls {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return
		default:
		}

		originalInput := originalInputMap[expandedURL]
		result := p.ProbeURL(ctx, expandedURL, originalInput)
		results <- result
	}
}
