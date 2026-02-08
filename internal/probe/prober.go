package probe

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"probeHTTP/internal/config"
	"probeHTTP/internal/hash"
	"probeHTTP/internal/output"
	"probeHTTP/internal/parser"
	"probeHTTP/internal/storage"
	"probeHTTP/pkg/useragent"
)

// Prober handles HTTP probing operations
type Prober struct {
	client *Client
	config *config.Config
	// Mutex for atomic stderr writes when flushing debug buffers
	stderrMutex  sync.Mutex
	cleanupFuncs []func() error
	cleanupMutex sync.Mutex
}

// NewProber creates a new Prober instance
func NewProber(cfg *config.Config) *Prober {
	return &Prober{
		client:       NewClient(cfg),
		config:       cfg,
		cleanupFuncs: make([]func() error, 0),
	}
}

// Close cleans up all resources used by the prober
func (p *Prober) Close() error {
	p.cleanupMutex.Lock()
	defer p.cleanupMutex.Unlock()

	var errs []error
	for _, cleanup := range p.cleanupFuncs {
		if err := cleanup(); err != nil {
			errs = append(errs, err)
		}
	}

	if err := p.client.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// ProbeURL performs the HTTP probe for a single URL with retry support
func (p *Prober) ProbeURL(ctx context.Context, probeURL string, originalInput string) output.ProbeResult {
	// Try with retries
	var result output.ProbeResult
	var lastErr error

	maxAttempts := p.config.MaxRetries + 1
	backoff := 1 * time.Second

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			p.config.Logger.Debug("retrying request",
				"url", probeURL,
				"attempt", attempt+1,
				"backoff", backoff,
			)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				result.Error = "cancelled"
				return result
			}
			backoff *= 2 // Exponential backoff
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}

		result = p.probeURLOnce(ctx, probeURL, originalInput)

		// Don't retry on success or 4xx/5xx status codes (only retry network errors)
		if result.Error == "" || result.StatusCode >= 400 {
			return result
		}

		lastErr = fmt.Errorf("%s", result.Error)
	}

	// All retries failed
	if lastErr != nil {
		result.Error = fmt.Sprintf("failed after %d attempts: %v", maxAttempts, lastErr)
	}

	return result
}

// probeURLOnce performs a single HTTP probe attempt
func (p *Prober) probeURLOnce(ctx context.Context, probeURL string, originalInput string) output.ProbeResult {
	// Ensure URL has scheme
	if !strings.HasPrefix(probeURL, "http://") && !strings.HasPrefix(probeURL, "https://") {
		probeURL = "http://" + probeURL
	}

	// Parse URL to validate and extract scheme
	parsedURL, err := url.Parse(probeURL)
	if err != nil {
		result := output.ProbeResult{
			Timestamp: time.Now().Format(time.RFC3339),
			Input:     originalInput,
			Method:    "GET",
			Error:     fmt.Sprintf("Invalid URL: %v", err),
		}
		p.logError("failed to parse URL", "url", probeURL, "error", err)
		return result
	}

	// Strip default ports so Go sends the correct Host header.
	// Servers may reject requests with "Host: example.com:443" for HTTPS
	// or "Host: example.com:80" for HTTP.
	probeURL = stripDefaultPort(parsedURL)

	// For HTTPS URLs, use parallel TLS attempts
	if parsedURL.Scheme == "https" {
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Info("probing HTTPS URL with parallel TLS", "url", probeURL)
		}
		return p.probeURLWithParallelTLS(ctx, probeURL, originalInput)
	}

	// For HTTP URLs, use the standard probe method
	if p.config.DebugLogger != nil {
		p.config.DebugLogger.Info("probing HTTP URL", "url", probeURL)
	}
	return p.probeURLHTTP(ctx, probeURL, originalInput)
}

// probeURLHTTP performs a standard HTTP probe (no TLS)
func (p *Prober) probeURLHTTP(ctx context.Context, probeURL string, originalInput string) output.ProbeResult {
	// Create debug buffer for collecting all debug output for this URL
	var debugBuf strings.Builder

	result := output.ProbeResult{
		Timestamp: time.Now().Format(time.RFC3339),
		Input:     originalInput,
		Method:    "GET",
		Protocol:  "HTTP/1.1",
	}

	if p.config.DebugLogger != nil {
		p.config.DebugLogger.Info("probing HTTP URL", "url", probeURL)
	}

	// Parse URL to validate and extract hostname
	parsedURL, err := url.Parse(probeURL)
	if err != nil {
		result.Error = fmt.Sprintf("Invalid URL: %v", err)
		p.logError("failed to parse URL", "url", probeURL, "error", err)
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Error("failed to parse URL", "url", probeURL, "error", err)
		}
		p.flushDebugBuffer(&debugBuf)
		return result
	}

	// Extract hostname for rate limiting
	hostname := parsedURL.Hostname()

	// Apply rate limiting per host with timeout
	limiter := p.client.GetLimiter(hostname)
	
	waitCtx, waitCancel := context.WithTimeout(ctx, time.Duration(p.config.RateLimitTimeout)*time.Second)
	defer waitCancel()
	
	if err := limiter.Wait(waitCtx); err != nil {
		if err == context.DeadlineExceeded {
			result.Error = fmt.Sprintf("rate limit wait timeout after %ds", p.config.RateLimitTimeout)
		} else {
			result.Error = fmt.Sprintf("rate limit wait cancelled: %v", err)
		}
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Warn("rate limit wait failed", "url", probeURL, "error", err)
		}
		p.flushDebugBuffer(&debugBuf)
		return result
	}

	// Debug: print separator at start of probe
	p.debugPrintSeparator(&debugBuf)

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", probeURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		p.logError("failed to create request", "url", probeURL, "error", err)
		p.flushDebugBuffer(&debugBuf)
		return result
	}

	// Set browser-like headers
	req.Header.Set("User-Agent", useragent.Get(p.config.UserAgent, p.config.RandomUserAgent))
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Capture raw request if storage or include-response is enabled
	var rawRequest string
	if p.config.StoreResponse || p.config.IncludeResponse {
		rawRequest = formatRawRequest(req)
	}

	// Debug: log initial request
	p.debugRequest(req, 1, &debugBuf)

	// Make HTTP request
	startTime := time.Now()
	resp, err := p.client.GetHTTPClient().Do(req)
	elapsed := time.Since(startTime)

	if err != nil {
		result.Error = fmt.Sprintf("Request failed: %v", err)
		p.logError("request failed", "url", probeURL, "error", err)
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Error("HTTP request failed",
				"url", probeURL,
				"error", err,
				"duration", elapsed,
			)
		}
		p.debugPrintSeparator(&debugBuf)
		p.flushDebugBuffer(&debugBuf)
		return result
	}

	if p.config.DebugLogger != nil {
		p.config.DebugLogger.Info("HTTP request succeeded",
			"url", probeURL,
			"status_code", resp.StatusCode,
			"duration", elapsed,
		)
	}

	// PERFORMANCE FIX: Use TeeReader for single-pass body reading in debug mode
	var bodyBuffer bytes.Buffer
	var bodyReader io.Reader = resp.Body

	if p.config.Debug {
		bodyReader = io.TeeReader(resp.Body, &bodyBuffer)
	}

	// SECURITY FIX: Add response body size limit
	limitedReader := io.LimitReader(bodyReader, p.config.MaxBodySize)

	// Read initial response body
	initialBody, err := io.ReadAll(limitedReader)
	resp.Body.Close()

	if err != nil {
		result.Error = fmt.Sprintf("Error reading body: %v", err)
		p.logError("failed to read body", "url", probeURL, "error", err)
		p.flushDebugBuffer(&debugBuf)
		return result
	}

	// Check if body was truncated
	if int64(len(initialBody)) >= p.config.MaxBodySize {
		p.config.Logger.Warn("response body truncated",
			"url", probeURL,
			"max_size", p.config.MaxBodySize,
		)
	}

	// Debug: log initial response (use buffered body if available)
	debugBody := initialBody
	if p.config.Debug && bodyBuffer.Len() > 0 {
		debugBody = bodyBuffer.Bytes()
	}
	p.debugResponse(resp, debugBody, elapsed, 1, &debugBuf)

	// Extract initial hostname for redirect tracking
	initialHostname := parsedURL.Hostname()

	// Follow redirects manually if enabled to capture the chain
	var finalResp *http.Response
	var statusChain []int
	var hostChain []string

	if p.config.FollowRedirects && (resp.StatusCode >= 300 && resp.StatusCode < 400) {
		// Response is a redirect and we should follow it
		// Recreate response body for redirect following
		resp.Body = io.NopCloser(bytes.NewReader(initialBody))
		finalResp, statusChain, hostChain, err = p.followRedirects(ctx, resp, p.config.MaxRedirects, 1, initialHostname, &debugBuf)
		if err != nil {
			result.Error = fmt.Sprintf("Redirect error: %v", err)
			result.ChainStatusCodes = statusChain
			result.ChainHosts = hostChain
			p.logError("redirect error", "url", probeURL, "error", err)
			p.debugPrintSeparator(&debugBuf)
			p.flushDebugBuffer(&debugBuf)
			return result
		}
		// Read final response body
		finalResp.Body = io.NopCloser(io.LimitReader(finalResp.Body, p.config.MaxBodySize))
		initialBody, _ = io.ReadAll(finalResp.Body)
		finalResp.Body.Close()
	} else {
		// Not a redirect or not following redirects
		finalResp = resp
		statusChain = []int{resp.StatusCode}
		hostChain = []string{initialHostname}
	}

	// Debug: print separator at end of probe
	p.debugPrintSeparator(&debugBuf)

	// Extract final URL after redirects
	finalURL := finalResp.Request.URL.String()
	finalParsedURL := finalResp.Request.URL

	// Calculate hashes
	result.Hash.BodyMMH3 = hash.CalculateMMH3(initialBody)
	result.Hash.HeaderMMH3 = hash.CalculateHeaderMMH3(finalResp.Header)

	// Extract metadata
	result.URL = probeURL
	result.FinalURL = finalURL
	result.ChainStatusCodes = statusChain
	result.ChainHosts = hostChain
	result.StatusCode = finalResp.StatusCode
	result.ContentLength = len(initialBody)
	result.Time = elapsed.String()
	result.WebServer = finalResp.Header.Get("Server")
	result.ContentType = finalResp.Header.Get("Content-Type")

	// Parse URL components
	result.Scheme = finalParsedURL.Scheme
	result.Host = finalParsedURL.Hostname()
	result.Path = finalParsedURL.Path
	if result.Path == "" {
		result.Path = "/"
	}

	// Extract port
	port := finalParsedURL.Port()
	if port == "" {
		if result.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	result.Port = port

	// Extract title and count words/lines
	bodyStr := string(initialBody)
	result.Title = parser.ExtractTitle(bodyStr)
	result.Words, result.Lines = parser.CountWordsAndLines(bodyStr)

	// Handle response storage and JSON output options
	if p.config.IncludeResponseHeader {
		result.ResponseHeaders = normalizeHeaders(finalResp.Header)
		result.RequestHeaders = normalizeHeaders(req.Header)
	}

	if p.config.IncludeResponse {
		result.RawRequest = rawRequest
		result.RawResponse = formatRawResponse(finalResp)
	}

	if p.config.StoreResponse {
		rawResponseHeaders := formatRawResponse(finalResp)
		// Build redirect chain info for storage
		redirectChain := make([]string, len(hostChain))
		for i, host := range hostChain {
			redirectChain[i] = fmt.Sprintf("%s (status: %d)", host, statusChain[i])
		}

		storedData := storage.FormatStoredResponse(rawRequest, rawResponseHeaders, redirectChain, initialBody, finalURL)
		storagePath, err := storage.StoreResponse(p.config.StoreResponseDir, finalParsedURL, "GET", storedData)
		if err != nil {
			p.config.Logger.Warn("failed to store response",
				"url", probeURL,
				"error", err,
			)
		} else {
			result.StoredResponsePath = storagePath
		}
	}

	// Flush debug buffer atomically to stderr
	p.flushDebugBuffer(&debugBuf)

	p.config.Logger.Debug("probe completed",
		"url", probeURL,
		"status", result.StatusCode,
		"duration", elapsed,
	)

	return result
}

// probeURLWithParallelTLS performs sequential TLS fallback for HTTPS URLs.
// It tries strategies in compatibility order and returns the first successful
// HTTP response. Only connection-level errors trigger fallback to the next strategy.
func (p *Prober) probeURLWithParallelTLS(ctx context.Context, probeURL string, originalInput string) output.ProbeResult {
	// Extract hostname for rate limiting
	parsedURL, err := url.Parse(probeURL)
	if err != nil {
		return output.ProbeResult{
			Timestamp: time.Now().Format(time.RFC3339),
			Input:     originalInput,
			Method:    "GET",
			Error:     fmt.Sprintf("Invalid URL: %v", err),
		}
	}

	hostname := parsedURL.Hostname()
	strategies := GetOrderedStrategies(p.config.DisableHTTP3)

	var allErrors []string

	for i, sp := range strategies {
		// Check context before each attempt
		if ctx.Err() != nil {
			return output.ProbeResult{
				Timestamp: time.Now().Format(time.RFC3339),
				Input:     originalInput,
				Method:    "GET",
				Error:     "cancelled",
			}
		}

		// Rate limit each actual connection attempt
		limiter := p.client.GetLimiter(hostname)
		waitCtx, waitCancel := context.WithTimeout(ctx, time.Duration(p.config.RateLimitTimeout)*time.Second)
		if err := limiter.Wait(waitCtx); err != nil {
			waitCancel()
			if err == context.DeadlineExceeded {
				return output.ProbeResult{
					Timestamp: time.Now().Format(time.RFC3339),
					Input:     originalInput,
					Method:    "GET",
					Error:     fmt.Sprintf("rate limit wait timeout after %ds", p.config.RateLimitTimeout),
				}
			}
			return output.ProbeResult{
				Timestamp: time.Now().Format(time.RFC3339),
				Input:     originalInput,
				Method:    "GET",
				Error:     fmt.Sprintf("rate limit wait cancelled: %v", err),
			}
		}
		waitCancel()

		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Info("trying TLS strategy",
				"url", probeURL,
				"strategy", sp.Strategy.Name,
				"protocol", sp.Protocol,
				"attempt", i+1,
				"of", len(strategies),
			)
		}

		// Create a per-attempt timeout context
		tlsCtx, tlsCancel := context.WithTimeout(ctx, time.Duration(p.config.TLSHandshakeTimeout)*time.Second)
		result := p.probeURLWithConfig(tlsCtx, probeURL, originalInput, sp.Strategy, sp.Protocol)
		tlsCancel()

		// Any HTTP response (even 4xx/5xx) means the host is reachable
		if result.Error == "" {
			return result
		}

		// Non-connection error — no point trying other strategies
		if !isConnectionError(result.Error) {
			if p.config.DebugLogger != nil {
				p.config.DebugLogger.Debug("non-retryable error, stopping fallback",
					"url", probeURL,
					"strategy", sp.Strategy.Name,
					"error", result.Error,
				)
			}
			return result
		}

		// Connection error — record and try next strategy
		allErrors = append(allErrors, fmt.Sprintf("%s/%s: %s", sp.Strategy.Name, sp.Protocol, result.Error))
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Debug("connection error, trying next strategy",
				"url", probeURL,
				"strategy", sp.Strategy.Name,
				"protocol", sp.Protocol,
				"error", result.Error,
			)
		}
	}

	// All strategies exhausted
	errorMsg := fmt.Sprintf("All TLS attempts failed: %s", strings.Join(allErrors, "; "))
	if p.config.DebugLogger != nil {
		p.config.DebugLogger.Error("all TLS strategies failed",
			"url", probeURL,
			"attempts", len(strategies),
			"errors", allErrors,
		)
	}
	return output.ProbeResult{
		Timestamp: time.Now().Format(time.RFC3339),
		Input:     originalInput,
		Method:    "GET",
		Error:     errorMsg,
	}
}

// isConnectionError returns true if the error string indicates a connection-level
// failure that warrants trying the next TLS strategy. Application-level errors
// (context cancelled, URL parse, rate limit) return false — no point retrying.
func isConnectionError(errMsg string) bool {
	lower := strings.ToLower(errMsg)
	connectionPatterns := []string{
		"tls:",
		"tls_",
		"handshake",
		"connection refused",
		"connection reset",
		"i/o timeout",
		"eof",
		"certificate",
		"no route to host",
		"network unreachable",
		"protocol",
		"no such host",
		"dial tcp",
		"remote error",
	}
	for _, pattern := range connectionPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

// probeURLWithConfig performs a single probe attempt with a specific TLS config and protocol
func (p *Prober) probeURLWithConfig(ctx context.Context, probeURL string, originalInput string, strategy TLSStrategy, protocol string) output.ProbeResult {
	var debugBuf strings.Builder

	result := output.ProbeResult{
		Timestamp:        time.Now().Format(time.RFC3339),
		Input:            originalInput,
		Method:           "GET",
		Protocol:         protocol,
		TLSConfigStrategy: strategy.Name,
	}

	// Log attempt to debug file if enabled
	if p.config.DebugLogger != nil {
		p.config.DebugLogger.Debug("attempting TLS connection",
			"url", probeURL,
			"strategy", strategy.Name,
			"protocol", protocol,
			"tls_min_version", strategy.MinVersion,
			"tls_max_version", strategy.MaxVersion,
		)
	}

	// Build TLS config
	tlsConfig := BuildTLSConfig(strategy, p.config)

	// Create appropriate client based on protocol
	var httpClient *http.Client
	var cleanup func()
	
	switch protocol {
	case "HTTP/3":
		client, transport := NewHTTP3Client(p.config, tlsConfig)
		httpClient = client
		cleanup = func() {
			transport.Close()
		}
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Debug("created HTTP/3 client", "url", probeURL)
		}
	case "HTTP/2":
		httpClient = NewHTTP2Client(p.config, tlsConfig)
		cleanup = func() {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				transport.CloseIdleConnections()
			}
		}
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Debug("created HTTP/2 client", "url", probeURL)
		}
	default: // HTTP/1.1
		httpClient = NewHTTP11Client(p.config, tlsConfig)
		cleanup = func() {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				transport.CloseIdleConnections()
			}
		}
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Debug("created HTTP/1.1 client", "url", probeURL)
		}
	}
	
	// Ensure cleanup happens
	if cleanup != nil {
		defer cleanup()
	}

	// Parse URL
	parsedURL, err := url.Parse(probeURL)
	if err != nil {
		result.Error = fmt.Sprintf("Invalid URL: %v", err)
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Error("failed to parse URL", "url", probeURL, "error", err)
		}
		return result
	}

	// Debug: print separator at start of probe
	p.debugPrintSeparator(&debugBuf)

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", probeURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		p.flushDebugBuffer(&debugBuf)
		return result
	}

	// Set browser-like headers
	req.Header.Set("User-Agent", useragent.Get(p.config.UserAgent, p.config.RandomUserAgent))
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	// Capture raw request if storage or include-response is enabled
	var rawRequestTLS string
	if p.config.StoreResponse || p.config.IncludeResponse {
		rawRequestTLS = formatRawRequest(req)
	}

	// Debug: log initial request
	p.debugRequest(req, 1, &debugBuf)

	// Make HTTP request
	startTime := time.Now()
	resp, err := httpClient.Do(req)
	elapsed := time.Since(startTime)

	if err != nil {
		errorMsg := fmt.Sprintf("Request failed: %v", err)
		result.Error = errorMsg
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Error("request failed",
				"url", probeURL,
				"strategy", strategy.Name,
				"protocol", protocol,
				"error", err,
				"duration", elapsed,
			)
		}
		p.flushDebugBuffer(&debugBuf)
		return result
	}

	if p.config.DebugLogger != nil {
		p.config.DebugLogger.Info("request succeeded",
			"url", probeURL,
			"strategy", strategy.Name,
			"protocol", protocol,
			"status_code", resp.StatusCode,
			"duration", elapsed,
		)
		if resp.TLS != nil {
			p.config.DebugLogger.Debug("TLS connection details",
				"tls_version", getTLSVersionString(resp.TLS.Version),
				"cipher_suite", tls.CipherSuiteName(resp.TLS.CipherSuite),
			)
		}
	}
	defer resp.Body.Close()

	// Extract TLS connection state if available
	if resp.TLS != nil {
		result.TLSVersion = getTLSVersionString(resp.TLS.Version)
		result.CipherSuite = tls.CipherSuiteName(resp.TLS.CipherSuite)
	}

	// Read response body
	var bodyBuffer bytes.Buffer
	var bodyReader io.Reader = resp.Body

	if p.config.Debug {
		bodyReader = io.TeeReader(resp.Body, &bodyBuffer)
	}

	limitedReader := io.LimitReader(bodyReader, p.config.MaxBodySize)
	initialBody, err := io.ReadAll(limitedReader)

	if err != nil {
		result.Error = fmt.Sprintf("Error reading body: %v", err)
		p.flushDebugBuffer(&debugBuf)
		return result
	}

	// Check if body was truncated
	if int64(len(initialBody)) >= p.config.MaxBodySize {
		p.config.Logger.Warn("response body truncated",
			"url", probeURL,
			"max_size", p.config.MaxBodySize,
		)
	}

	// Debug: log initial response
	debugBody := initialBody
	if p.config.Debug && bodyBuffer.Len() > 0 {
		debugBody = bodyBuffer.Bytes()
	}
	p.debugResponse(resp, debugBody, elapsed, 1, &debugBuf)

	// Extract initial hostname for redirect tracking
	initialHostname := parsedURL.Hostname()

	// Follow redirects manually if enabled
	var finalResp *http.Response
	var statusChain []int
	var hostChain []string

	if p.config.FollowRedirects && (resp.StatusCode >= 300 && resp.StatusCode < 400) {
		// Recreate response body for redirect following
		resp.Body = io.NopCloser(bytes.NewReader(initialBody))
		finalResp, statusChain, hostChain, err = p.followRedirectsWithClient(ctx, resp, p.config.MaxRedirects, 1, initialHostname, &debugBuf, httpClient)
		if err != nil {
			result.Error = fmt.Sprintf("Redirect error: %v", err)
			result.ChainStatusCodes = statusChain
			result.ChainHosts = hostChain
			// Close finalResp body if it exists
			if finalResp != nil && finalResp.Body != nil {
				finalResp.Body.Close()
			}
			p.flushDebugBuffer(&debugBuf)
			return result
		}
		// Read final response body
		finalResp.Body = io.NopCloser(io.LimitReader(finalResp.Body, p.config.MaxBodySize))
		bodyReadErr := error(nil)
		initialBody, bodyReadErr = io.ReadAll(finalResp.Body)
		if bodyReadErr != nil {
			p.config.Logger.Warn("error reading final response body",
				"url", probeURL,
				"error", bodyReadErr,
			)
			result.Error = fmt.Sprintf("partial body read: %v", bodyReadErr)
		}
		finalResp.Body.Close()
	} else {
		finalResp = resp
		statusChain = []int{resp.StatusCode}
		hostChain = []string{initialHostname}
	}

	// Debug: print separator at end of probe
	p.debugPrintSeparator(&debugBuf)

	// Extract final URL after redirects
	finalURL := finalResp.Request.URL.String()
	finalParsedURL := finalResp.Request.URL

	// Calculate hashes
	result.Hash.BodyMMH3 = hash.CalculateMMH3(initialBody)
	result.Hash.HeaderMMH3 = hash.CalculateHeaderMMH3(finalResp.Header)

	// Extract metadata
	result.URL = probeURL
	result.FinalURL = finalURL
	result.ChainStatusCodes = statusChain
	result.ChainHosts = hostChain
	result.StatusCode = finalResp.StatusCode
	result.ContentLength = len(initialBody)
	result.Time = elapsed.String()
	result.WebServer = finalResp.Header.Get("Server")
	result.ContentType = finalResp.Header.Get("Content-Type")

	// Parse URL components
	result.Scheme = finalParsedURL.Scheme
	result.Host = finalParsedURL.Hostname()
	result.Path = finalParsedURL.Path
	if result.Path == "" {
		result.Path = "/"
	}

	// Extract port
	port := finalParsedURL.Port()
	if port == "" {
		if result.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	result.Port = port

	// Extract title and count words/lines
	bodyStr := string(initialBody)
	result.Title = parser.ExtractTitle(bodyStr)
	result.Words, result.Lines = parser.CountWordsAndLines(bodyStr)

	// Handle response storage and JSON output options
	if p.config.IncludeResponseHeader {
		result.ResponseHeaders = normalizeHeaders(finalResp.Header)
		result.RequestHeaders = normalizeHeaders(req.Header)
	}

	if p.config.IncludeResponse {
		result.RawRequest = rawRequestTLS
		result.RawResponse = formatRawResponse(finalResp)
	}

	if p.config.StoreResponse {
		rawResponseHeaders := formatRawResponse(finalResp)
		// Build redirect chain info for storage
		redirectChain := make([]string, len(hostChain))
		for i, host := range hostChain {
			redirectChain[i] = fmt.Sprintf("%s (status: %d)", host, statusChain[i])
		}

		storedData := storage.FormatStoredResponse(rawRequestTLS, rawResponseHeaders, redirectChain, initialBody, finalURL)
		storagePath, err := storage.StoreResponse(p.config.StoreResponseDir, finalParsedURL, "GET", storedData)
		if err != nil {
			p.config.Logger.Warn("failed to store response",
				"url", probeURL,
				"error", err,
			)
		} else {
			result.StoredResponsePath = storagePath
		}
	}

	// Flush debug buffer atomically to stderr
	p.flushDebugBuffer(&debugBuf)

	return result
}

// getTLSVersionString converts TLS version to string
func getTLSVersionString(version uint16) string {
	switch version {
	case 0x0304:
		return "1.3"
	case 0x0303:
		return "1.2"
	case 0x0302:
		return "1.1"
	case 0x0301:
		return "1.0"
	default:
		return fmt.Sprintf("0x%04x", version)
	}
}

// followRedirectsWithClient follows redirects using a specific HTTP client
func (p *Prober) followRedirectsWithClient(ctx context.Context, initialResp *http.Response, maxRedirects int, startStep int, initialHostname string, buf *strings.Builder, httpClient *http.Client) (*http.Response, []int, []string, error) {
	statusChain := []int{initialResp.StatusCode}
	hostChain := []string{initialHostname}
	currentResp := initialResp

	if currentResp.StatusCode < 300 || currentResp.StatusCode >= 400 {
		return currentResp, statusChain, hostChain, nil
	}

	redirectCount := 0
	stepNum := startStep
	for {
		select {
		case <-ctx.Done():
			return currentResp, statusChain, hostChain, ctx.Err()
		default:
		}

		if redirectCount >= maxRedirects {
			return currentResp, statusChain, hostChain, fmt.Errorf("stopped after %d redirects", maxRedirects)
		}

		location := currentResp.Header.Get("Location")
		if location == "" {
			return currentResp, statusChain, hostChain, nil
		}

		currentResp.Body.Close()

		nextURL, err := currentResp.Request.URL.Parse(location)
		if err != nil {
			return currentResp, statusChain, hostChain, fmt.Errorf("invalid redirect location: %v", err)
		}

		// Normalize port when scheme changes (e.g., http:80 -> https should use 443)
		nextURL = normalizeRedirectURL(currentResp.Request.URL, nextURL)

		nextHostname := nextURL.Hostname()

		if p.config.SameHostOnly && nextHostname != initialHostname {
			if p.config.Debug {
				warning := fmt.Sprintf("  ⚠ Cross-host redirect blocked: %s → %s (same-host-only mode)\n", initialHostname, nextHostname)
				if buf != nil {
					buf.WriteString(warning)
				}
			}
			return currentResp, statusChain, hostChain, fmt.Errorf("cross-host redirect blocked: %s → %s", initialHostname, nextHostname)
		}

		req, err := http.NewRequestWithContext(ctx, "GET", nextURL.String(), nil)
		if err != nil {
			return currentResp, statusChain, hostChain, fmt.Errorf("failed to create redirect request: %v", err)
		}

		req.Header = currentResp.Request.Header

		stepNum++
		p.debugRequest(req, stepNum, buf)
		if p.config.Debug && nextHostname != initialHostname {
			warning := fmt.Sprintf("  ⚠ Cross-host redirect: %s → %s\n", initialHostname, nextHostname)
			if buf != nil {
				buf.WriteString(warning)
			}
		}

		requestStart := time.Now()
		nextResp, err := httpClient.Do(req)
		requestElapsed := time.Since(requestStart)
		if err != nil {
			return currentResp, statusChain, hostChain, fmt.Errorf("redirect request failed: %v", err)
		}

		var nextBody []byte
		if p.config.Debug {
			var bodyBuffer bytes.Buffer
			bodyReader := io.TeeReader(nextResp.Body, &bodyBuffer)
			nextBody, _ = io.ReadAll(io.LimitReader(bodyReader, p.config.MaxBodySize))
			nextResp.Body.Close()
			nextResp.Body = io.NopCloser(bytes.NewReader(nextBody))
		}

		p.debugResponse(nextResp, nextBody, requestElapsed, stepNum, buf)

		statusChain = append(statusChain, nextResp.StatusCode)
		hostChain = append(hostChain, nextHostname)
		currentResp = nextResp
		redirectCount++

		if nextResp.StatusCode < 300 || nextResp.StatusCode >= 400 {
			return nextResp, statusChain, hostChain, nil
		}
	}
}

// Helper methods for debugging
func (p *Prober) debugPrintSeparator(buf *strings.Builder) {
	if !p.config.Debug {
		return
	}
	line := "========================================\n"
	if buf != nil {
		buf.WriteString(line)
	}
}

func (p *Prober) debugRequest(req *http.Request, stepNum int, buf *strings.Builder) {
	if !p.config.Debug {
		return
	}

	var out strings.Builder
	fmt.Fprintf(&out, "[%d] REQUEST: %s %s\n", stepNum, req.Method, req.URL.String())

	if len(req.Header) > 0 {
		fmt.Fprintln(&out, "Headers:")
		var keys []string
		for k := range req.Header {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			for _, v := range req.Header[k] {
				fmt.Fprintf(&out, "  %s: %s\n", k, v)
			}
		}
	}
	fmt.Fprintln(&out, "")

	if buf != nil {
		buf.WriteString(out.String())
	}
}

func (p *Prober) debugResponse(resp *http.Response, body []byte, elapsed time.Duration, stepNum int, buf *strings.Builder) {
	if !p.config.Debug {
		return
	}

	var out strings.Builder
	fmt.Fprintf(&out, "[%d] RESPONSE: %d %s (%s)\n", stepNum, resp.StatusCode, resp.Status, elapsed)

	if len(resp.Header) > 0 {
		fmt.Fprintln(&out, "Headers:")
		var keys []string
		for k := range resp.Header {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			for _, v := range resp.Header[k] {
				fmt.Fprintf(&out, "  %s: %s\n", k, v)
			}
		}
	}

	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location != "" {
			fmt.Fprintf(&out, "  → Redirecting to: %s\n", location)
		}
	}

	if len(body) > 0 {
		preview := body
		maxPreview := 200
		if len(preview) > maxPreview {
			preview = preview[:maxPreview]
		}
		fmt.Fprintf(&out, "Body preview (first %d bytes):\n", len(preview))
		fmt.Fprintf(&out, "  %s\n", string(preview))
		if len(body) > maxPreview {
			fmt.Fprintf(&out, "  ... (%d more bytes)\n", len(body)-maxPreview)
		}
	}
	fmt.Fprintln(&out, "")

	if buf != nil {
		buf.WriteString(out.String())
	}
}

func (p *Prober) flushDebugBuffer(buf *strings.Builder) {
	if buf.Len() > 0 {
		p.stderrMutex.Lock()
		fmt.Fprint(os.Stderr, buf.String())
		p.stderrMutex.Unlock()
	}
}

func (p *Prober) logError(msg string, args ...interface{}) {
	if !p.config.Silent {
		p.config.Logger.Error(msg, args...)
	}
}

// formatRawRequest formats an HTTP request as a raw string
func formatRawRequest(req *http.Request) string {
	var builder strings.Builder

	// Request line
	path := req.URL.Path
	if path == "" {
		path = "/"
	}
	if req.URL.RawQuery != "" {
		path += "?" + req.URL.RawQuery
	}
	builder.WriteString(fmt.Sprintf("%s %s HTTP/1.1\n", req.Method, path))

	// Host header
	builder.WriteString(fmt.Sprintf("Host: %s\n", req.Host))

	// Other headers (sorted for consistency)
	var keys []string
	for k := range req.Header {
		if k != "Host" { // Skip Host as we already wrote it
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range req.Header[k] {
			builder.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
	}

	return builder.String()
}

// formatRawResponse formats HTTP response headers as a raw string
func formatRawResponse(resp *http.Response) string {
	var builder strings.Builder

	// Status line
	builder.WriteString(fmt.Sprintf("HTTP/%d.%d %d %s\n",
		resp.ProtoMajor, resp.ProtoMinor, resp.StatusCode, resp.Status))

	// Headers (sorted for consistency)
	var keys []string
	for k := range resp.Header {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range resp.Header[k] {
			builder.WriteString(fmt.Sprintf("%s: %s\n", k, v))
		}
	}

	return builder.String()
}

// normalizeHeaders normalizes HTTP headers for JSON output
// Converts header names to lowercase and replaces hyphens with underscores
func normalizeHeaders(headers http.Header) map[string]string {
	normalized := make(map[string]string, len(headers))
	for k, v := range headers {
		// Convert to lowercase and replace hyphens with underscores
		key := strings.ReplaceAll(strings.ToLower(k), "-", "_")
		// Join multiple values with comma
		normalized[key] = strings.Join(v, ", ")
	}
	return normalized
}

// stripDefaultPort returns the URL string with the port removed when it matches
// the scheme's default (80 for HTTP, 443 for HTTPS). This prevents Go's net/http
// from sending "Host: example.com:443" which some servers reject.
func stripDefaultPort(u *url.URL) string {
	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		// Rebuild with just the hostname (no port)
		u.Host = u.Hostname()
	}
	return u.String()
}

// Ensure storage import is used
var _ = storage.GenerateFilename
