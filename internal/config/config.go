package config

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
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
	Logger             *slog.Logger // NEW: Structured logger
}

// New creates a new Config with default values
func New() *Config {
	return &Config{
		FollowRedirects:    true,
		MaxRedirects:       10,
		Timeout:            30,
		Concurrency:        10,
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
	flag.IntVar(&cfg.Concurrency, "c", 10, "Concurrent requests")
	flag.IntVar(&cfg.Concurrency, "concurrency", 10, "Concurrent requests")
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

	flag.Parse()

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

	return cfg, nil
}

// HasPipedData checks if there is data being piped to stdin
func HasPipedData() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}
