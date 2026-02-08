package probe

import (
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/quic-go/quic-go/http3"
	"github.com/quic-go/quic-go"
	"golang.org/x/time/rate"
	"probeHTTP/internal/config"
)

// Client wraps an HTTP client with rate limiting capabilities
type Client struct {
	httpClient     *http.Client
	limiters       map[string]*rate.Limiter
	mu             sync.Mutex
	config         *config.Config
	http3Transport *http3.Transport // Track HTTP/3 transport for cleanup
	ipTracker      *IPTracker
}

// NewClient creates a new HTTP client with optimized settings
func NewClient(cfg *config.Config) *Client {
	// Configure TLS with minimum version 1.2
	// Don't restrict cipher suites to allow compatibility with more servers
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		MinVersion:         tls.VersionTLS12,
		// Let Go choose cipher suites automatically for better compatibility
		// This allows connections to servers that don't support the restricted set
	}

	// PERFORMANCE FIX: Configure connection pooling
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second, // Timeout for reading response headers
		ExpectContinueTimeout: 1 * time.Second,  // Timeout for Expect: 100-continue
		// Enable compression
		DisableCompression: false,
		// Enable HTTP/2 support
		ForceAttemptHTTP2: true,
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

// SetIPTracker sets the IP tracker for recording resolved IPs
func (c *Client) SetIPTracker(tracker *IPTracker) {
	c.ipTracker = tracker
	// Update the default transport's DialContext
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		dialer := &net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}
		transport.DialContext = tracker.DialContext(dialer)
	}
}

// Note: Rate limiting is done directly in prober.go using limiter.Wait(ctx)

// NewClientWithTLSConfig creates a new Client with a specific TLS configuration
func NewClientWithTLSConfig(cfg *config.Config, tlsConfig *tls.Config) *Client {
	// Configure connection pooling
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   time.Duration(cfg.TLSHandshakeTimeout) * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    false,
		ForceAttemptHTTP2:     true,
	}

	httpClient := &http.Client{
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
		Transport: transport,
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

// NewHTTP3Client creates an HTTP/3 client with the specified TLS configuration
// Returns both the client and transport so the transport can be closed later
func NewHTTP3Client(cfg *config.Config, tlsConfig *tls.Config) (*http.Client, *http3.Transport) {
	// Create HTTP/3 transport
	transport := &http3.Transport{
		TLSClientConfig: tlsConfig,
		QUICConfig:      &quic.Config{}, // Use default QUIC config
	}

	httpClient := &http.Client{
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return httpClient, transport
}

// Close cleans up the client's resources
func (c *Client) Close() error {
	if c.http3Transport != nil {
		return c.http3Transport.Close()
	}
	// For HTTP/1.1 and HTTP/2, close idle connections
	if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	return nil
}

// NewHTTP2Client creates an HTTP/2 client with the specified TLS configuration
func NewHTTP2Client(cfg *config.Config, tlsConfig *tls.Config) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   time.Duration(cfg.TLSHandshakeTimeout) * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    false,
		ForceAttemptHTTP2:     true, // Force HTTP/2
	}

	httpClient := &http.Client{
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return httpClient
}

// NewHTTP11Client creates an HTTP/1.1 client with the specified TLS configuration
func NewHTTP11Client(cfg *config.Config, tlsConfig *tls.Config) *http.Client {
	transport := &http.Transport{
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false,
		TLSClientConfig:       tlsConfig,
		TLSHandshakeTimeout:   time.Duration(cfg.TLSHandshakeTimeout) * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    false,
		ForceAttemptHTTP2:     false, // Disable HTTP/2, use HTTP/1.1 only
	}

	httpClient := &http.Client{
		Timeout:   time.Duration(cfg.Timeout) * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return httpClient
}
