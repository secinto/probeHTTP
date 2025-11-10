package probe

import (
	"bytes"
	"context"
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
	"probeHTTP/pkg/useragent"
)

// Prober handles HTTP probing operations
type Prober struct {
	client *Client
	config *config.Config
	// Mutex for atomic stderr writes when flushing debug buffers
	stderrMutex sync.Mutex
}

// NewProber creates a new Prober instance
func NewProber(cfg *config.Config) *Prober {
	return &Prober{
		client: NewClient(cfg),
		config: cfg,
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
	// Create debug buffer for collecting all debug output for this URL
	var debugBuf strings.Builder

	result := output.ProbeResult{
		Timestamp: time.Now().Format(time.RFC3339),
		Input:     originalInput,
		Method:    "GET",
	}

	// Ensure URL has scheme
	if !strings.HasPrefix(probeURL, "http://") && !strings.HasPrefix(probeURL, "https://") {
		probeURL = "http://" + probeURL
	}

	// Parse URL to validate and extract hostname
	parsedURL, err := url.Parse(probeURL)
	if err != nil {
		result.Error = fmt.Sprintf("Invalid URL: %v", err)
		p.logError("failed to parse URL", "url", probeURL, "error", err)
		p.flushDebugBuffer(&debugBuf)
		return result
	}

	// Extract hostname for rate limiting
	hostname := parsedURL.Hostname()

	// Apply rate limiting per host
	limiter := p.client.GetLimiter(hostname)
	if err := limiter.Wait(ctx); err != nil {
		result.Error = fmt.Sprintf("Rate limit wait cancelled: %v", err)
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

	// Debug: log initial request
	p.debugRequest(req, 1, &debugBuf)

	// Make HTTP request
	startTime := time.Now()
	resp, err := p.client.GetHTTPClient().Do(req)
	elapsed := time.Since(startTime)

	if err != nil {
		result.Error = fmt.Sprintf("Request failed: %v", err)
		p.logError("request failed", "url", probeURL, "error", err)
		p.debugPrintSeparator(&debugBuf)
		p.flushDebugBuffer(&debugBuf)
		return result
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

	// Flush debug buffer atomically to stderr
	p.flushDebugBuffer(&debugBuf)

	p.config.Logger.Debug("probe completed",
		"url", probeURL,
		"status", result.StatusCode,
		"duration", elapsed,
	)

	return result
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
