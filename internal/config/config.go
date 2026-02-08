package config

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"probeHTTP/pkg/version"
)

// Config holds the CLI configuration
type Config struct {
	InputFile          string
	OutputFile         string
	FollowRedirects    bool
	MaxRedirects       int
	Timeout            int
	Concurrency        int
	Silent             bool
	Debug              bool
	SameHostOnly       bool
	UserAgent          string
	RandomUserAgent    bool
	AllSchemes         bool
	IgnorePorts        bool
	CustomPorts        string
	InsecureSkipVerify bool
	AllowPrivateIPs    bool // NEW: Allow scanning private IPs
	MaxBodySize        int64 // NEW: Maximum response body size in bytes
	MaxRetries         int   // NEW: Maximum number of retries
	TLSHandshakeTimeout int  // NEW: Timeout for TLS handshake attempts in seconds
	RateLimitTimeout   int   // NEW: Timeout for rate limit wait in seconds
	DisableHTTP3       bool  // NEW: Disable HTTP/3 (QUIC) support
	DebugLogFile       string // NEW: Debug log file path (optional)
	Version            bool   // NEW: Show version information
	// Feature detection options
	ResolveIP      bool     // Resolve and report IP addresses
	DetectHSTS     bool     // Detect HSTS headers
	TechDetect     bool     // Enable technology detection
	DetectCDN      bool     // Enable CDN detection
	DetectCNAME    bool     // Enable CNAME resolution
	// Storage options
	StoreResponse         bool   // Store HTTP responses to disk
	StoreResponseDir      string // Directory for stored responses
	IncludeResponseHeader bool   // Include response headers in JSON output
	IncludeResponse       bool   // Include full request/response in JSON output
	Logger             *slog.Logger // NEW: Structured logger
	DebugLogger        *slog.Logger // NEW: Debug file logger (if DebugLogFile is set)
	debugFileHandle    *os.File // Track debug file handle for cleanup
}

// New creates a new Config with default values
func New() *Config {
	return &Config{
		FollowRedirects:    true,
		MaxRedirects:       10,
		Timeout:            30,
		Concurrency:        20,
		Silent:             false,
		Debug:              false,
		SameHostOnly:       false,
		RandomUserAgent:    false,
		AllSchemes:         false,
		IgnorePorts:        false,
		InsecureSkipVerify: false,
		AllowPrivateIPs:    false,
		MaxBodySize:        10 * 1024 * 1024, // 10 MB default
		MaxRetries:         0,                // No retries by default
		TLSHandshakeTimeout: 10,              // 10 seconds default
		RateLimitTimeout:   60,               // 60 seconds default
		DisableHTTP3:       false,            // HTTP/3 enabled by default
		Version:            false,
		StoreResponse:      false,            // Response storage disabled by default
		StoreResponseDir:   "output",         // Default storage directory
		IncludeResponseHeader: false,         // Response headers not included by default
		IncludeResponse:    false,            // Full request/response not included by default
	}
}

// ParseFlags parses command-line flags into the config
func ParseFlags() (*Config, error) {
	cfg := New()

	flag.StringVar(&cfg.InputFile, "i", "", "Input file (default: stdin)")
	flag.StringVar(&cfg.InputFile, "input", "", "Input file (default: stdin)")
	flag.StringVar(&cfg.OutputFile, "o", "", "Output file (default: stdout)")
	flag.StringVar(&cfg.OutputFile, "output", "", "Output file (default: stdout)")
	flag.BoolVar(&cfg.FollowRedirects, "fr", true, "Follow redirects")
	flag.BoolVar(&cfg.FollowRedirects, "follow-redirects", true, "Follow redirects")
	flag.IntVar(&cfg.MaxRedirects, "maxr", 10, "Max redirects")
	flag.IntVar(&cfg.MaxRedirects, "max-redirects", 10, "Max redirects")
	flag.IntVar(&cfg.Timeout, "t", 30, "Request timeout in seconds")
	flag.IntVar(&cfg.Timeout, "timeout", 30, "Request timeout in seconds")
	flag.IntVar(&cfg.Concurrency, "c", 20, "Concurrent requests")
	flag.IntVar(&cfg.Concurrency, "concurrency", 20, "Concurrent requests")
	flag.BoolVar(&cfg.Silent, "silent", false, "Silent mode (no errors to stderr)")
	flag.BoolVar(&cfg.Debug, "d", false, "Debug mode (show all requests and responses to stderr)")
	flag.BoolVar(&cfg.Debug, "debug", false, "Debug mode (show all requests and responses to stderr)")
	flag.BoolVar(&cfg.SameHostOnly, "sho", false, "Only follow redirects to same hostname")
	flag.BoolVar(&cfg.SameHostOnly, "same-host-only", false, "Only follow redirects to same hostname")
	flag.StringVar(&cfg.UserAgent, "ua", "", "Custom User-Agent header")
	flag.StringVar(&cfg.UserAgent, "user-agent", "", "Custom User-Agent header")
	flag.BoolVar(&cfg.RandomUserAgent, "rua", false, "Use random User-Agent from pool")
	flag.BoolVar(&cfg.RandomUserAgent, "random-user-agent", false, "Use random User-Agent from pool")
	flag.BoolVar(&cfg.AllSchemes, "as", false, "Test both HTTP and HTTPS schemes")
	flag.BoolVar(&cfg.AllSchemes, "all-schemes", false, "Test both HTTP and HTTPS schemes")
	flag.BoolVar(&cfg.IgnorePorts, "ip", false, "Ignore input ports and test common HTTP/HTTPS ports")
	flag.BoolVar(&cfg.IgnorePorts, "ignore-ports", false, "Ignore input ports and test common HTTP/HTTPS ports")
	flag.StringVar(&cfg.CustomPorts, "p", "", "Custom port list (comma-separated, supports ranges)")
	flag.StringVar(&cfg.CustomPorts, "ports", "", "Custom port list (comma-separated, supports ranges)")
	flag.BoolVar(&cfg.InsecureSkipVerify, "k", false, "Skip TLS certificate verification")
	flag.BoolVar(&cfg.InsecureSkipVerify, "insecure", false, "Skip TLS certificate verification")
	flag.BoolVar(&cfg.AllowPrivateIPs, "allow-private", false, "Allow scanning private IP addresses")
	flag.IntVar(&cfg.MaxRetries, "retries", 0, "Maximum number of retries for failed requests")
	flag.IntVar(&cfg.TLSHandshakeTimeout, "tls-timeout", 10, "Timeout for TLS handshake attempts in seconds")
	flag.IntVar(&cfg.TLSHandshakeTimeout, "tls-handshake-timeout", 10, "Timeout for TLS handshake attempts in seconds")
	flag.IntVar(&cfg.RateLimitTimeout, "rate-limit-timeout", 60, "Rate limit wait timeout in seconds")
	flag.BoolVar(&cfg.DisableHTTP3, "disable-http3", false, "Disable HTTP/3 (QUIC) support")
	flag.StringVar(&cfg.DebugLogFile, "debug-log", "", "Write detailed debug logs to file")
	flag.BoolVar(&cfg.Version, "version", false, "Show version information")
	flag.BoolVar(&cfg.Version, "v", false, "Show version information")
	// Storage options
	flag.BoolVar(&cfg.StoreResponse, "sr", false, "Store HTTP responses to output directory")
	flag.BoolVar(&cfg.StoreResponse, "store-response", false, "Store HTTP responses to output directory")
	flag.StringVar(&cfg.StoreResponseDir, "srd", "output", "Directory to store HTTP responses")
	flag.StringVar(&cfg.StoreResponseDir, "store-response-dir", "output", "Directory to store HTTP responses")
	flag.BoolVar(&cfg.IncludeResponseHeader, "irh", false, "Include response headers in JSON output")
	flag.BoolVar(&cfg.IncludeResponseHeader, "include-response-header", false, "Include response headers in JSON output")
	flag.BoolVar(&cfg.IncludeResponse, "irr", false, "Include full request/response in JSON output")
	flag.BoolVar(&cfg.IncludeResponse, "include-response", false, "Include full request/response in JSON output")
	// Feature detection flags
	flag.BoolVar(&cfg.ResolveIP, "rip", false, "Resolve and include IP address in output")
	flag.BoolVar(&cfg.ResolveIP, "resolve-ip", false, "Resolve and include IP address in output")
	flag.BoolVar(&cfg.DetectHSTS, "hsts", false, "Detect and report HSTS headers")
	flag.BoolVar(&cfg.DetectHSTS, "detect-hsts", false, "Detect and report HSTS headers")
	flag.BoolVar(&cfg.TechDetect, "td", false, "Enable technology detection using wappalyzer")
	flag.BoolVar(&cfg.TechDetect, "tech-detect", false, "Enable technology detection using wappalyzer")
	flag.BoolVar(&cfg.DetectCDN, "cdn", false, "Detect CDN from response headers")
	flag.BoolVar(&cfg.DetectCDN, "detect-cdn", false, "Detect CDN from response headers")
	flag.BoolVar(&cfg.DetectCNAME, "cname", false, "Resolve and report CNAME records")
	flag.BoolVar(&cfg.DetectCNAME, "detect-cname", false, "Resolve and report CNAME records")

	flag.Parse()

	// Handle version flag
	if cfg.Version {
		fmt.Println(version.GetVersion())
		os.Exit(0)
	}

	// Validate mutually exclusive flags
	if cfg.UserAgent != "" && cfg.RandomUserAgent {
		return nil, fmt.Errorf("-ua/--user-agent and -rua/--random-user-agent are mutually exclusive")
	}

	// Set up structured logger
	logLevel := slog.LevelInfo
	if cfg.Debug {
		logLevel = slog.LevelDebug
	}
	if cfg.Silent {
		logLevel = slog.LevelError
	}

	cfg.Logger = slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: logLevel,
	}))

	// Set up debug file logger if specified
	if cfg.DebugLogFile != "" {
		debugFile, err := os.Create(cfg.DebugLogFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create debug log file: %v", err)
		}
		cfg.debugFileHandle = debugFile // Track handle for cleanup
		cfg.DebugLogger = slog.New(slog.NewTextHandler(debugFile, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		}))
		cfg.Logger.Info("debug logging enabled", "file", cfg.DebugLogFile)
	}

	return cfg, nil
}

// Close cleans up the config's resources
func (c *Config) Close() error {
	if c.debugFileHandle != nil {
		return c.debugFileHandle.Close()
	}
	return nil
}

// HasPipedData checks if there is data being piped to stdin
func HasPipedData() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}
