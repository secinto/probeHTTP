package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"golang.org/x/term"

	"probeHTTP/internal/config"
	"probeHTTP/internal/parser"
	"probeHTTP/internal/probe"
)

func main() {
	// Parse configuration
	cfg, err := config.ParseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer cfg.Close() // Clean up debug log file

	// If no arguments provided and nothing is piped to stdin, show help
	if flag.NFlag() == 0 && cfg.InputFile == "" && !config.HasPipedData() {
		flag.Usage()
		os.Exit(0)
	}

	// Set up context with cancellation support for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle interrupt signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cfg.Logger.Info("shutting down gracefully...")
		cancel()
	}()

	// Initialize response storage directory if enabled
	if cfg.StoreResponse {
		if err := os.MkdirAll(cfg.StoreResponseDir, 0755); err != nil {
			cfg.Logger.Error("failed to create response directory",
				"path", cfg.StoreResponseDir,
				"error", err,
			)
			os.Exit(1)
		}
		cfg.Logger.Info("response storage enabled",
			"directory", cfg.StoreResponseDir,
		)
	}

	// Get input reader
	var inputReader io.Reader
	if cfg.InputFile != "" {
		file, err := os.Open(cfg.InputFile)
		if err != nil {
			cfg.Logger.Error("failed to open input file", "file", cfg.InputFile, "error", err)
			os.Exit(1)
		}
		defer file.Close()
		inputReader = file
	} else {
		inputReader = os.Stdin
	}

	// Get output writer
	var outputWriter io.Writer
	if cfg.OutputFile != "" {
		file, err := os.Create(cfg.OutputFile)
		if err != nil {
			cfg.Logger.Error("failed to create output file", "file", cfg.OutputFile, "error", err)
			os.Exit(1)
		}
		defer file.Close()
		outputWriter = file
	} else {
		outputWriter = os.Stdout
	}

	// Read URLs from input
	urls := readURLs(inputReader)
	cfg.Logger.Info("loaded URLs", "count", len(urls))

	// Expand URLs based on scheme and port configuration
	expandedURLs := []string{}
	originalInputMap := make(map[string]string)

	for _, inputURL := range urls {
		// Validate URL
		if err := parser.ValidateURL(inputURL, cfg.AllowPrivateIPs); err != nil {
			cfg.Logger.Warn("skipping invalid URL", "url", inputURL, "error", err)
			continue
		}

		expanded := parser.ExpandURLs(inputURL, cfg.AllSchemes, cfg.IgnorePorts, cfg.CustomPorts)
		if cfg.DebugLogger != nil {
			cfg.DebugLogger.Info("expanded URL",
				"input", inputURL,
				"expanded_count", len(expanded),
				"expanded_urls", expanded,
			)
		}
		for _, expandedURL := range expanded {
			expandedURLs = append(expandedURLs, expandedURL)
			originalInputMap[expandedURL] = inputURL
		}
	}

	cfg.Logger.Info("expanded URLs", "count", len(expandedURLs))

	// Deduplicate URLs that resolve to the same endpoint
	// (e.g., http://host and http://host:80 are the same)
	beforeDedup := len(expandedURLs)
	expandedURLs = parser.DeduplicateURLs(expandedURLs)
	afterDedup := len(expandedURLs)
	if beforeDedup != afterDedup {
		cfg.Logger.Info("deduplicated URLs", "before", beforeDedup, "after", afterDedup)
		// Rebuild originalInputMap for deduplicated URLs - O(n) instead of O(n²)
		// Pre-compute normalized mappings
		normalizedMap := make(map[string]string)
		for origURL, origInput := range originalInputMap {
			normalized := parser.NormalizeURL(origURL)
			if _, exists := normalizedMap[normalized]; !exists {
				normalizedMap[normalized] = origInput
			}
		}

		// O(n) lookup
		newOriginalInputMap := make(map[string]string)
		for _, urlStr := range expandedURLs {
			if originalInput, exists := originalInputMap[urlStr]; exists {
				newOriginalInputMap[urlStr] = originalInput
			} else {
				normalized := parser.NormalizeURL(urlStr)
				if origInput, exists := normalizedMap[normalized]; exists {
					newOriginalInputMap[urlStr] = origInput
				} else {
					// Fallback: use the URL itself as input
					newOriginalInputMap[urlStr] = urlStr
				}
			}
		}
		originalInputMap = newOriginalInputMap
	}

	// Create prober
	prober := probe.NewProber(cfg)
	defer prober.Close() // Clean up HTTP clients and transports

	// Process URLs with worker pool
	results := prober.ProcessURLs(ctx, expandedURLs, originalInputMap, cfg.Concurrency)

	// Write results
	successCount := 0
	errorCount := 0
	completed := 0
	total := len(expandedURLs)

	// Check if stderr is a terminal for progress display
	showProgress := !cfg.Silent && term.IsTerminal(int(os.Stderr.Fd()))
	var termHeight int

	// Set up persistent status bar at bottom of terminal
	if showProgress {
		_, termHeight, _ = term.GetSize(int(os.Stderr.Fd()))
		if termHeight > 0 {
			// Set scroll region to exclude the bottom line
			fmt.Fprintf(os.Stderr, "\033[1;%dr", termHeight-1)
			// Move cursor to top of scroll region
			fmt.Fprintf(os.Stderr, "\033[1;1H")
			// Draw initial status bar
			fmt.Fprintf(os.Stderr, "\033[s\033[%d;1H\033[K[0/%d] Starting...\033[u", termHeight, total)
		}
	}

	// Helper function to update status bar
	updateStatusBar := func(url string) {
		if !showProgress || termHeight <= 0 {
			return
		}
		// Truncate URL to fit terminal width
		displayURL := url
		maxURLLen := 70
		if len(displayURL) > maxURLLen {
			displayURL = displayURL[:maxURLLen-3] + "..."
		}
		// Save cursor, move to bottom, clear line, draw status, restore cursor
		fmt.Fprintf(os.Stderr, "\033[s\033[%d;1H\033[K[%d/%d] %s\033[u", termHeight, completed, total, displayURL)
	}

	for result := range results {
		completed++
		updateStatusBar(result.URL)

		// Skip results with errors in JSON output (but emit diagnostic results)
		if result.Error != "" {
			if result.SNIRequired {
				// Emit SNI diagnostic results — these are valuable security intelligence
				if diagJSON, err := json.Marshal(result); err == nil {
					fmt.Fprintln(outputWriter, string(diagJSON))
				}
			}
			errorCount++
			continue
		}

		jsonData, err := json.Marshal(result)
		if err != nil {
			cfg.Logger.Error("failed to marshal result", "error", err)
			continue
		}

		// Write JSON to output
		fmt.Fprintln(outputWriter, string(jsonData))

		// If output file is specified AND status is successful (2XX), print URL to console
		if cfg.OutputFile != "" && result.StatusCode >= 200 && result.StatusCode < 300 {
			fmt.Println(result.FinalURL)
		}

		successCount++
	}

	// Clean up terminal state
	if showProgress && termHeight > 0 {
		// Reset scroll region to full terminal
		fmt.Fprintf(os.Stderr, "\033[r")
		// Clear the status bar line
		fmt.Fprintf(os.Stderr, "\033[%d;1H\033[K", termHeight)
		// Move cursor back to normal position
		fmt.Fprintf(os.Stderr, "\033[%d;1H", termHeight-1)
	}

	cfg.Logger.Info("probing completed",
		"total", len(expandedURLs),
		"success", successCount,
		"errors", errorCount,
	)
}

// readURLs reads URLs from the input reader, skipping comments and empty lines
func readURLs(reader io.Reader) []string {
	var urls []string
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			urls = append(urls, line)
		}
	}
	return urls
}
