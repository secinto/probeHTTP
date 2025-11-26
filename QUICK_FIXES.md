# probeHTTP - Quick Fixes Guide

**For:** Critical and High Priority Issues  
**Time Required:** 2-3 days  
**Impact:** Fixes resource leaks and major bugs

---

## Fix #1: HTTP/3 Transport Resource Leak ⚡ CRITICAL

### Problem
HTTP/3 transport is never closed, causing goroutine and UDP connection leaks.

### Location
`internal/probe/client.go`

### Fix

```go
// Step 1: Update Client struct to track HTTP/3 transport
type Client struct {
    httpClient     *http.Client
    limiters       map[string]*rate.Limiter
    mu             sync.Mutex
    config         *config.Config
    http3Transport *http3.Transport // ADD THIS
}

// Step 2: Update NewHTTP3Client to return transport reference
func NewHTTP3Client(cfg *config.Config, tlsConfig *tls.Config) (*http.Client, *http3.Transport) {
    transport := &http3.Transport{
        TLSClientConfig: tlsConfig,
        QUICConfig:      &quic.Config{},
    }

    httpClient := &http.Client{
        Timeout:   time.Duration(cfg.Timeout) * time.Second,
        Transport: transport,
        CheckRedirect: func(req *http.Request, via []*http.Request) error {
            return http.ErrUseLastResponse
        },
    }

    return httpClient, transport // Return both
}

// Step 3: Add Close method to Client
func (c *Client) Close() error {
    if c.http3Transport != nil {
        return c.http3Transport.Close()
    }
    return nil
}

// Step 4: Update Prober to track and close clients
type Prober struct {
    client      *Client
    config      *config.Config
    stderrMutex sync.Mutex
    cleanupFuncs []func() error // ADD THIS
    mu          sync.Mutex
}

func (p *Prober) Close() error {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    var errs []error
    for _, cleanup := range p.cleanupFuncs {
        if err := cleanup(); err != nil {
            errs = append(errs, err)
        }
    }
    
    if err := p.client.Close(); err != nil {
        errs = append(errs, err)
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("cleanup errors: %v", errs)
    }
    return nil
}

// Step 5: In probeURLWithConfig, track HTTP/3 clients
if protocol == "HTTP/3" {
    httpClient, http3Trans := NewHTTP3Client(p.config, tlsConfig)
    p.mu.Lock()
    p.cleanupFuncs = append(p.cleanupFuncs, func() error {
        return http3Trans.Close()
    })
    p.mu.Unlock()
}

// Step 6: In main.go, add defer
prober := probe.NewProber(cfg)
defer prober.Close()
```

---

## Fix #2: Goroutine Leak in Parallel TLS ⚡ CRITICAL

### Problem
Goroutines may block on channel send after context cancellation.

### Location
`internal/probe/prober.go:395-450` (tryTLSBatch)

### Fix

```go
func (p *Prober) tryTLSBatch(ctx context.Context, probeURL string, originalInput string, strategies []TLSStrategy, protocols []string) output.ProbeResult {
    tlsCtx, cancel := context.WithTimeout(ctx, time.Duration(p.config.TLSHandshakeTimeout)*time.Second)
    defer cancel()

    // CHANGE: Make buffered channel to prevent blocking
    results := make(chan output.ProbeResult, len(strategies))
    var wg sync.WaitGroup

    // Launch parallel attempts
    for i, strategy := range strategies {
        protocol := "HTTP/1.1"
        if i < len(protocols) {
            protocol = protocols[i]
        }

        wg.Add(1)
        go func(s TLSStrategy, proto string) {
            defer wg.Done()
            
            // Try to send result, but don't block if channel is closed
            select {
            case results <- p.probeURLWithConfig(tlsCtx, probeURL, originalInput, s, proto):
            case <-tlsCtx.Done():
                // Context cancelled, don't block
                return
            }
        }(strategy, protocol)
    }

    // Close results channel when all goroutines complete
    go func() {
        wg.Wait()
        close(results)
    }()

    // Collect first successful result
    var firstSuccess output.ProbeResult
    var allErrors []string
    successFound := false

    for result := range results {
        if result.Error == "" && !successFound {
            firstSuccess = result
            successFound = true
            cancel() // Cancel remaining attempts
            
            // IMPORTANT: Drain remaining results to prevent goroutine leaks
            go func() {
                for range results {
                    // Discard remaining results
                }
            }()
            
            return firstSuccess
        } else if result.Error != "" {
            allErrors = append(allErrors, result.Error)
        }
    }

    // All attempts failed
    errorMsg := fmt.Sprintf("All TLS attempts failed: %s", strings.Join(allErrors, "; "))
    return output.ProbeResult{
        Timestamp: time.Now().Format(time.RFC3339),
        Input:     originalInput,
        Method:    "GET",
        Error:     errorMsg,
    }
}
```

---

## Fix #3: Missing Response Body Close ⚡ CRITICAL

### Problem
Response bodies not closed on error paths during redirect handling.

### Location
`internal/probe/prober.go:630-640` and similar locations

### Fix

```go
// Pattern 1: Add defer with nil check
if p.config.FollowRedirects && (resp.StatusCode >= 300 && resp.StatusCode < 400) {
    resp.Body = io.NopCloser(bytes.NewReader(initialBody))
    finalResp, statusChain, hostChain, err = p.followRedirectsWithClient(ctx, resp, p.config.MaxRedirects, 1, initialHostname, &debugBuf, httpClient)
    
    if err != nil {
        result.Error = fmt.Sprintf("Redirect error: %v", err)
        result.ChainStatusCodes = statusChain
        result.ChainHosts = hostChain
        
        // ADD THIS: Close finalResp body if it exists
        if finalResp != nil && finalResp.Body != nil {
            finalResp.Body.Close()
        }
        
        p.flushDebugBuffer(&debugBuf)
        return result
    }
    
    // Existing code continues...
}

// Pattern 2: Use defer right after getting response
resp, err := httpClient.Do(req)
if err != nil {
    // handle error
}
defer func() {
    if resp != nil && resp.Body != nil {
        resp.Body.Close()
    }
}()

// Pattern 3: In redirect.go, ensure cleanup on all paths
func (p *Prober) followRedirectsWithClient(...) (*http.Response, []int, []string, error) {
    // ... existing code ...
    
    for {
        // ... loop code ...
        
        nextResp, err := httpClient.Do(req)
        if err != nil {
            // ADD: Close current response before returning
            if currentResp != nil && currentResp.Body != nil {
                currentResp.Body.Close()
            }
            return currentResp, statusChain, hostChain, fmt.Errorf("redirect request failed: %v", err)
        }
        
        // Continue with nextResp...
    }
}
```

---

## Fix #4: Debug File Logger Cleanup 🟡 HIGH

### Problem
Debug log file handle never closed, causing file descriptor leak.

### Location
`internal/config/config.go`

### Fix

```go
// Step 1: Add file handle to Config
type Config struct {
    // ... existing fields ...
    DebugLogFile    string
    DebugLogger     *slog.Logger
    debugFileHandle *os.File // ADD THIS (private)
}

// Step 2: Update ParseFlags to track handle
func ParseFlags() (*Config, error) {
    cfg := New()
    // ... existing flag parsing ...
    
    if cfg.DebugLogFile != "" {
        debugFile, err := os.Create(cfg.DebugLogFile)
        if err != nil {
            return nil, fmt.Errorf("failed to create debug log file: %v", err)
        }
        cfg.debugFileHandle = debugFile // Track handle
        cfg.DebugLogger = slog.New(slog.NewTextHandler(debugFile, &slog.HandlerOptions{
            Level: slog.LevelDebug,
        }))
        cfg.Logger.Info("debug logging enabled", "file", cfg.DebugLogFile)
    }
    
    return cfg, nil
}

// Step 3: Add Close method
func (c *Config) Close() error {
    if c.debugFileHandle != nil {
        return c.debugFileHandle.Close()
    }
    return nil
}

// Step 4: In main.go
cfg, err := config.ParseFlags()
if err != nil {
    fmt.Fprintf(os.Stderr, "Error: %v\n", err)
    os.Exit(1)
}
defer cfg.Close() // ADD THIS
```

---

## Fix #5: O(n²) URL Deduplication 🟡 HIGH

### Problem
Nested loop causes slow performance with large URL lists.

### Location
`cmd/probehttp/main.go:107-126`

### Fix

```go
// BEFORE (O(n²)):
newOriginalInputMap := make(map[string]string)
for _, urlStr := range expandedURLs {
    if originalInput, exists := originalInputMap[urlStr]; exists {
        newOriginalInputMap[urlStr] = originalInput
    } else {
        normalized := parser.NormalizeURL(urlStr)
        for origURL, origInput := range originalInputMap { // ← O(n) inner loop
            if parser.NormalizeURL(origURL) == normalized {
                newOriginalInputMap[urlStr] = origInput
                break
            }
        }
    }
}

// AFTER (O(n)):
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
    normalized := parser.NormalizeURL(urlStr)
    if origInput, exists := normalizedMap[normalized]; exists {
        newOriginalInputMap[urlStr] = origInput
    } else {
        // Fallback: use the URL itself as input
        newOriginalInputMap[urlStr] = urlStr
    }
}
```

---

## Fix #6: Silent Error Swallowing 🟡 HIGH

### Problem
Errors from io.ReadAll are ignored with `_`.

### Location
Multiple locations in `internal/probe/prober.go`

### Fix

```go
// BEFORE:
initialBody, _ = io.ReadAll(finalResp.Body)

// AFTER:
initialBody, err := io.ReadAll(finalResp.Body)
if err != nil {
    p.config.Logger.Warn("error reading final response body",
        "url", probeURL,
        "error", err,
    )
    // Decide: use partial body, or set error flag
    result.Error = fmt.Sprintf("partial body read: %v", err)
}

// For debug/non-critical reads:
nextBody, err := io.ReadAll(io.LimitReader(bodyReader, p.config.MaxBodySize))
if err != nil {
    p.config.Logger.Debug("error reading body for debug",
        "error", err,
    )
    nextBody = []byte{} // Use empty instead of partial
}
```

---

## Fix #7: Rate Limiter Timeout 🟡 HIGH

### Problem
Rate limiter Wait() can block indefinitely.

### Location
`internal/probe/prober.go:350-359`

### Fix

```go
// Step 1: Add timeout to Config
type Config struct {
    // ... existing fields ...
    RateLimitTimeout int // NEW: in seconds, default 60
}

func New() *Config {
    return &Config{
        // ... existing defaults ...
        RateLimitTimeout: 60, // 60 seconds default
    }
}

// Step 2: Add flag
flag.IntVar(&cfg.RateLimitTimeout, "rate-limit-timeout", 60, "Rate limit wait timeout in seconds")

// Step 3: Use timeout in prober
limiter := p.client.GetLimiter(hostname)

// Create timeout context
waitCtx, waitCancel := context.WithTimeout(ctx, time.Duration(p.config.RateLimitTimeout)*time.Second)
defer waitCancel()

if err := limiter.Wait(waitCtx); err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
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
```

---

## Fix #8: HTTP Client Cleanup 🟡 HIGH

### Problem
Multiple HTTP clients created but never cleaned up.

### Location
`internal/probe/prober.go` (probeURLWithConfig)

### Fix

```go
func (p *Prober) probeURLWithConfig(ctx context.Context, probeURL string, originalInput string, strategy TLSStrategy, protocol string) output.ProbeResult {
    // ... existing code ...
    
    // Create appropriate client
    var httpClient *http.Client
    var cleanup func()
    
    switch protocol {
    case "HTTP/3":
        client, transport := NewHTTP3Client(p.config, tlsConfig)
        httpClient = client
        cleanup = func() {
            transport.Close()
        }
    case "HTTP/2":
        httpClient = NewHTTP2Client(p.config, tlsConfig)
        cleanup = func() {
            if transport, ok := httpClient.Transport.(*http.Transport); ok {
                transport.CloseIdleConnections()
            }
        }
    case "HTTP/1.1":
        httpClient = NewHTTP11Client(p.config, tlsConfig)
        cleanup = func() {
            if transport, ok := httpClient.Transport.(*http.Transport); ok {
                transport.CloseIdleConnections()
            }
        }
    }
    
    // ADD: Defer cleanup
    if cleanup != nil {
        defer cleanup()
    }
    
    // ... rest of function ...
}
```

---

## Testing After Fixes

```bash
# 1. Build with race detector
go build -race -o probeHTTP-race ./cmd/probehttp

# 2. Test for goroutine leaks
cat > test_leak.sh << 'SCRIPT'
#!/bin/bash
echo "Testing for goroutine leaks..."

# Create test URLs
for i in {1..100}; do
    echo "https://example.com:$i"
done > test_urls.txt

# Run with profiling
./probeHTTP-race -i test_urls.txt -o /dev/null 2>&1 | grep -i "race\|leak\|warning"

# Check goroutine count
go tool pprof -alloc_space probeHTTP cpu.prof
SCRIPT

chmod +x test_leak.sh
./test_leak.sh

# 3. Test with limited file descriptors
ulimit -n 128
echo "https://example.com" | ./probeHTTP --ports "1-1000"
# Should not fail with "too many open files"

# 4. Memory leak test
go test -run TestConcurrent -count=100 -v ./test/
# Monitor memory with: watch -n 1 'ps aux | grep probeHTTP'

# 5. Load test
time echo "example.com" | ./probeHTTP --ports "1-10000" -c 100
# Should complete without errors
```

---

## Verification Checklist

- [ ] All HTTP/3 transports are closed
- [ ] No goroutine leaks in parallel TLS
- [ ] All response bodies closed on error paths
- [ ] Debug log file is closed
- [ ] URL deduplication is O(n)
- [ ] All io.ReadAll errors are checked
- [ ] Rate limiter has timeout
- [ ] HTTP clients are cleaned up
- [ ] Race detector shows no issues
- [ ] Memory usage is stable
- [ ] File descriptors don't leak

---

## Deployment Steps

1. Apply all fixes
2. Run full test suite
3. Run race detector tests
4. Run load tests
5. Monitor in staging for 24h
6. Deploy to production
7. Monitor goroutine count
8. Monitor memory usage
9. Monitor file descriptors

---

**Time Estimate:** 2-3 days for implementation + testing  
**Risk Level After Fixes:** LOW ✅
