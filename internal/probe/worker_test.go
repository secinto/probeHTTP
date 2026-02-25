package probe

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"probeHTTP/internal/config"
)

func TestProcessURLs_CancelledContextReturnsPromptly(t *testing.T) {
	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	prober := NewProber(cfg)
	defer prober.Close()

	urls := make([]string, 200000)
	originalInputMap := make(map[string]string, len(urls))
	for index := range urls {
		url := "http://example.com"
		urls[index] = url
		originalInputMap[url] = url
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	results := prober.ProcessURLs(ctx, urls, originalInputMap, 4)

	resultCount := 0
	for range results {
		resultCount++
	}

	elapsed := time.Since(start)
	if resultCount != 0 {
		t.Fatalf("expected no results for cancelled context, got %d", resultCount)
	}
	if elapsed > 250*time.Millisecond {
		t.Fatalf("expected prompt return for cancelled context, took %s", elapsed)
	}
}
