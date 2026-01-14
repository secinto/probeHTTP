package probe

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// followRedirects manually follows HTTP redirects and captures the status code and host chains
// Returns the final response, complete status code chain, host chain, and any error
func (p *Prober) followRedirects(ctx context.Context, initialResp *http.Response, maxRedirects int, startStep int, initialHostname string, buf *strings.Builder) (*http.Response, []int, []string, error) {
	statusChain := []int{initialResp.StatusCode}
	hostChain := []string{initialHostname}
	currentResp := initialResp

	// Check if initial response is not a redirect
	if currentResp.StatusCode < 300 || currentResp.StatusCode >= 400 {
		return currentResp, statusChain, hostChain, nil
	}

	redirectCount := 0
	stepNum := startStep
	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return currentResp, statusChain, hostChain, ctx.Err()
		default:
		}

		// Check if we've hit max redirects
		if redirectCount >= maxRedirects {
			return currentResp, statusChain, hostChain, fmt.Errorf("stopped after %d redirects", maxRedirects)
		}

		// Get redirect location
		location := currentResp.Header.Get("Location")
		if location == "" {
			// No location header, stop here
			return currentResp, statusChain, hostChain, nil
		}

		// Close previous response body
		currentResp.Body.Close()

		// Parse location URL
		nextURL, err := currentResp.Request.URL.Parse(location)
		if err != nil {
			return currentResp, statusChain, hostChain, fmt.Errorf("invalid redirect location: %v", err)
		}

		// Normalize port when scheme changes (e.g., http:80 -> https should use 443)
		nextURL = normalizeRedirectURL(currentResp.Request.URL, nextURL)

		// Extract hostname from next URL
		nextHostname := nextURL.Hostname()

		// Check if same-host-only mode is enabled and hostname changed
		if p.config.SameHostOnly && nextHostname != initialHostname {
			// Cross-host redirect detected - stop following
			if p.config.Debug {
				warning := fmt.Sprintf("  ⚠ Cross-host redirect blocked: %s → %s (same-host-only mode)\n", initialHostname, nextHostname)
				if buf != nil {
					buf.WriteString(warning)
				}
			}
			return currentResp, statusChain, hostChain, fmt.Errorf("cross-host redirect blocked: %s → %s", initialHostname, nextHostname)
		}

		// Make request to next URL
		req, err := http.NewRequestWithContext(ctx, "GET", nextURL.String(), nil)
		if err != nil {
			return currentResp, statusChain, hostChain, fmt.Errorf("failed to create redirect request: %v", err)
		}

		// Copy headers from original request
		req.Header = currentResp.Request.Header

		// Debug: log redirect request with cross-host warning
		stepNum++
		p.debugRequest(req, stepNum, buf)
		if p.config.Debug && nextHostname != initialHostname {
			warning := fmt.Sprintf("  ⚠ Cross-host redirect: %s → %s\n", initialHostname, nextHostname)
			if buf != nil {
				buf.WriteString(warning)
			}
		}

		// Execute request
		requestStart := time.Now()
		nextResp, err := p.client.GetHTTPClient().Do(req)
		requestElapsed := time.Since(requestStart)
		if err != nil {
			return currentResp, statusChain, hostChain, fmt.Errorf("redirect request failed: %v", err)
		}

		// Read body for debug logging
		var nextBody []byte
		if p.config.Debug {
			// Use TeeReader for efficient reading
			var bodyBuffer bytes.Buffer
			bodyReader := io.TeeReader(nextResp.Body, &bodyBuffer)
			nextBody, _ = io.ReadAll(io.LimitReader(bodyReader, p.config.MaxBodySize))
			nextResp.Body.Close()
			// Recreate body for further processing
			nextResp.Body = io.NopCloser(bytes.NewReader(nextBody))
		}

		// Debug: log redirect response
		p.debugResponse(nextResp, nextBody, requestElapsed, stepNum, buf)

		// Add status code and hostname to chains
		statusChain = append(statusChain, nextResp.StatusCode)
		hostChain = append(hostChain, nextHostname)
		currentResp = nextResp
		redirectCount++

		// Check if we've reached a non-redirect response
		if nextResp.StatusCode < 300 || nextResp.StatusCode >= 400 {
			return nextResp, statusChain, hostChain, nil
		}
	}
}

// normalizeRedirectURL fixes port issues when scheme changes during redirect
// e.g., http://host:80 -> https://host:80 should become https://host:443
// This prevents "http: server gave HTTP response to HTTPS client" errors
func normalizeRedirectURL(currentURL, nextURL *url.URL) *url.URL {
	// Only normalize if scheme changed
	if currentURL.Scheme == nextURL.Scheme {
		return nextURL
	}

	currentPort := currentURL.Port()
	nextPort := nextURL.Port()

	// If the next URL has no explicit port, nothing to normalize
	if nextPort == "" {
		return nextURL
	}

	// Get default port for current scheme
	currentDefaultPort := "80"
	if currentURL.Scheme == "https" {
		currentDefaultPort = "443"
	}

	// If current URL was using default port (explicit or implicit)
	// and next URL kept that same port number, normalize to new scheme's default
	if (currentPort == "" || currentPort == currentDefaultPort) && nextPort == currentDefaultPort {
		// Create a copy with normalized port
		normalized := *nextURL
		// Remove the port from Host (it will default to scheme's standard port)
		normalized.Host = nextURL.Hostname()
		return &normalized
	}

	return nextURL
}
