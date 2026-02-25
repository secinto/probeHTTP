package probe

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"

	"probeHTTP/internal/cdn"
	"probeHTTP/internal/config"
	"probeHTTP/internal/hash"
	"probeHTTP/internal/output"
	"probeHTTP/internal/parser"
	"probeHTTP/internal/storage"
	"probeHTTP/internal/tech"
	"probeHTTP/pkg/useragent"
)

// cachedClient wraps an HTTP client with its cleanup function for the client cache.
type cachedClient struct {
	client  *http.Client
	cleanup func()
}

const maxCnameCacheSize = 10000

// Prober handles HTTP probing operations
type Prober struct {
	client        *Client
	config        *config.Config
	ipTracker     *IPTracker
	techDetector  *tech.Detector
	cnameCache    sync.Map            // hostname -> CNAME string
	cnameCacheSz  atomic.Int64        // approximate size for eviction
	cnameCacheMu  sync.Mutex          // serializes eviction to avoid concurrent Range/Delete
	cnameFlight   singleflight.Group  // per-hostname dedup for CNAME lookups
	clientCache   map[string]*cachedClient // strategy:protocol -> cached client
	clientCacheMu sync.Mutex
	// Mutex for atomic stderr writes when flushing debug buffers
	stderrMutex  sync.Mutex
	cleanupFuncs []func() error
	cleanupMutex sync.Mutex
}

// NewProber creates a new Prober instance
func NewProber(cfg *config.Config) *Prober {
	p := &Prober{
		client:       NewClient(cfg),
		config:       cfg,
		cleanupFuncs: make([]func() error, 0),
		clientCache:  make(map[string]*cachedClient),
	}
	if cfg.ResolveIP {
		p.ipTracker = NewIPTracker()
		p.client.SetIPTracker(p.ipTracker)
	}
	if cfg.TechDetect {
		detector, err := tech.NewDetector()
		if err != nil {
			cfg.Logger.Warn("failed to initialize tech detector", "error", err)
		} else {
			p.techDetector = detector
		}
	}
	return p
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

	// Clean up cached TLS clients
	p.clientCacheMu.Lock()
	for _, cc := range p.clientCache {
		if cc.cleanup != nil {
			cc.cleanup()
		}
	}
	p.clientCacheMu.Unlock()

	if err := p.client.Close(); err != nil {
		errs = append(errs, err)
	}

	if len(errs) > 0 {
		return fmt.Errorf("cleanup errors: %v", errs)
	}
	return nil
}

// getOrCreateClient returns a cached HTTP client for the given TLS strategy and protocol,
// creating one if it doesn't exist yet. Clients are reused across requests to preserve
// connection pooling.
func (p *Prober) getOrCreateClient(strategy TLSStrategy, protocol string) *http.Client {
	key := strategy.Name + ":" + protocol

	p.clientCacheMu.Lock()
	defer p.clientCacheMu.Unlock()

	if cached, ok := p.clientCache[key]; ok {
		return cached.client
	}

	tlsConfig := BuildTLSConfig(strategy, p.config)

	var httpClient *http.Client
	var cleanup func()

	switch protocol {
	case "HTTP/3":
		client, transport := NewHTTP3Client(p.config, tlsConfig)
		httpClient = client
		cleanup = func() { transport.Close() }
	case "HTTP/2":
		httpClient = NewHTTP2Client(p.config, tlsConfig)
		if p.ipTracker != nil {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
				transport.DialContext = p.ipTracker.DialContext(dialer)
			}
		}
		cleanup = func() {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				transport.CloseIdleConnections()
			}
		}
	default: // HTTP/1.1
		httpClient = NewHTTP11Client(p.config, tlsConfig)
		if p.ipTracker != nil {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				dialer := &net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second}
				transport.DialContext = p.ipTracker.DialContext(dialer)
			}
		}
		cleanup = func() {
			if transport, ok := httpClient.Transport.(*http.Transport); ok {
				transport.CloseIdleConnections()
			}
		}
	}

	p.clientCache[key] = &cachedClient{client: httpClient, cleanup: cleanup}

	if p.config.DebugLogger != nil {
		p.config.DebugLogger.Debug("created and cached client", "strategy", strategy.Name, "protocol", protocol)
	}

	return httpClient
}

// resolveCNAME resolves and caches the CNAME record for the given hostname.
// Uses singleflight to deduplicate concurrent lookups for the same hostname
// without blocking lookups for different hostnames.
func (p *Prober) resolveCNAME(hostname string, result *output.ProbeResult) {
	if !p.config.DetectCNAME {
		return
	}

	if cachedCNAME, ok := p.cnameCache.Load(hostname); ok {
		if cname := cachedCNAME.(string); cname != "" && cname != hostname+"." {
			result.CNAME = strings.TrimSuffix(cname, ".")
		}
		return
	}

	// singleflight deduplicates concurrent lookups for the same hostname
	// while allowing different hostnames to resolve concurrently.
	v, _, _ := p.cnameFlight.Do(hostname, func() (interface{}, error) {
		// Double-check cache after winning the flight
		if cachedCNAME, ok := p.cnameCache.Load(hostname); ok {
			return cachedCNAME.(string), nil
		}
		cname, err := net.LookupCNAME(hostname)
		if err != nil {
			cname = "" // cache empty string for failed lookups
		}
		// Evict if cache exceeds max size to bound memory.
		// Eviction is serialized via cnameCacheMu so concurrent resolveCNAME callers
		// (different hostnames) cannot run Range/Delete simultaneously and delete
		// entries stored by others.
		if p.cnameCacheSz.Add(1) > maxCnameCacheSize {
			p.cnameCacheMu.Lock()
			if p.cnameCacheSz.Load() > maxCnameCacheSize {
				p.cnameCache.Range(func(k, _ interface{}) bool {
					p.cnameCache.Delete(k)
					return true
				})
				p.cnameCacheSz.Store(0)
			}
			p.cnameCacheMu.Unlock()
		}
		p.cnameCache.Store(hostname, cname)
		return cname, nil
	})

	if cname := v.(string); cname != "" && cname != hostname+"." {
		result.CNAME = strings.TrimSuffix(cname, ".")
	}
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

	// For HTTPS URLs, use sequential TLS fallback
	if parsedURL.Scheme == "https" {
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Info("probing HTTPS URL with TLS fallback", "url", probeURL)
		}
		return p.probeURLWithTLSFallback(ctx, probeURL, originalInput)
	}

	// For HTTP URLs, use the standard probe method
	if p.config.DebugLogger != nil {
		p.config.DebugLogger.Info("probing HTTP URL", "url", probeURL)
	}
	return p.probeURLHTTP(ctx, probeURL, originalInput)
}

// probeURLHTTP performs a standard HTTP probe (no TLS)
func (p *Prober) probeURLHTTP(ctx context.Context, probeURL string, originalInput string) output.ProbeResult {
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

	parsedURL, err := url.Parse(probeURL)
	if err != nil {
		result.Error = fmt.Sprintf("Invalid URL: %v", err)
		p.logError("failed to parse URL", "url", probeURL, "error", err)
		return result
	}

	hostname := parsedURL.Hostname()
	p.resolveCNAME(hostname, &result)

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
		return result
	}

	p.debugPrintSeparator(&debugBuf)

	req, err := http.NewRequestWithContext(ctx, "GET", probeURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		p.logError("failed to create request", "url", probeURL, "error", err)
		p.flushDebugBuffer(&debugBuf)
		return result
	}

	req.Header.Set("User-Agent", useragent.Get(p.config.UserAgent, p.config.RandomUserAgent))
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	var rawRequest string
	if p.config.StoreResponse || p.config.IncludeResponse {
		rawRequest = formatRawRequest(req)
	}

	p.debugRequest(req, 1, &debugBuf)

	startTime := time.Now()
	resp, err := p.client.GetHTTPClient().Do(req)
	elapsed := time.Since(startTime)

	if err != nil {
		result.Error = fmt.Sprintf("Request failed: %v", err)
		p.logError("request failed", "url", probeURL, "error", err)
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Error("HTTP request failed", "url", probeURL, "error", err, "duration", elapsed)
		}
		p.debugPrintSeparator(&debugBuf)
		p.flushDebugBuffer(&debugBuf)
		return result
	}

	if p.config.DebugLogger != nil {
		p.config.DebugLogger.Info("HTTP request succeeded", "url", probeURL, "status_code", resp.StatusCode, "duration", elapsed)
	}

	state := &probeState{
		probeURL:   probeURL,
		parsedURL:  parsedURL,
		req:        req,
		rawRequest: rawRequest,
		httpClient: p.client.GetHTTPClient(),
		elapsed:    elapsed,
		probeStart: startTime,
		debugBuf:   &debugBuf,
	}
	p.processResponse(ctx, resp, state, &result)
	return result
}

// probeState carries per-request context needed by processResponse.
type probeState struct {
	probeURL   string
	parsedURL  *url.URL
	req        *http.Request
	rawRequest string
	httpClient *http.Client
	elapsed    time.Duration    // initial request duration (for debug logging)
	probeStart time.Time        // start of entire probe (for result.Time)
	debugBuf   *strings.Builder
	tlsState   *tls.ConnectionState // nil for plain HTTP
}

// processResponse reads the response body, follows redirects, extracts metadata,
// and populates the ProbeResult. The caller must set protocol-specific fields
// (Protocol, TLSConfigStrategy, TLS info) on result before calling.
// The response body is consumed and closed by this method.
func (p *Prober) processResponse(ctx context.Context, resp *http.Response, state *probeState, result *output.ProbeResult) {
	// Read body with optional debug tee and size limit
	var bodyBuffer bytes.Buffer
	var bodyReader io.Reader = resp.Body
	if p.config.Debug {
		bodyReader = io.TeeReader(resp.Body, &bodyBuffer)
	}
	limitedReader := io.LimitReader(bodyReader, p.config.MaxBodySize)
	initialBody, err := io.ReadAll(limitedReader)
	resp.Body.Close() // Explicitly close transport body (fixes connection leak)

	if err != nil {
		result.Error = fmt.Sprintf("Error reading body: %v", err)
		p.logError("failed to read body", "url", state.probeURL, "error", err)
		p.flushDebugBuffer(state.debugBuf)
		return
	}

	if int64(len(initialBody)) >= p.config.MaxBodySize {
		p.config.Logger.Warn("response body truncated",
			"url", state.probeURL,
			"max_size", p.config.MaxBodySize,
		)
	}

	// Debug: log initial response
	debugBody := initialBody
	if p.config.Debug && bodyBuffer.Len() > 0 {
		debugBody = bodyBuffer.Bytes()
	}
	p.debugResponse(resp, debugBody, state.elapsed, 1, state.debugBuf)

	// Save initial response body and headers for storage before redirects
	initialResponseBody := make([]byte, len(initialBody))
	copy(initialResponseBody, initialBody)
	var initialRawResponse string
	if p.config.StoreResponse {
		initialRawResponse = formatRawResponse(resp)
	}

	initialHostname := state.parsedURL.Hostname()

	// Follow redirects manually if enabled
	var finalResp *http.Response
	var statusChain []int
	var hostChain []string
	var chainEntries []storage.ChainEntry

	if p.config.FollowRedirects && (resp.StatusCode >= 300 && resp.StatusCode < 400) {
		resp.Body = io.NopCloser(bytes.NewReader(initialBody))
		var redirectChainEntries []storage.ChainEntry
		finalResp, statusChain, hostChain, redirectChainEntries, err = p.followRedirects(ctx, resp, p.config.MaxRedirects, 1, initialHostname, state.debugBuf, state.httpClient)
		chainEntries = redirectChainEntries
		if err != nil {
			result.Error = fmt.Sprintf("Redirect error: %v", err)
			result.ChainStatusCodes = statusChain
			result.ChainHosts = hostChain
			if finalResp != nil && finalResp.Body != nil {
				finalResp.Body.Close()
			}
			p.logError("redirect error", "url", state.probeURL, "error", err)
			p.debugPrintSeparator(state.debugBuf)
			p.flushDebugBuffer(state.debugBuf)
			return
		}
		// Read final response body
		finalResp.Body = io.NopCloser(io.LimitReader(finalResp.Body, p.config.MaxBodySize))
		initialBody, err = io.ReadAll(finalResp.Body)
		if err != nil {
			p.config.Logger.Warn("error reading final response body",
				"url", state.probeURL,
				"error", err,
			)
			result.Error = fmt.Sprintf("partial body read: %v", err)
		}
		finalResp.Body.Close()
	} else {
		finalResp = resp
		statusChain = []int{resp.StatusCode}
		hostChain = []string{initialHostname}
	}

	// Debug: print separator
	p.debugPrintSeparator(state.debugBuf)

	// Extract final URL
	finalURL := finalResp.Request.URL.String()
	finalParsedURL := finalResp.Request.URL

	// Calculate hashes
	result.Hash.BodyMMH3 = hash.CalculateMMH3(initialBody)
	result.Hash.HeaderMMH3 = hash.CalculateHeaderMMH3(finalResp.Header)

	// Extract metadata
	result.URL = state.probeURL
	result.FinalURL = finalURL
	result.ChainStatusCodes = statusChain
	result.ChainHosts = hostChain
	result.StatusCode = finalResp.StatusCode
	result.ContentLength = len(initialBody)
	if !state.probeStart.IsZero() {
		result.Time = time.Since(state.probeStart).String()
	} else {
		result.Time = state.elapsed.String()
	}
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
	result.Title = parser.ExtractTitle(bodyStr, finalResp.Header.Get("Content-Type"))
	result.Words, result.Lines = parser.CountWordsAndLines(bodyStr)

	// Resolve IP address
	if p.ipTracker != nil {
		ip := p.ipTracker.GetIP(result.Host)
		if ip != "" {
			result.HostIP = ip
		}
	}

	// HSTS detection
	if p.config.DetectHSTS {
		if hstsHeader := finalResp.Header.Get("Strict-Transport-Security"); hstsHeader != "" {
			result.HSTS = true
			result.HSTSHeader = hstsHeader
		}
	}

	// Technology detection
	if p.techDetector != nil {
		result.Technologies = p.techDetector.Detect(finalResp.Header, initialBody)
	}

	// CDN detection
	if p.config.DetectCDN {
		isCDN, cdnName := cdn.DetectCDN(finalResp.Header)
		result.CDN = isCDN
		result.CDNName = cdnName
	}

	// Domain discovery from certificate SANs/CN and CSP headers
	if p.config.DiscoverDomains {
		result.DiscoveredDomains = DiscoverDomains(state.tlsState, finalResp.Header, state.parsedURL.Hostname())
	}

	// Response headers in JSON output
	if p.config.IncludeResponseHeader {
		result.ResponseHeaders = normalizeHeaders(finalResp.Header)
		result.RequestHeaders = normalizeHeaders(state.req.Header)
	}

	if p.config.IncludeResponse {
		result.RawRequest = state.rawRequest
		result.RawResponse = formatRawResponse(finalResp)
	}

	// Storage
	if p.config.StoreResponse {
		initialEntry := storage.ChainEntry{
			RawRequest:  state.rawRequest,
			RawResponse: initialRawResponse,
			Body:        initialResponseBody,
		}
		fullChain := append([]storage.ChainEntry{initialEntry}, chainEntries...)
		if len(chainEntries) > 0 {
			fullChain[len(fullChain)-1].Body = initialBody
		}

		storedData := storage.FormatStoredResponse(fullChain, finalURL)
		storagePath, storeErr := storage.StoreResponse(p.config.StoreResponseDir, finalParsedURL, storedData)
		if storeErr != nil {
			p.config.Logger.Warn("failed to store response",
				"url", state.probeURL,
				"error", storeErr,
			)
		} else {
			result.StoredResponsePath = storagePath
			storage.AppendToIndex(p.config.StoreResponseDir, storagePath, state.probeURL,
				result.StatusCode, http.StatusText(result.StatusCode))
		}
	}

	// Flush debug buffer
	p.flushDebugBuffer(state.debugBuf)

	p.config.Logger.Debug("probe completed",
		"url", state.probeURL,
		"status", result.StatusCode,
		"duration", state.elapsed,
	)
}

// probeURLWithTLSFallback performs sequential TLS strategy fallback for HTTPS URLs.
// It tries strategies in compatibility order and returns the first successful
// HTTP response. Only connection-level errors trigger fallback to the next strategy.
func (p *Prober) probeURLWithTLSFallback(ctx context.Context, probeURL string, originalInput string) output.ProbeResult {
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

	result := output.ProbeResult{
		Timestamp: time.Now().Format(time.RFC3339),
		Input:     originalInput,
		Method:    "GET",
		Error:     errorMsg,
	}

	// Check if this looks like an SNI requirement (bare IP, TLS rejection)
	if IsSNIRequired(hostname, allErrors) {
		result.SNIRequired = true
		result.Diagnostic = "TLS handshake rejected for bare IP; server likely requires SNI (Server Name Indication)"
		result.URL = probeURL
		result.Scheme = parsedURL.Scheme
		result.Host = hostname
		port := parsedURL.Port()
		if port == "" {
			port = "443"
		}
		result.Port = port
	}

	return result
}

// IsSNIRequired returns true if the target is a bare IP address and all TLS errors
// indicate handshake rejection (not connectivity issues). This diagnoses servers
// that require SNI (Server Name Indication) in the TLS ClientHello.
func IsSNIRequired(hostname string, allErrors []string) bool {
	// Must be a bare IP address (no hostname to send as SNI)
	if net.ParseIP(hostname) == nil {
		return false
	}

	if len(allErrors) == 0 {
		return false
	}

	// Check for TLS handshake rejection indicators
	hasTLSRejection := false
	for _, errMsg := range allErrors {
		lower := strings.ToLower(errMsg)
		// Connectivity issues disqualify SNI diagnosis
		connectivityPatterns := []string{
			"connection refused",
			"timeout",
			"no route to host",
			"no such host",
			"network unreachable",
			"eof",
		}
		for _, pattern := range connectivityPatterns {
			if strings.Contains(lower, pattern) {
				return false
			}
		}
		if strings.Contains(lower, "handshake failure") || strings.Contains(lower, "remote error") {
			hasTLSRejection = true
		}
	}

	return hasTLSRejection
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
		"timeout",
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
		Timestamp:          time.Now().Format(time.RFC3339),
		Input:              originalInput,
		Method:             "GET",
		Protocol:           protocol,
		TLSConfigStrategy: strategy.Name,
	}

	if p.config.DebugLogger != nil {
		p.config.DebugLogger.Debug("attempting TLS connection",
			"url", probeURL, "strategy", strategy.Name, "protocol", protocol,
			"tls_min_version", strategy.MinVersion, "tls_max_version", strategy.MaxVersion)
	}

	parsedURL, err := url.Parse(probeURL)
	if err != nil {
		result.Error = fmt.Sprintf("Invalid URL: %v", err)
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Error("failed to parse URL", "url", probeURL, "error", err)
		}
		return result
	}

	p.resolveCNAME(parsedURL.Hostname(), &result)

	p.debugPrintSeparator(&debugBuf)

	// Get or create cached client for this strategy+protocol
	httpClient := p.getOrCreateClient(strategy, protocol)

	req, err := http.NewRequestWithContext(ctx, "GET", probeURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		p.flushDebugBuffer(&debugBuf)
		return result
	}

	req.Header.Set("User-Agent", useragent.Get(p.config.UserAgent, p.config.RandomUserAgent))
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	var rawRequest string
	if p.config.StoreResponse || p.config.IncludeResponse {
		rawRequest = formatRawRequest(req)
	}

	p.debugRequest(req, 1, &debugBuf)

	startTime := time.Now()
	resp, err := httpClient.Do(req)
	elapsed := time.Since(startTime)

	if err != nil {
		result.Error = fmt.Sprintf("Request failed: %v", err)
		if p.config.DebugLogger != nil {
			p.config.DebugLogger.Error("request failed",
				"url", probeURL, "strategy", strategy.Name, "protocol", protocol,
				"error", err, "duration", elapsed)
		}
		p.flushDebugBuffer(&debugBuf)
		return result
	}

	if p.config.DebugLogger != nil {
		p.config.DebugLogger.Info("request succeeded",
			"url", probeURL, "strategy", strategy.Name, "protocol", protocol,
			"status_code", resp.StatusCode, "duration", elapsed)
		if resp.TLS != nil {
			p.config.DebugLogger.Debug("TLS connection details",
				"tls_version", getTLSVersionString(resp.TLS.Version),
				"cipher_suite", tls.CipherSuiteName(resp.TLS.CipherSuite))
		}
	}

	// Extract TLS connection state if available
	if resp.TLS != nil {
		result.TLSVersion = getTLSVersionString(resp.TLS.Version)
		result.CipherSuite = tls.CipherSuiteName(resp.TLS.CipherSuite)
		result.TLS = &output.TLSInfo{
			Version: result.TLSVersion,
			Cipher:  result.CipherSuite,
		}
		if p.config.ExtractTLS {
			result.TLS.Certificate = ExtractCertificateInfo(resp.TLS)
			if p.config.ExtractTLSChain {
				result.TLS.Chain = ExtractCertificateChain(resp.TLS)
			}
		}
	}

	// Process response (shared logic handles body read, redirects, metadata)
	state := &probeState{
		probeURL:   probeURL,
		parsedURL:  parsedURL,
		req:        req,
		rawRequest: rawRequest,
		httpClient: httpClient,
		elapsed:    elapsed,
		probeStart: startTime,
		debugBuf:   &debugBuf,
		tlsState:   resp.TLS,
	}
	p.processResponse(ctx, resp, state, &result)
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
// The original *url.URL is not modified.
func stripDefaultPort(u *url.URL) string {
	port := u.Port()
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		// Work on a shallow copy to avoid mutating the caller's URL
		copy := *u
		copy.Host = u.Hostname()
		return copy.String()
	}
	return u.String()
}

