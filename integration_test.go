package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"probeHTTP/internal/config"
	"probeHTTP/internal/output"
	"probeHTTP/internal/probe"
)

// TestProbeURL_Success tests successful HTTP requests
func TestProbeURL_Success(t *testing.T) {
	cfg := resetConfig()
	cfg.Silent = true

	server := createTestServer(simpleHTMLHandler)
	defer server.Close()

	prober := probe.NewProber(cfg)
	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	// Verify no error
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}

	// Verify basic fields
	assertStringEqual(t, result.Input, server.URL, "input URL")
	assertStringEqual(t, result.URL, server.URL, "final URL")
	assertStringEqual(t, result.Method, "GET", "method")
	assertIntEqual(t, result.StatusCode, 200, "status code")

	// Verify title extraction
	assertStringEqual(t, result.Title, "Test Page", "title")

	// Verify server header
	assertStringEqual(t, result.WebServer, "TestServer/1.0", "web server")

	// Verify content type
	if !strings.Contains(result.ContentType, "text/html") {
		t.Errorf("content type: expected text/html, got %s", result.ContentType)
	}

	// Verify hashes are not empty
	assertNotEmpty(t, result.Hash.BodyMMH3, "body hash")
	assertNotEmpty(t, result.Hash.HeaderMMH3, "header hash")

	// Verify word and line counts
	if result.Words == 0 {
		t.Error("expected non-zero word count")
	}
	if result.Lines == 0 {
		t.Error("expected non-zero line count")
	}

	// Verify timestamp is set
	assertNotEmpty(t, result.Timestamp, "timestamp")

	// Verify time is recorded
	assertNotEmpty(t, result.Time, "response time")
}

// TestProbeURL_StatusCodes tests various HTTP status codes
func TestProbeURL_StatusCodes(t *testing.T) {
	cfg := resetConfig()
	cfg.Silent = true

	tests := []struct {
		name       string
		statusCode int
	}{
		{"OK", http.StatusOK},
		{"Created", http.StatusCreated},
		{"No Content", http.StatusNoContent},
		{"Bad Request", http.StatusBadRequest},
		{"Not Found", http.StatusNotFound},
		{"Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(statusCodeHandler(tt.statusCode))
			defer server.Close()

			prober := probe.NewProber(cfg)
			ctx := context.Background()
			result := prober.ProbeURL(ctx, server.URL, server.URL)

			assertIntEqual(t, result.StatusCode, tt.statusCode, "status code")
		})
	}
}

// TestProbeURL_Redirects tests redirect handling
func TestProbeURL_Redirects(t *testing.T) {
	cfg := resetConfig()
	cfg.FollowRedirects = true
	cfg.Silent = true

	// Create final destination server
	finalServer := createTestServer(simpleHTMLHandler)
	defer finalServer.Close()

	// Create redirect server
	redirectServer := createTestServer(redirectHandler(finalServer.URL))
	defer redirectServer.Close()

	prober := probe.NewProber(cfg)
	ctx := context.Background()
	result := prober.ProbeURL(ctx, redirectServer.URL, redirectServer.URL)

	// Should follow redirect to final server
	assertStringEqual(t, result.URL, redirectServer.URL, "URL field should be original probe URL")
	assertStringEqual(t, result.FinalURL, finalServer.URL, "final URL after redirect")
	assertIntEqual(t, result.StatusCode, 200, "status code")
	assertStringEqual(t, result.Title, "Test Page", "title from final page")

	// Verify redirect chain
	if len(result.ChainStatusCodes) != 2 {
		t.Errorf("expected chain with 2 status codes, got %d: %v", len(result.ChainStatusCodes), result.ChainStatusCodes)
	}
	if len(result.ChainStatusCodes) >= 2 {
		assertIntEqual(t, result.ChainStatusCodes[0], 302, "first status in chain (redirect)")
		assertIntEqual(t, result.ChainStatusCodes[1], 200, "second status in chain (final)")
	}
}

// TestProbeURL_NoFollowRedirects tests redirect handling when disabled
func TestProbeURL_NoFollowRedirects(t *testing.T) {
	cfg := resetConfig()
	cfg.FollowRedirects = false
	cfg.Silent = true

	// Create final destination server
	finalServer := createTestServer(simpleHTMLHandler)
	defer finalServer.Close()

	// Create redirect server
	redirectServer := createTestServer(redirectHandler(finalServer.URL))
	defer redirectServer.Close()

	prober := probe.NewProber(cfg)
	ctx := context.Background()
	result := prober.ProbeURL(ctx, redirectServer.URL, redirectServer.URL)

	// Should not follow redirect
	assertStringEqual(t, result.URL, redirectServer.URL, "should stay at redirect URL")
	assertStringEqual(t, result.FinalURL, redirectServer.URL, "final URL should be same as redirect URL")
	assertIntEqual(t, result.StatusCode, 302, "should have redirect status code")

	// Verify redirect chain contains only the redirect status
	if len(result.ChainStatusCodes) != 1 {
		t.Errorf("expected chain with 1 status code, got %d: %v", len(result.ChainStatusCodes), result.ChainStatusCodes)
	}
	if len(result.ChainStatusCodes) >= 1 {
		assertIntEqual(t, result.ChainStatusCodes[0], 302, "status in chain (redirect not followed)")
	}
}

// TestProbeURL_MaxRedirects tests maximum redirect limit
func TestProbeURL_MaxRedirects(t *testing.T) {
	cfg := resetConfig()
	cfg.FollowRedirects = true
	cfg.MaxRedirects = 3
	cfg.Silent = true

	// Create a chain of redirects longer than max
	finalServer := createTestServer(simpleHTMLHandler)
	defer finalServer.Close()

	redirectServer := createTestServer(multiRedirectHandler(5, simpleHTMLHandler))
	defer redirectServer.Close()

	prober := probe.NewProber(cfg)
	ctx := context.Background()
	result := prober.ProbeURL(ctx, redirectServer.URL, redirectServer.URL)

	// Should fail due to too many redirects
	if result.Error == "" {
		t.Error("expected error due to max redirects exceeded")
	}
	if !strings.Contains(result.Error, "redirect") {
		t.Errorf("expected redirect error, got: %s", result.Error)
	}
}

// TestProbeURL_Timeout tests request timeout
func TestProbeURL_Timeout(t *testing.T) {
	cfg := resetConfig()
	cfg.Timeout = 1 // 1 second timeout
	cfg.Silent = true

	// Create server that delays longer than timeout
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	})
	defer server.Close()

	prober := probe.NewProber(cfg)
	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	// Should timeout
	if result.Error == "" {
		t.Error("expected timeout error")
	}
	if !strings.Contains(result.Error, "Request failed") {
		t.Errorf("expected request failed error, got: %s", result.Error)
	}
}

// TestProbeURL_InvalidURL tests invalid URL handling
func TestProbeURL_InvalidURL(t *testing.T) {
	cfg := resetConfig()
	cfg.Silent = true

	tests := []struct {
		name string
		url  string
	}{
		{"invalid scheme", "htp://invalid.com"},
		{"no host", "http://"},
		{"malformed", "ht!tp://bad-url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prober := probe.NewProber(cfg)
			ctx := context.Background()
			result := prober.ProbeURL(ctx, tt.url, tt.url)

			// Should have an error
			if result.Error == "" {
				t.Errorf("expected error for invalid URL: %s", tt.url)
			}
		})
	}
}

// TestProbeURL_URLWithoutScheme tests URL scheme defaulting
func TestProbeURL_URLWithoutScheme(t *testing.T) {
	cfg := resetConfig()
	cfg.Silent = true

	server := createTestServer(simpleHTMLHandler)
	defer server.Close()

	// Extract host:port from server URL
	serverURL := server.URL
	host := strings.TrimPrefix(serverURL, "http://")

	prober := probe.NewProber(cfg)
	ctx := context.Background()
	result := prober.ProbeURL(ctx, host, host)

	// Should add http:// scheme and succeed
	assertStringEqual(t, result.Input, host, "input should be preserved")
	assertStringEqual(t, result.Scheme, "http", "should default to http scheme")
}

// TestProbeURL_ContentAnalysis tests content extraction and analysis
func TestProbeURL_ContentAnalysis(t *testing.T) {
	cfg := resetConfig()
	cfg.Silent = true

	// Create server with specific content
	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("Server", "CustomServer/2.0")
		w.WriteHeader(http.StatusOK)
		content := `<!DOCTYPE html>
<html>
<head><title>Content Analysis Test</title></head>
<body>
<p>Word one two three.</p>
<p>Line two content here.</p>
<p>Line three more words.</p>
</body>
</html>`
		fmt.Fprint(w, content)
	})
	defer server.Close()

	prober := probe.NewProber(cfg)
	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	// Verify title
	assertStringEqual(t, result.Title, "Content Analysis Test", "title")

	// Verify server
	assertStringEqual(t, result.WebServer, "CustomServer/2.0", "server")

	// Verify content analysis
	if result.Words == 0 {
		t.Error("expected non-zero word count")
	}
	if result.Lines == 0 {
		t.Error("expected non-zero line count")
	}

	// Verify content length
	if result.ContentLength == 0 {
		t.Error("expected non-zero content length")
	}

	// Verify hashes
	assertNotEmpty(t, result.Hash.BodyMMH3, "body hash")
	assertNotEmpty(t, result.Hash.HeaderMMH3, "header hash")
}

// TestProbeURL_HashConsistency tests that hashes are consistent
func TestProbeURL_HashConsistency(t *testing.T) {
	cfg := resetConfig()
	cfg.Silent = true

	server := createTestServer(simpleHTMLHandler)
	defer server.Close()

	prober := probe.NewProber(cfg)
	ctx := context.Background()

	// Probe same URL twice
	result1 := prober.ProbeURL(ctx, server.URL, server.URL)
	result2 := prober.ProbeURL(ctx, server.URL, server.URL)

	// Hashes should be identical for same content
	assertStringEqual(t, result1.Hash.BodyMMH3, result2.Hash.BodyMMH3, "body hash consistency")
	assertStringEqual(t, result1.Hash.HeaderMMH3, result2.Hash.HeaderMMH3, "header hash consistency")
}

// TestProbeURL_PortExtraction tests port extraction from URLs
func TestProbeURL_PortExtraction(t *testing.T) {
	cfg := resetConfig()
	cfg.Silent = true

	tests := []struct {
		name         string
		setupHandler http.HandlerFunc
		wantPort     string
	}{
		{
			name:         "HTTP default port",
			setupHandler: simpleHTMLHandler,
			wantPort:     "80", // Will be overridden by test server's actual port
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := createTestServer(tt.setupHandler)
			defer server.Close()

			prober := probe.NewProber(cfg)
			ctx := context.Background()
			result := prober.ProbeURL(ctx, server.URL, server.URL)

			// Port should be extracted from server URL
			assertNotEmpty(t, result.Port, "port")
		})
	}
}

// TestProbeURL_PathExtraction tests path extraction from URLs
func TestProbeURL_PathExtraction(t *testing.T) {
	cfg := resetConfig()
	cfg.Silent = true

	server := createTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Path: %s", r.URL.Path)
	})
	defer server.Close()

	tests := []struct {
		name     string
		path     string
		wantPath string
	}{
		{"root", "/", "/"},
		{"simple path", "/test", "/test"},
		{"nested path", "/api/v1/users", "/api/v1/users"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prober := probe.NewProber(cfg)
			ctx := context.Background()
			url := server.URL + tt.path
			result := prober.ProbeURL(ctx, url, url)

			assertStringEqual(t, result.Path, tt.wantPath, "path")
		})
	}
}

// TestProbeURL_HostExtraction tests host extraction from URLs
func TestProbeURL_HostExtraction(t *testing.T) {
	cfg := resetConfig()
	cfg.Silent = true

	server := createTestServer(simpleHTMLHandler)
	defer server.Close()

	prober := probe.NewProber(cfg)
	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	// Host should be localhost or 127.0.0.1 from test server
	if result.Host != "127.0.0.1" && result.Host != "localhost" && !strings.HasPrefix(result.Host, "127.0.0.1") {
		t.Logf("Note: Host is %s (expected localhost or 127.0.0.1, but test server may use other addresses)", result.Host)
	}
	assertNotEmpty(t, result.Host, "host")
}

// TestWorker tests the worker function
func TestWorker(t *testing.T) {
	cfg := resetConfig()
	cfg.Silent = true

	server := createTestServer(simpleHTMLHandler)
	defer server.Close()

	urlChan := make(chan string, 2)
	resultsChan := make(chan output.ProbeResult, 2)

	// Send URLs to worker
	urlChan <- server.URL
	urlChan <- server.URL
	close(urlChan)

	// Start worker
	// We'll manually track completion
	done := make(chan bool)
	go func() {
		prober := probe.NewProber(cfg)
		ctx := context.Background()
		for url := range urlChan {
			result := prober.ProbeURL(ctx, url, url)
			resultsChan <- result
		}
		done <- true
	}()

	// Wait for completion
	<-done
	close(resultsChan)

	// Verify results
	count := 0
	for result := range resultsChan {
		count++
		if result.Error != "" {
			t.Errorf("unexpected error in result: %s", result.Error)
		}
	}

	if count != 2 {
		t.Errorf("expected 2 results, got %d", count)
	}
}
