package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/twmb/murmur3"
	htmlparser "golang.org/x/net/html"
)

// Default common HTTP ports for probing
var DefaultHTTPPorts = []string{"80", "8000", "8080", "8888"}

// Default common HTTPS ports for probing
var DefaultHTTPSPorts = []string{"443", "8443", "10443", "8444"}

// Default User-Agent (latest Chrome on Windows 10)
var DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// Pool of common browser User-Agents for random selection
var UserAgentPool = []string{
	// Chrome on Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	// Chrome on macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	// Chrome on Linux
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
	// Firefox on Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
	// Firefox on macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:120.0) Gecko/20100101 Firefox/120.0",
	// Firefox on Linux
	"Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
	"Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:120.0) Gecko/20100101 Firefox/120.0",
	// Safari on macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15",
	// Edge on Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0",
	// Mobile Chrome on Android
	"Mozilla/5.0 (Linux; Android 13; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
	"Mozilla/5.0 (Linux; Android 13; Pixel 7 Pro) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
	// Mobile Safari on iPhone
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_1_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Mobile/15E148 Safari/604.1",
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1",
}

// Config holds the CLI configuration
type Config struct {
	InputFile       string
	OutputFile      string
	FollowRedirects bool
	MaxRedirects    int
	Timeout         int
	Concurrency     int
	Silent          bool
	Debug           bool   // Debug mode (show all requests and responses)
	SameHostOnly    bool   // Only follow redirects to same hostname
	UserAgent       string // Custom User-Agent header
	RandomUserAgent bool   // Use random User-Agent from pool
	AllSchemes      bool   // Test both HTTP and HTTPS schemes
	IgnorePorts     bool   // Ignore input ports and test common ports
	CustomPorts     string // Custom port list (comma-separated, supports ranges)
}

// Hash contains MMH3 hashes
type Hash struct {
	BodyMMH3   string `json:"body_mmh3"`
	HeaderMMH3 string `json:"header_mmh3"`
}

// ProbeResult represents the JSON output for each probed URL
type ProbeResult struct {
	Timestamp        string `json:"timestamp"`
	Hash             Hash   `json:"hash"`
	Port             string `json:"port"`
	URL              string `json:"url"`
	Input            string `json:"input"`
	FinalURL         string `json:"final_url"`
	Title            string `json:"title"`
	Scheme           string `json:"scheme"`
	WebServer        string `json:"webserver"`
	ContentType      string `json:"content_type"`
	Method           string `json:"method"`
	Host             string `json:"host"`
	Path             string   `json:"path"`
	Time             string   `json:"time"`
	ChainStatusCodes []int    `json:"chain_status_codes"`
	ChainHosts       []string `json:"chain_hosts"`
	Words            int      `json:"words"`
	Lines            int    `json:"lines"`
	StatusCode       int    `json:"status_code"`
	ContentLength    int    `json:"content_length"`
	Error            string `json:"error,omitempty"`
}

// ParsedURL holds the components of a parsed input URL
type ParsedURL struct {
	Original string // Original input string
	Scheme   string // http, https, or empty if not specified
	Host     string // hostname only (no port)
	Port     string // port number or empty
	Path     string // path component (default "/")
}

var config Config

// Mutex for atomic stderr writes when flushing debug buffers
var stderrMutex sync.Mutex

func init() {
	// Define CLI flags
	flag.StringVar(&config.InputFile, "i", "", "Input file (default: stdin)")
	flag.StringVar(&config.InputFile, "input", "", "Input file (default: stdin)")
	flag.StringVar(&config.OutputFile, "o", "", "Output file (default: stdout)")
	flag.StringVar(&config.OutputFile, "output", "", "Output file (default: stdout)")
	flag.BoolVar(&config.FollowRedirects, "fr", true, "Follow redirects")
	flag.BoolVar(&config.FollowRedirects, "follow-redirects", true, "Follow redirects")
	flag.IntVar(&config.MaxRedirects, "maxr", 10, "Max redirects")
	flag.IntVar(&config.MaxRedirects, "max-redirects", 10, "Max redirects")
	flag.IntVar(&config.Timeout, "t", 30, "Request timeout in seconds")
	flag.IntVar(&config.Timeout, "timeout", 30, "Request timeout in seconds")
	flag.IntVar(&config.Concurrency, "c", 10, "Concurrent requests")
	flag.IntVar(&config.Concurrency, "concurrency", 10, "Concurrent requests")
	flag.BoolVar(&config.Silent, "silent", false, "Silent mode (no errors to stderr)")
	flag.BoolVar(&config.Debug, "d", false, "Debug mode (show all requests and responses to stderr)")
	flag.BoolVar(&config.Debug, "debug", false, "Debug mode (show all requests and responses to stderr)")
	flag.BoolVar(&config.SameHostOnly, "sho", false, "Only follow redirects to same hostname (different scheme/port/path allowed)")
	flag.BoolVar(&config.SameHostOnly, "same-host-only", false, "Only follow redirects to same hostname (different scheme/port/path allowed)")
	flag.StringVar(&config.UserAgent, "ua", "", "Custom User-Agent header (mutually exclusive with -rua)")
	flag.StringVar(&config.UserAgent, "user-agent", "", "Custom User-Agent header (mutually exclusive with -rua)")
	flag.BoolVar(&config.RandomUserAgent, "rua", false, "Use random User-Agent from pool (mutually exclusive with -ua)")
	flag.BoolVar(&config.RandomUserAgent, "random-user-agent", false, "Use random User-Agent from pool (mutually exclusive with -ua)")
	flag.BoolVar(&config.AllSchemes, "as", false, "Test both HTTP and HTTPS schemes (overrides input scheme)")
	flag.BoolVar(&config.AllSchemes, "all-schemes", false, "Test both HTTP and HTTPS schemes (overrides input scheme)")
	flag.BoolVar(&config.IgnorePorts, "ip", false, "Ignore input ports and test common HTTP/HTTPS ports")
	flag.BoolVar(&config.IgnorePorts, "ignore-ports", false, "Ignore input ports and test common HTTP/HTTPS ports")
	flag.StringVar(&config.CustomPorts, "p", "", "Custom port list (comma-separated, supports ranges like 8000-8010)")
	flag.StringVar(&config.CustomPorts, "ports", "", "Custom port list (comma-separated, supports ranges like 8000-8010)")
}

func main() {
	flag.Parse()

	// Validate mutually exclusive flags
	if config.UserAgent != "" && config.RandomUserAgent {
		fmt.Fprintln(os.Stderr, "Error: -ua/--user-agent and -rua/--random-user-agent are mutually exclusive")
		os.Exit(1)
	}

	// Get input reader
	var inputReader io.Reader
	if config.InputFile != "" {
		file, err := os.Open(config.InputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error opening input file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		inputReader = file
	} else {
		inputReader = os.Stdin
	}

	// Get output writer
	var outputWriter io.Writer
	if config.OutputFile != "" {
		file, err := os.Create(config.OutputFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error creating output file: %v\n", err)
			os.Exit(1)
		}
		defer file.Close()
		outputWriter = file
	} else {
		outputWriter = os.Stdout
	}

	// Read URLs from input
	urls := readURLs(inputReader)

	// Expand URLs based on scheme and port configuration
	// Create mapping from expanded URL to original input for traceability
	expandedURLs := []string{}
	originalInputMap := make(map[string]string) // expanded URL -> original input
	for _, inputURL := range urls {
		expanded := expandURLs(inputURL)
		for _, expandedURL := range expanded {
			expandedURLs = append(expandedURLs, expandedURL)
			originalInputMap[expandedURL] = inputURL
		}
	}

	// Process expanded URLs with worker pool
	results := processURLs(expandedURLs, originalInputMap, config.Concurrency)

	// Write results
	for result := range results {
		jsonData, err := json.Marshal(result)
		if err != nil {
			if !config.Silent {
				fmt.Fprintf(os.Stderr, "Error marshaling result: %v\n", err)
			}
			continue
		}

		// Write JSON to output file/stdout
		fmt.Fprintln(outputWriter, string(jsonData))

		// If output file is specified AND status is successful (2XX), print URL to console
		if config.OutputFile != "" && result.StatusCode >= 200 && result.StatusCode < 300 {
			fmt.Println(result.FinalURL)
		}
	}
}

// readURLs reads URLs from the input reader
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

// processURLs processes URLs concurrently using a worker pool
func processURLs(urls []string, originalInputMap map[string]string, concurrency int) <-chan ProbeResult {
	results := make(chan ProbeResult, len(urls))
	urlChan := make(chan string, len(urls))

	// Create worker pool
	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go worker(urlChan, results, originalInputMap, &wg)
	}

	// Send URLs to workers
	go func() {
		for _, url := range urls {
			urlChan <- url
		}
		close(urlChan)
	}()

	// Close results channel when all workers are done
	go func() {
		wg.Wait()
		close(results)
	}()

	return results
}

// worker processes URLs from the channel
func worker(urls <-chan string, results chan<- ProbeResult, originalInputMap map[string]string, wg *sync.WaitGroup) {
	defer wg.Done()

	client := createHTTPClient()

	for expandedURL := range urls {
		originalInput := originalInputMap[expandedURL]
		result := probeURL(expandedURL, originalInput, client)
		results <- result
	}
}

// createHTTPClient creates an HTTP client with configured settings
func createHTTPClient() *http.Client {
	client := &http.Client{
		Timeout: time.Duration(config.Timeout) * time.Second,
	}

	// Always disable automatic redirects - we'll handle them manually to capture the chain
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	return client
}

// getUserAgent returns the appropriate User-Agent based on configuration
func getUserAgent() string {
	// Priority: Custom UA > Random UA > Default UA
	if config.UserAgent != "" {
		return config.UserAgent
	}
	if config.RandomUserAgent {
		return UserAgentPool[rand.Intn(len(UserAgentPool))]
	}
	return DefaultUserAgent
}

// debugPrintSeparator prints a visual separator for debug output
func debugPrintSeparator(buf *strings.Builder) {
	if !config.Debug {
		return
	}
	line := "========================================\n"
	if buf != nil {
		buf.WriteString(line)
	} else {
		fmt.Fprint(os.Stderr, line)
	}
}

// debugRequest logs HTTP request details in debug mode
func debugRequest(req *http.Request, stepNum int, buf *strings.Builder) {
	if !config.Debug {
		return
	}

	var out strings.Builder
	fmt.Fprintf(&out, "[%d] REQUEST: %s %s\n", stepNum, req.Method, req.URL.String())

	if len(req.Header) > 0 {
		fmt.Fprintln(&out, "Headers:")
		// Sort headers for consistent output
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
	} else {
		fmt.Fprint(os.Stderr, out.String())
	}
}

// debugResponse logs HTTP response details in debug mode
func debugResponse(resp *http.Response, body []byte, elapsed time.Duration, stepNum int, buf *strings.Builder) {
	if !config.Debug {
		return
	}

	var out strings.Builder
	fmt.Fprintf(&out, "[%d] RESPONSE: %d %s (%s)\n", stepNum, resp.StatusCode, resp.Status, elapsed)

	if len(resp.Header) > 0 {
		fmt.Fprintln(&out, "Headers:")
		// Sort headers for consistent output
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

	// Show Location header prominently for redirects
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		location := resp.Header.Get("Location")
		if location != "" {
			fmt.Fprintf(&out, "  → Redirecting to: %s\n", location)
		}
	}

	// Show body preview if available
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
	} else {
		fmt.Fprint(os.Stderr, out.String())
	}
}

// followRedirects manually follows HTTP redirects and captures the status code and host chains
// Returns the final response, complete status code chain, host chain, and any error
func followRedirects(initialResp *http.Response, client *http.Client, maxRedirects int, startStep int, initialHostname string, buf *strings.Builder) (*http.Response, []int, []string, error) {
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

		// Extract hostname from next URL
		nextHostname := nextURL.Hostname()

		// Check if same-host-only mode is enabled and hostname changed
		if config.SameHostOnly && nextHostname != initialHostname {
			// Cross-host redirect detected - stop following
			if config.Debug {
				warning := fmt.Sprintf("  ⚠ Cross-host redirect blocked: %s → %s (same-host-only mode)\n", initialHostname, nextHostname)
				if buf != nil {
					buf.WriteString(warning)
				} else {
					fmt.Fprint(os.Stderr, warning)
				}
			}
			return currentResp, statusChain, hostChain, fmt.Errorf("cross-host redirect blocked: %s → %s", initialHostname, nextHostname)
		}

		// Make request to next URL
		req, err := http.NewRequest("GET", nextURL.String(), nil)
		if err != nil {
			return currentResp, statusChain, hostChain, fmt.Errorf("failed to create redirect request: %v", err)
		}

		// Copy headers from original request
		req.Header = currentResp.Request.Header

		// Debug: log redirect request with cross-host warning
		stepNum++
		debugRequest(req, stepNum, buf)
		if config.Debug && nextHostname != initialHostname {
			warning := fmt.Sprintf("  ⚠ Cross-host redirect: %s → %s\n", initialHostname, nextHostname)
			if buf != nil {
				buf.WriteString(warning)
			} else {
				fmt.Fprint(os.Stderr, warning)
			}
		}

		// Execute request
		requestStart := time.Now()
		nextResp, err := client.Do(req)
		requestElapsed := time.Since(requestStart)
		if err != nil {
			return currentResp, statusChain, hostChain, fmt.Errorf("redirect request failed: %v", err)
		}

		// Read body for debug logging
		var nextBody []byte
		if config.Debug {
			nextBody, _ = io.ReadAll(nextResp.Body)
			nextResp.Body.Close()
			// Recreate body for further processing
			nextResp.Body = io.NopCloser(strings.NewReader(string(nextBody)))
		}

		// Debug: log redirect response
		debugResponse(nextResp, nextBody, requestElapsed, stepNum, buf)

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

// probeURL performs the HTTP probe for a single URL
func probeURL(probeURL string, originalInput string, client *http.Client) ProbeResult {
	// Create debug buffer for collecting all debug output for this URL
	var debugBuf strings.Builder

	result := ProbeResult{
		Timestamp: time.Now().Format(time.RFC3339),
		Input:     originalInput, // Preserve original input for traceability
		Method:    "GET",
	}

	// Ensure URL has scheme (should already be present from expansion, but check anyway)
	if !strings.HasPrefix(probeURL, "http://") && !strings.HasPrefix(probeURL, "https://") {
		probeURL = "http://" + probeURL
	}

	// Parse URL to validate it
	_, err := url.Parse(probeURL)
	if err != nil {
		result.Error = fmt.Sprintf("Invalid URL: %v", err)
		if !config.Silent {
			fmt.Fprintf(os.Stderr, "Error parsing URL %s: %v\n", probeURL, err)
		}
		// Flush debug buffer before returning
		if debugBuf.Len() > 0 {
			stderrMutex.Lock()
			fmt.Fprint(os.Stderr, debugBuf.String())
			stderrMutex.Unlock()
		}
		return result
	}

	// Debug: print separator at start of probe
	debugPrintSeparator(&debugBuf)

	// Create HTTP request
	req, err := http.NewRequest("GET", probeURL, nil)
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create request: %v", err)
		if !config.Silent {
			fmt.Fprintf(os.Stderr, "Error creating request for %s: %v\n", probeURL, err)
		}
		// Flush debug buffer before returning
		if debugBuf.Len() > 0 {
			stderrMutex.Lock()
			fmt.Fprint(os.Stderr, debugBuf.String())
			stderrMutex.Unlock()
		}
		return result
	}

	// Set browser-like headers
	req.Header.Set("User-Agent", getUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	// Note: Accept-Encoding is intentionally not set here. Go's http.Client
	// automatically handles gzip compression when Accept-Encoding is not manually set.

	// Debug: log initial request
	debugRequest(req, 1, &debugBuf)

	// Make HTTP request
	startTime := time.Now()
	resp, err := client.Do(req)
	elapsed := time.Since(startTime)

	if err != nil {
		result.Error = fmt.Sprintf("Request failed: %v", err)
		if !config.Silent {
			fmt.Fprintf(os.Stderr, "Error requesting URL %s: %v\n", probeURL, err)
		}
		debugPrintSeparator(&debugBuf)
		// Flush debug buffer before returning
		if debugBuf.Len() > 0 {
			stderrMutex.Lock()
			fmt.Fprint(os.Stderr, debugBuf.String())
			stderrMutex.Unlock()
		}
		return result
	}

	// Read initial response body for debug logging
	var initialBody []byte
	if config.Debug {
		initialBody, _ = io.ReadAll(resp.Body)
		resp.Body.Close()
		// Recreate body for further processing
		resp.Body = io.NopCloser(strings.NewReader(string(initialBody)))
	}

	// Debug: log initial response
	debugResponse(resp, initialBody, elapsed, 1, &debugBuf)

	// Extract initial hostname for redirect tracking
	parsedProbeURL, _ := url.Parse(probeURL)
	initialHostname := parsedProbeURL.Hostname()

	// Follow redirects manually if enabled to capture the chain
	var finalResp *http.Response
	var statusChain []int
	var hostChain []string

	if config.FollowRedirects && (resp.StatusCode >= 300 && resp.StatusCode < 400) {
		// Response is a redirect and we should follow it
		finalResp, statusChain, hostChain, err = followRedirects(resp, client, config.MaxRedirects, 1, initialHostname, &debugBuf)
		if err != nil {
			result.Error = fmt.Sprintf("Redirect error: %v", err)
			result.ChainStatusCodes = statusChain // Still include partial chain
			result.ChainHosts = hostChain         // Still include partial host chain
			if !config.Silent {
				fmt.Fprintf(os.Stderr, "Error following redirects for %s: %v\n", probeURL, err)
			}
			debugPrintSeparator(&debugBuf)
			// Flush debug buffer before returning
			if debugBuf.Len() > 0 {
				stderrMutex.Lock()
				fmt.Fprint(os.Stderr, debugBuf.String())
				stderrMutex.Unlock()
			}
			return result
		}
	} else {
		// Not a redirect or not following redirects
		finalResp = resp
		statusChain = []int{resp.StatusCode}
		hostChain = []string{initialHostname}
	}
	defer finalResp.Body.Close()

	// Debug: print separator at end of probe
	debugPrintSeparator(&debugBuf)

	// Read response body from final response
	body, err := io.ReadAll(finalResp.Body)
	if err != nil {
		result.Error = fmt.Sprintf("Error reading body: %v", err)
		if !config.Silent {
			fmt.Fprintf(os.Stderr, "Error reading body for %s: %v\n", probeURL, err)
		}
		// Flush debug buffer before returning
		if debugBuf.Len() > 0 {
			stderrMutex.Lock()
			fmt.Fprint(os.Stderr, debugBuf.String())
			stderrMutex.Unlock()
		}
		return result
	}

	// Extract final URL after redirects
	finalURL := finalResp.Request.URL.String()
	finalParsedURL := finalResp.Request.URL

	// Calculate hashes
	result.Hash.BodyMMH3 = calculateMMH3(body)
	result.Hash.HeaderMMH3 = calculateHeaderMMH3(finalResp.Header)

	// Extract metadata
	result.URL = probeURL                 // Original probe URL
	result.FinalURL = finalURL            // Final URL after redirects
	result.ChainStatusCodes = statusChain // Complete redirect status chain
	result.ChainHosts = hostChain         // Complete redirect host chain
	result.StatusCode = finalResp.StatusCode
	result.ContentLength = len(body)
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
	bodyStr := string(body)
	result.Title = extractTitle(bodyStr)
	result.Words, result.Lines = countWordsAndLines(bodyStr)

	// Flush debug buffer atomically to stderr
	if debugBuf.Len() > 0 {
		stderrMutex.Lock()
		fmt.Fprint(os.Stderr, debugBuf.String())
		stderrMutex.Unlock()
	}

	return result
}

// calculateMMH3 calculates the MMH3 hash of the data
func calculateMMH3(data []byte) string {
	hash := murmur3.Sum32(data)
	return fmt.Sprintf("%d", hash)
}

// calculateHeaderMMH3 calculates the MMH3 hash of concatenated headers
func calculateHeaderMMH3(headers http.Header) string {
	// Sort headers for consistent hashing
	var keys []string
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Concatenate headers
	var headerStr strings.Builder
	for _, k := range keys {
		for _, v := range headers[k] {
			headerStr.WriteString(k)
			headerStr.WriteString(": ")
			headerStr.WriteString(v)
			headerStr.WriteString("\n")
		}
	}

	return calculateMMH3([]byte(headerStr.String()))
}

// decodeTitleString decodes HTML entities and Unicode escapes in a title string
// Handles both \uXXXX Unicode escapes and HTML entities like &amp;, &#38;, etc.
// Uses standard library functions: regexp for Unicode escapes, html.UnescapeString for HTML entities
func decodeTitleString(s string) string {
	// First, decode Unicode escapes like \u0026
	// Use regexp to find and replace \uXXXX patterns
	unicodeEscapeRegex := regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)
	s = unicodeEscapeRegex.ReplaceAllStringFunc(s, func(match string) string {
		// Extract hex value from \uXXXX
		hex := strings.TrimPrefix(match, `\u`)
		var r rune
		fmt.Sscanf(hex, "%x", &r)
		// Validate the rune before converting
		if utf8.ValidRune(r) {
			return string(r)
		}
		// If invalid, return original match
		return match
	})

	// Then, decode HTML entities like &amp;, &#38;, &#x26;
	s = html.UnescapeString(s)

	return s
}

// extractTitle extracts the HTML title from the body with fallbacks
// Priority: 1) <title> tag, 2) og:title meta tag, 3) twitter:title meta tag
func extractTitle(body string) string {
	doc, err := htmlparser.Parse(strings.NewReader(body))
	if err != nil {
		return ""
	}

	var htmlTitle string
	var ogTitle string
	var twitterTitle string

	var traverse func(*htmlparser.Node)
	traverse = func(n *htmlparser.Node) {
		if n.Type == htmlparser.ElementNode {
			// Check for <title> tag
			if n.Data == "title" && htmlTitle == "" {
				if n.FirstChild != nil {
					htmlTitle = n.FirstChild.Data
				}
			}

			// Check for <meta> tags with property or name attributes
			if n.Data == "meta" {
				var property, name, content string
				for _, attr := range n.Attr {
					switch attr.Key {
					case "property":
						property = attr.Val
					case "name":
						name = attr.Val
					case "content":
						content = attr.Val
					}
				}

				// Check for Open Graph title
				if property == "og:title" && ogTitle == "" {
					ogTitle = content
				}

				// Check for Twitter Card title
				if name == "twitter:title" && twitterTitle == "" {
					twitterTitle = content
				}
			}
		}

		// Continue traversing child nodes
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(doc)

	// Return first non-empty title in priority order, decoded
	if htmlTitle != "" {
		return decodeTitleString(strings.TrimSpace(htmlTitle))
	}
	if ogTitle != "" {
		return decodeTitleString(strings.TrimSpace(ogTitle))
	}
	if twitterTitle != "" {
		return decodeTitleString(strings.TrimSpace(twitterTitle))
	}

	return ""
}

// countWordsAndLines counts words and lines in the text
func countWordsAndLines(text string) (words int, lines int) {
	lines = strings.Count(text, "\n") + 1
	if text == "" {
		lines = 0
	}

	// Count words
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Split(bufio.ScanWords)
	for scanner.Scan() {
		words++
	}

	return words, lines
}

// parsePortList parses a comma-separated port list with support for ranges
// Examples:
//   "80,443,8080" → [80, 443, 8080]
//   "8000-8005" → [8000, 8001, 8002, 8003, 8004, 8005]
//   "80,443,8000-8010" → [80, 443, 8000, 8001, ..., 8010]
func parsePortList(portStr string) ([]string, error) {
	if portStr == "" {
		return nil, fmt.Errorf("empty port list")
	}

	portMap := make(map[string]bool) // Use map for deduplication
	parts := strings.Split(portStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if it's a range (e.g., "8000-8010")
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid port range format: %s", part)
			}

			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid start port in range %s: %v", part, err)
			}

			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid end port in range %s: %v", part, err)
			}

			// Validate port range
			if start < 1 || start > 65535 {
				return nil, fmt.Errorf("start port out of range (1-65535): %d", start)
			}
			if end < 1 || end > 65535 {
				return nil, fmt.Errorf("end port out of range (1-65535): %d", end)
			}
			if start > end {
				return nil, fmt.Errorf("invalid range %s: start port > end port", part)
			}

			// Add all ports in range
			for port := start; port <= end; port++ {
				portMap[strconv.Itoa(port)] = true
			}
		} else {
			// Single port
			port, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid port number: %s", part)
			}

			// Validate port
			if port < 1 || port > 65535 {
				return nil, fmt.Errorf("port out of range (1-65535): %d", port)
			}

			portMap[part] = true
		}
	}

	// Check if we got any valid ports
	if len(portMap) == 0 {
		return nil, fmt.Errorf("no valid ports found in port list")
	}

	// Convert map to sorted slice
	ports := make([]string, 0, len(portMap))
	for port := range portMap {
		ports = append(ports, port)
	}

	// Sort ports numerically
	sort.Slice(ports, func(i, j int) bool {
		pi, _ := strconv.Atoi(ports[i])
		pj, _ := strconv.Atoi(ports[j])
		return pi < pj
	})

	return ports, nil
}

// parseInputURL parses an input URL string and extracts its components
func parseInputURL(inputURL string) ParsedURL {
	parsed := ParsedURL{
		Original: inputURL,
		Path:     "/",
	}

	// Check if input has a scheme
	hasScheme := strings.HasPrefix(inputURL, "http://") || strings.HasPrefix(inputURL, "https://")

	var u *url.URL
	var err error

	if hasScheme {
		// Parse as full URL
		u, err = url.Parse(inputURL)
		if err != nil {
			// If parsing fails, treat whole string as host
			parsed.Host = inputURL
			return parsed
		}

		parsed.Scheme = u.Scheme
		parsed.Host = u.Hostname()
		parsed.Port = u.Port()

		// Preserve full path including query and fragment
		if u.Path != "" {
			parsed.Path = u.Path
		}
		if u.RawQuery != "" {
			parsed.Path += "?" + u.RawQuery
		}
		if u.Fragment != "" {
			parsed.Path += "#" + u.Fragment
		}
	} else {
		// No scheme - could be "host", "host:port", "host/path", or "host:port/path"
		// First, check if there's a path separator
		pathStart := strings.Index(inputURL, "/")
		queryStart := strings.Index(inputURL, "?")
		fragmentStart := strings.Index(inputURL, "#")

		// Find where host:port ends
		hostPortEnd := len(inputURL)
		if pathStart >= 0 {
			hostPortEnd = pathStart
		}
		if queryStart >= 0 && queryStart < hostPortEnd {
			hostPortEnd = queryStart
		}
		if fragmentStart >= 0 && fragmentStart < hostPortEnd {
			hostPortEnd = fragmentStart
		}

		hostPort := inputURL[:hostPortEnd]
		rest := ""
		if hostPortEnd < len(inputURL) {
			rest = inputURL[hostPortEnd:]
		}

		// Parse host:port
		if strings.Contains(hostPort, ":") {
			parts := strings.SplitN(hostPort, ":", 2)
			parsed.Host = parts[0]
			// Check if second part looks like a port number
			portStr := parts[1]
			if port, err := strconv.Atoi(portStr); err == nil && port >= 1 && port <= 65535 {
				parsed.Port = portStr
			} else {
				// Doesn't look like a port, treat whole thing as host
				parsed.Host = hostPort
			}
		} else {
			parsed.Host = hostPort
		}

		// Preserve path, query, and fragment
		if rest != "" {
			parsed.Path = rest
		}
	}

	return parsed
}

// getSchemesToTest returns the list of schemes to test based on configuration and input
func getSchemesToTest(parsed ParsedURL) []string {
	// If AllSchemes flag is set, always test both
	if config.AllSchemes {
		return []string{"http", "https"}
	}

	// If no scheme in input, test both by default
	if parsed.Scheme == "" {
		return []string{"http", "https"}
	}

	// Use the scheme from input
	return []string{parsed.Scheme}
}

// getPortsToTest returns the list of ports to test based on configuration and input
func getPortsToTest(parsed ParsedURL, scheme string) []string {
	// Custom ports override everything
	if config.CustomPorts != "" {
		ports, err := parsePortList(config.CustomPorts)
		if err != nil {
			if !config.Silent {
				fmt.Fprintf(os.Stderr, "Error parsing custom ports: %v\n", err)
			}
			// Fall back to default behavior
		} else {
			return ports
		}
	}

	// If IgnorePorts is set, use default common ports for the scheme
	if config.IgnorePorts {
		if scheme == "https" {
			return DefaultHTTPSPorts
		}
		return DefaultHTTPPorts
	}

	// If port is specified in input, use it
	if parsed.Port != "" {
		return []string{parsed.Port}
	}

	// Default: use standard port for the scheme
	if scheme == "https" {
		return []string{"443"}
	}
	return []string{"80"}
}

// shouldIncludePortInURL determines if the port should be explicitly included in the URL
func shouldIncludePortInURL(parsed ParsedURL, port string, scheme string) bool {
	// Always include port if port-related flags are active
	if config.IgnorePorts || config.CustomPorts != "" {
		return true
	}

	// Always include port if it was explicitly specified in the original input
	if parsed.Port != "" {
		return true
	}

	// Include port if it's non-standard for the scheme
	standardPort := "80"
	if scheme == "https" {
		standardPort = "443"
	}

	return port != standardPort
}

// buildProbeURL builds a URL string with optional port inclusion
func buildProbeURL(scheme string, host string, port string, path string, includePort bool) string {
	if includePort {
		return fmt.Sprintf("%s://%s:%s%s", scheme, host, port, path)
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, path)
}

// expandURLs takes an input URL and returns all URLs to probe based on configuration
func expandURLs(inputURL string) []string {
	parsed := parseInputURL(inputURL)
	schemes := getSchemesToTest(parsed)

	urlMap := make(map[string]bool) // For deduplication
	var urls []string

	for _, scheme := range schemes {
		ports := getPortsToTest(parsed, scheme)

		for _, port := range ports {
			// Determine if port should be included in URL
			includePort := shouldIncludePortInURL(parsed, port, scheme)

			// Build the URL
			urlStr := buildProbeURL(scheme, parsed.Host, port, parsed.Path, includePort)

			// Deduplicate
			if !urlMap[urlStr] {
				urlMap[urlStr] = true
				urls = append(urls, urlStr)
			}
		}
	}

	return urls
}
