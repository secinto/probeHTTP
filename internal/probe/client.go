package probe

import (
	"crypto/tls"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
	"probeHTTP/internal/config"
)

// Client wraps an HTTP client with rate limiting capabilities
type Client struct {
	httpClient *http.Client
	limiters   map[string]*rate.Limiter
	mu         sync.Mutex
	config     *config.Config
}

// NewClient creates a new HTTP client with optimized settings
func NewClient(cfg *config.Config) *Client {
	// SECURITY FIX: Configure TLS with minimum version 1.2 and strong ciphers
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
		},
	}

	// PERFORMANCE FIX: Configure connection pooling
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		TLSClientConfig:     tlsConfig,
		TLSHandshakeTimeout: 10 * time.Second,
		// Enable compression
		DisableCompression: false,
	}

	httpClient := &http.Client{
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
		Transport: transport,
		// Always disable automatic redirects - we handle them manually
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return &Client{
		httpClient: httpClient,
		limiters:   make(map[string]*rate.Limiter),
		config:     cfg,
	}
}

// GetHTTPClient returns the underlying HTTP client
func (c *Client) GetHTTPClient() *http.Client {
	return c.httpClient
}

// GetLimiter returns a rate limiter for the given host
// Creates a new limiter if one doesn't exist
// Rate: 10 requests per second per host
func (c *Client) GetLimiter(host string) *rate.Limiter {
	c.mu.Lock()
	defer c.mu.Unlock()

	if limiter, exists := c.limiters[host]; exists {
		return limiter
	}

	// Allow 10 requests per second per host with burst of 1
	limiter := rate.NewLimiter(10, 1)
	c.limiters[host] = limiter
	return limiter
}

// Note: Rate limiting is done directly in prober.go using limiter.Wait(ctx)
