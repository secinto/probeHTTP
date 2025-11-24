package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"probeHTTP/internal/config"
	"probeHTTP/internal/hash"
	"probeHTTP/internal/parser"
	"probeHTTP/internal/probe"
)

// Benchmark hash calculation
func BenchmarkCalculateMMH3(b *testing.B) {
	data := []byte(strings.Repeat("test data with some content ", 100))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.CalculateMMH3(data)
	}
}

// Benchmark header hash calculation
func BenchmarkCalculateHeaderMMH3(b *testing.B) {
	headers := http.Header{
		"Content-Type":  []string{"text/html; charset=utf-8"},
		"Server":        []string{"nginx/1.18.0"},
		"Cache-Control": []string{"no-cache, no-store, must-revalidate"},
		"Set-Cookie":    []string{"session=abc123", "tracking=xyz789"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash.CalculateHeaderMMH3(headers)
	}
}

// Benchmark HTML title extraction
func BenchmarkExtractTitle(b *testing.B) {
	html := `
	<!DOCTYPE html>
	<html>
	<head>
		<title>Example Page Title</title>
		<meta property="og:title" content="OG Title">
		<meta name="twitter:title" content="Twitter Title">
	</head>
	<body>
		<p>Some content</p>
	</body>
	</html>
	`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.ExtractTitle(html)
	}
}

// Benchmark URL parsing
func BenchmarkParseInputURL(b *testing.B) {
	testURLs := []string{
		"example.com",
		"http://example.com:8080/path",
		"https://example.com/path?query=value",
		"subdomain.example.com:9000/api/v1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, url := range testURLs {
			parser.ParseInputURL(url)
		}
	}
}

// Benchmark URL expansion
func BenchmarkExpandURLs(b *testing.B) {
	testCases := []struct {
		url         string
		allSchemes  bool
		ignorePorts bool
		customPorts string
	}{
		{"example.com", false, false, ""},
		{"http://example.com:8080", true, false, ""},
		{"example.com", false, true, ""},
		{"example.com", false, false, "80,443,8080-8082"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range testCases {
			parser.ExpandURLs(tc.url, tc.allSchemes, tc.ignorePorts, tc.customPorts)
		}
	}
}

// Benchmark port list parsing
func BenchmarkParsePortList(b *testing.B) {
	portLists := []string{
		"80,443,8080",
		"8000-8010",
		"80,443,8000-8010,9000",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, portList := range portLists {
			parser.ParsePortList(portList)
		}
	}
}

// Benchmark full URL probe
func BenchmarkProbeURL(b *testing.B) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "TestServer/1.0")
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head><title>Test Page</title></head>
		<body><p>Test content</p></body>
		</html>
		`))
	}))
	defer server.Close()

	// Create config
	cfg := config.New()
	cfg.Silent = true

	// Create prober
	prober := probe.NewProber(cfg)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		prober.ProbeURL(ctx, server.URL, server.URL)
	}
}

// Benchmark concurrent probing with different worker counts
func BenchmarkProcessURLsConcurrent(b *testing.B) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Server", "TestServer/1.0")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><title>Test</title><body>Content</body></html>"))
	}))
	defer server.Close()

	urls := []string{server.URL}
	originalInputMap := map[string]string{server.URL: server.URL}

	workerCounts := []int{1, 5, 10, 20}

	for _, workers := range workerCounts {
		b.Run(fmt.Sprintf("workers_%d", workers), func(b *testing.B) {
			cfg := config.New()
			cfg.Silent = true
			prober := probe.NewProber(cfg)
			ctx := context.Background()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				results := prober.ProcessURLs(ctx, urls, originalInputMap, workers)
				for range results {
					// Consume results
				}
			}
		})
	}
}

// Benchmark word and line counting
func BenchmarkCountWordsAndLines(b *testing.B) {
	text := strings.Repeat("This is a test sentence with multiple words. ", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser.CountWordsAndLines(text)
	}
}

// Benchmark URL validation
func BenchmarkValidateURL(b *testing.B) {
	urls := []string{
		"http://example.com",
		"https://subdomain.example.com:8080/path",
		"http://localhost:8080",
		"http://192.168.1.1",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, url := range urls {
			parser.ValidateURL(url, false)
		}
	}
}
