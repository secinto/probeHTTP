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

	formatter := RegisterFlags(cfg)
	flag.Usage = func() {
		formatter.PrintUsage(os.Stderr)
	}

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
