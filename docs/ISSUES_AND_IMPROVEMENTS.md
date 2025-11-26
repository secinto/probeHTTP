# probeHTTP - Issues and Improvements Analysis

**Date:** November 26, 2024  
**Version:** 2.0  
**Analysis Type:** Code Quality, Security, Performance, and Architecture Review

---

## Executive Summary

The codebase is generally well-structured with good test coverage (~2,971 lines of tests) and modern Go practices. However, there are several critical issues and opportunities for improvement across security, performance, resource management, and error handling.

**Severity Levels:**
- 🔴 **Critical** - Must fix (security/data loss)
- 🟡 **High** - Should fix (bugs/performance)
- 🟢 **Medium** - Nice to have (code quality)
- 🔵 **Low** - Enhancement (future improvements)

---

## Critical Issues 🔴

### 1. Resource Leak: HTTP/3 Transport Not Closed

**Location:** `internal/probe/client.go:119-131` (NewHTTP3Client)

**Issue:**
```go
transport := &http3.Transport{
    TLSClientConfig: tlsConfig,
    QUICConfig:      &quic.Config{},
}
```

HTTP/3 transports maintain UDP connections and goroutines that need explicit cleanup. The transport is never closed, causing resource leaks.

**Impact:**
- Memory leaks in long-running processes
- File descriptor exhaustion
- Goroutine leaks

**Fix:**
```go
// Add Close method to Client struct
type Client struct {
    httpClient *http.Client
    limiters   map[string]*rate.Limiter
    mu         sync.Mutex
    config     *config.Config
    http3Transport *http3.Transport // Track HTTP/3 transport
}

// Close method to cleanup resources
func (c *Client) Close() error {
    if c.http3Transport != nil {
        return c.http3Transport.Close()
    }
    return nil
}

// In main.go
defer prober.Close()
```

**Priority:** CRITICAL - Fix immediately

---

### 2. Potential Goroutine Leak in Parallel TLS Attempts

**Location:** `internal/probe/prober.go:395-450` (tryTLSBatch)

**Issue:**
```go
for result := range results {
    if result.Error == "" && !successFound {
        firstSuccess = result
        successFound = true
        cancel() // Cancel remaining attempts
    } else if result.Error != "" {
        allErrors = append(allErrors, result.Error)
    }
}
```

When `cancel()` is called, remaining goroutines may be blocked on channel sends if the results channel is full. The goroutines will leak if they can't send.

**Impact:**
- Goroutine accumulation over time
- Memory growth
- Potential deadlocks

**Fix:**
```go
// Make results channel buffered to match goroutine count
results := make(chan output.ProbeResult, len(strategies))

// AND drain channel after finding success
if successFound {
    // Drain remaining results to prevent goroutine leaks
    go func() {
        for range results {
            // Discard remaining results
        }
    }()
    return firstSuccess
}
```

**Priority:** CRITICAL - Fix immediately

---

### 3. Missing Response Body Close on Redirect Error Path

**Location:** `internal/probe/prober.go:630-640` and similar locations

**Issue:**
```go
if p.config.FollowRedirects && (resp.StatusCode >= 300 && resp.StatusCode < 400) {
    resp.Body = io.NopCloser(bytes.NewReader(initialBody))
    finalResp, statusChain, hostChain, err = p.followRedirectsWithClient(...)
    if err != nil {
        result.Error = fmt.Sprintf("Redirect error: %v", err)
        // Missing: finalResp.Body.Close() if finalResp exists
        return result
    }
}
```

**Impact:**
- Connection leaks
- File descriptor exhaustion
- Memory leaks

**Fix:**
```go
if err != nil {
    result.Error = fmt.Sprintf("Redirect error: %v", err)
    result.ChainStatusCodes = statusChain
    result.ChainHosts = hostChain
    if finalResp != nil && finalResp.Body != nil {
        finalResp.Body.Close()
    }
    p.flushDebugBuffer(&debugBuf)
    return result
}
```

**Priority:** CRITICAL - Fix immediately

---

## High Priority Issues 🟡

### 4. Race Condition in Debug File Logger

**Location:** `internal/config/config.go:119-126`

**Issue:**
```go
if cfg.DebugLogFile != "" {
    debugFile, err := os.Create(cfg.DebugLogFile)
    if err != nil {
        return nil, fmt.Errorf("failed to create debug log file: %v", err)
    }
    cfg.DebugLogger = slog.New(slog.NewTextHandler(debugFile, ...))
}
```

The file is never closed, and concurrent writes from multiple goroutines could corrupt the log file.

**Impact:**
- File descriptor leak
- Corrupted debug logs
- Data loss

**Fix:**
```go
// Add file handle to Config
type Config struct {
    // ... existing fields
    DebugLogFile   string
    DebugLogger    *slog.Logger
    debugFileHandle *os.File // NEW
}

// In ParseFlags
if cfg.DebugLogFile != "" {
    debugFile, err := os.Create(cfg.DebugLogFile)
    if err != nil {
        return nil, fmt.Errorf("failed to create debug log file: %v", err)
    }
    cfg.debugFileHandle = debugFile
    cfg.DebugLogger = slog.New(slog.NewTextHandler(debugFile, &slog.HandlerOptions{
        Level: slog.LevelDebug,
    }))
}

// Add Close method
func (c *Config) Close() error {
    if c.debugFileHandle != nil {
        return c.debugFileHandle.Close()
    }
    return nil
}

// In main.go
defer cfg.Close()
```

**Priority:** HIGH - Fix soon

---

### 5. Inefficient URL Deduplication in Main Loop

**Location:** `cmd/probehttp/main.go:107-126`

**Issue:**
```go
newOriginalInputMap := make(map[string]string)
for _, urlStr := range expandedURLs {
    if originalInput, exists := originalInputMap[urlStr]; exists {
        newOriginalInputMap[urlStr] = originalInput
    } else {
        normalized := parser.NormalizeURL(urlStr)
        for origURL, origInput := range originalInputMap {
            if parser.NormalizeURL(origURL) == normalized {
                newOriginalInputMap[urlStr] = origInput
                break
            }
        }
    }
}
```

This is O(n²) complexity - for each deduplicated URL, we iterate through all original URLs.

**Impact:**
- Slow performance with large URL lists
- O(n²) time complexity

**Fix:**
```go
// Pre-compute normalized mappings
normalizedMap := make(map[string]string)
for origURL, origInput := range originalInputMap {
    normalized := parser.NormalizeURL(origURL)
    normalizedMap[normalized] = origInput
}

// O(n) lookup
newOriginalInputMap := make(map[string]string)
for _, urlStr := range expandedURLs {
    normalized := parser.NormalizeURL(urlStr)
    if origInput, exists := normalizedMap[normalized]; exists {
        newOriginalInputMap[urlStr] = origInput
    }
}
```

**Priority:** HIGH - Impacts performance at scale

---

### 6. Silent Error Swallowing in Body Reading

**Location:** Multiple locations including `internal/probe/prober.go:272`, `640`, `790`

**Issue:**
```go
initialBody, _ = io.ReadAll(finalResp.Body)
```

Errors from `io.ReadAll` are silently ignored.

**Impact:**
- Incomplete data processing
- Silent failures
- Incorrect results

**Fix:**
```go
initialBody, err := io.ReadAll(finalResp.Body)
if err != nil {
    p.config.Logger.Warn("error reading final body", "error", err)
    // Use partial body or set error flag
}
```

**Priority:** HIGH - Data integrity issue

---

### 7. No Timeout for Rate Limiter Wait

**Location:** `internal/probe/prober.go:350-359`

**Issue:**
```go
limiter := p.client.GetLimiter(hostname)
if err := limiter.Wait(ctx); err != nil {
    return output.ProbeResult{
        Timestamp: time.Now().Format(time.RFC3339),
        Input:     originalInput,
        Method:    "GET",
        Error:     fmt.Sprintf("Rate limit wait cancelled: %v", err),
    }
}
```

If context has no timeout, `Wait` could block indefinitely.

**Impact:**
- Indefinite hangs
- No progress on heavily rate-limited hosts

**Fix:**
```go
// Add rate limit timeout to config
RateLimitTimeout: 60 * time.Second, // Default 60s

// In prober
waitCtx, cancel := context.WithTimeout(ctx, p.config.RateLimitTimeout)
defer cancel()

if err := limiter.Wait(waitCtx); err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        return output.ProbeResult{
            Error: "rate limit wait timeout exceeded",
        }
    }
    return output.ProbeResult{
        Error: fmt.Sprintf("rate limit wait cancelled: %v", err),
    }
}
```

**Priority:** HIGH - Prevents hangs

---

### 8. Missing HTTP Client Cleanup

**Location:** `internal/probe/client.go` and `prober.go`

**Issue:**
Multiple HTTP clients are created for different TLS strategies but never cleaned up. Each client has a transport with connection pools.

**Impact:**
- Resource accumulation
- Memory leaks in long-running processes

**Fix:**
```go
// Track all created clients
type Prober struct {
    client      *Client
    config      *config.Config
    stderrMutex sync.Mutex
    clients     []*http.Client // Track all clients
    mu          sync.Mutex
}

// Add cleanup in probeURLWithConfig
defer func() {
    if transport, ok := httpClient.Transport.(*http.Transport); ok {
        transport.CloseIdleConnections()
    } else if transport, ok := httpClient.Transport.(*http3.Transport); ok {
        transport.Close()
    }
}()

// Or implement a client pool with cleanup
```

**Priority:** HIGH - Resource management

---

## Medium Priority Issues 🟢

### 9. Inefficient String Building in Debug Logging

**Location:** `internal/probe/prober.go:809-900` (debug methods)

**Issue:**
```go
func (p *Prober) debugRequest(req *http.Request, stepNum int, buf *strings.Builder) {
    var out strings.Builder
    fmt.Fprintf(&out, "[%d] REQUEST: %s %s\n", stepNum, req.Method, req.URL.String())
    // ... more writes
    if buf != nil {
        buf.WriteString(out.String())
    }
}
```

Creates intermediate `strings.Builder` then copies to another buffer.

**Impact:**
- Extra allocations
- Unnecessary copying
- Reduced debug mode performance

**Fix:**
```go
func (p *Prober) debugRequest(req *http.Request, stepNum int, buf *strings.Builder) {
    if !p.config.Debug || buf == nil {
        return
    }
    
    // Write directly to buf
    fmt.Fprintf(buf, "[%d] REQUEST: %s %s\n", stepNum, req.Method, req.URL.String())
    // ... continue writing to buf directly
}
```

**Priority:** MEDIUM - Performance optimization

---

### 10. Hard-coded TLS Version Constants

**Location:** `internal/probe/prober.go:700-714` (getTLSVersionString)

**Issue:**
```go
func getTLSVersionString(version uint16) string {
    switch version {
    case 0x0304:
        return "1.3"
    case 0x0303:
        return "1.2"
    // ...
    }
}
```

Uses magic numbers instead of tls package constants.

**Impact:**
- Code maintainability
- Potential bugs with TLS updates

**Fix:**
```go
import "crypto/tls"

func getTLSVersionString(version uint16) string {
    switch version {
    case tls.VersionTLS13:
        return "1.3"
    case tls.VersionTLS12:
        return "1.2"
    case tls.VersionTLS11:
        return "1.1"
    case tls.VersionTLS10:
        return "1.0"
    default:
        return fmt.Sprintf("unknown(0x%04x)", version)
    }
}
```

**Priority:** MEDIUM - Code quality

---

### 11. Incomplete Error Context in Validation

**Location:** `internal/parser/url.go:112-142` (ValidateURL)

**Issue:**
```go
func ValidateURL(input string, allowPrivateIPs bool) error {
    if len(input) > 2048 {
        return fmt.Errorf("URL too long (max 2048 chars)")
    }
    // Missing: actual length in error message
}
```

Error messages don't include relevant details for debugging.

**Impact:**
- Harder debugging
- Poor user experience

**Fix:**
```go
if len(input) > 2048 {
    return fmt.Errorf("URL too long: %d chars (max 2048)", len(input))
}

if ip.IsLoopback() || ip.IsPrivate() {
    return fmt.Errorf("private IP addresses not allowed: %s", parsed.Host)
}
```

**Priority:** MEDIUM - User experience

---

### 12. No Metrics or Observability

**Location:** Throughout codebase

**Issue:**
No built-in metrics for:
- Success/failure rates
- Response time percentiles
- TLS strategy success rates
- Rate limit hits
- Resource usage

**Impact:**
- Limited operational visibility
- Harder to debug production issues
- No performance insights

**Fix:**
```go
// Add metrics package
type Metrics struct {
    TotalRequests     atomic.Int64
    SuccessfulRequests atomic.Int64
    FailedRequests    atomic.Int64
    TLSStrategyStats  map[string]*atomic.Int64
    ResponseTimes     []time.Duration
    mu                sync.Mutex
}

// Export metrics
func (m *Metrics) Export() map[string]interface{} {
    // Return JSON-serializable metrics
}

// In config
type Config struct {
    // ...
    EnableMetrics bool
    Metrics       *Metrics
}
```

**Priority:** MEDIUM - Operational improvement

---

### 13. Missing Request Context Propagation

**Location:** `internal/probe/prober.go` (user-agent handling)

**Issue:**
User-Agent selection happens in `probeURLWithConfig` but isn't configurable per-request.

**Impact:**
- Limited flexibility
- Can't rotate User-Agents per request

**Fix:**
```go
// Add to ProbeOptions
type ProbeOptions struct {
    UserAgent      string
    CustomHeaders  map[string]string
    FollowRedirects bool
}

// Support per-request configuration
func (p *Prober) ProbeURLWithOptions(ctx context.Context, url string, opts ProbeOptions) ProbeResult
```

**Priority:** MEDIUM - Feature enhancement

---

## Low Priority Issues 🔵

### 14. Limited Error Types

**Location:** Throughout codebase

**Issue:**
All errors are strings. No typed errors for programmatic handling.

**Impact:**
- Can't distinguish error types
- Harder to handle specific errors
- Limited automation

**Fix:**
```go
// Define error types
var (
    ErrTimeout           = errors.New("request timeout")
    ErrTLSHandshake      = errors.New("TLS handshake failed")
    ErrPrivateIP         = errors.New("private IP not allowed")
    ErrMaxRedirects      = errors.New("max redirects exceeded")
    ErrRateLimited       = errors.New("rate limited")
)

// Use errors.Is() for checking
if errors.Is(err, ErrTimeout) {
    // Handle timeout specifically
}
```

**Priority:** LOW - API improvement

---

### 15. No Support for HTTP Methods Other Than GET

**Location:** Hard-coded throughout

**Issue:**
```go
req, err := http.NewRequestWithContext(ctx, "GET", probeURL, nil)
```

Only GET requests are supported.

**Impact:**
- Limited use cases
- Can't test POST/PUT endpoints

**Fix:**
```go
type Config struct {
    // ...
    HTTPMethod string // Default: "GET"
    RequestBody []byte // For POST/PUT
}

// In prober
req, err := http.NewRequestWithContext(ctx, p.config.HTTPMethod, probeURL, bytes.NewReader(p.config.RequestBody))
```

**Priority:** LOW - Feature enhancement

---

### 16. Hard-coded Rate Limit (10 req/s)

**Location:** `internal/probe/client.go:53-54`

**Issue:**
```go
// Allow 10 requests per second per host with burst of 1
limiter := rate.NewLimiter(10, 1)
```

**Impact:**
- Not configurable
- May be too aggressive or too lenient

**Fix:**
```go
type Config struct {
    // ...
    RateLimitPerSecond float64 // Default: 10
    RateLimitBurst     int     // Default: 1
}

// In GetLimiter
limiter := rate.NewLimiter(rate.Limit(c.config.RateLimitPerSecond), c.config.RateLimitBurst)
```

**Priority:** LOW - Configuration enhancement

---

### 17. Missing DNS Cache

**Location:** HTTP transport configuration

**Issue:**
No DNS caching. Each request performs DNS lookup.

**Impact:**
- Unnecessary DNS queries
- Slower performance for repeated hosts
- Increased load on DNS servers

**Fix:**
```go
// Use dnscache or custom DNS resolver
import "github.com/mercari/go-dnscache"

resolver := dnscache.New(time.Minute * 5)

transport := &http.Transport{
    // ...
    DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
        host, port, err := net.SplitHostPort(addr)
        if err != nil {
            return nil, err
        }
        
        ips, err := resolver.Fetch(host)
        if err != nil {
            return nil, err
        }
        
        dialer := &net.Dialer{}
        return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
    },
}
```

**Priority:** LOW - Performance optimization

---

### 18. No Request/Response Hooks

**Location:** Architecture

**Issue:**
No way to intercept requests/responses for custom processing.

**Impact:**
- Limited extensibility
- Can't add custom analysis
- Hard to integrate with other tools

**Fix:**
```go
type Hooks struct {
    PreRequest  func(*http.Request) error
    PostResponse func(*http.Response, *ProbeResult) error
}

type Config struct {
    // ...
    Hooks *Hooks
}

// In prober
if p.config.Hooks != nil && p.config.Hooks.PreRequest != nil {
    if err := p.config.Hooks.PreRequest(req); err != nil {
        return result
    }
}
```

**Priority:** LOW - Extensibility

---

## Suggestions for Improvement

### Architecture Improvements

1. **Implement Resource Cleanup Pattern**
   ```go
   type Closer interface {
       Close() error
   }
   
   // Prober, Config, Client should all implement Closer
   ```

2. **Add Context Value Propagation**
   - Request ID for tracing
   - Custom metadata
   - Request-specific configuration

3. **Implement Circuit Breaker Pattern**
   - Prevent hammering failing hosts
   - Automatic failure detection
   - Graceful degradation

4. **Add Result Streaming**
   - Stream results as they complete
   - Don't buffer all results in memory
   - Better for large-scale scans

### Performance Improvements

1. **Connection Pool Tuning**
   - Make pool sizes configurable
   - Implement per-host connection limits
   - Add connection pool metrics

2. **Body Reading Optimization**
   - Stream processing instead of full buffering
   - Configurable body size limits per content type
   - Early termination for large bodies

3. **Parallel Processing Optimization**
   - Dynamic worker pool sizing
   - Work stealing between workers
   - Priority queue for URLs

### Testing Improvements

1. **Add Integration Tests for Resource Cleanup**
   ```go
   func TestNoGoroutineLeaks(t *testing.T) {
       before := runtime.NumGoroutine()
       // Run probes
       after := runtime.NumGoroutine()
       assert.Equal(t, before, after)
   }
   ```

2. **Add Load Tests**
   - Test with 10k+ URLs
   - Memory profiling
   - CPU profiling

3. **Add Chaos Testing**
   - Random failures
   - Network delays
   - DNS failures

### Security Improvements

1. **Add Input Sanitization**
   - Stricter URL validation
   - Path traversal prevention
   - SSRF protection enhancements

2. **Add TLS Certificate Pinning Option**
   ```go
   type Config struct {
       // ...
       TLSPins map[string][]byte // hostname -> cert fingerprint
   }
   ```

3. **Add Request Signing**
   - HMAC-based request signing
   - API key authentication
   - OAuth support

### Documentation Improvements

1. **Add Godoc Comments**
   - All exported functions
   - All exported types
   - Package-level documentation

2. **Add Architecture Diagrams**
   - Data flow
   - Component interactions
   - Concurrency model

3. **Add Runbooks**
   - Common troubleshooting
   - Performance tuning guide
   - Security best practices

---

## Priority Action Items

### Immediate (This Week)
1. ✅ Fix HTTP/3 transport leak (#1)
2. ✅ Fix goroutine leak in parallel TLS (#2)
3. ✅ Fix response body close on error paths (#3)
4. ✅ Fix debug file logger race condition (#4)

### Short Term (This Month)
1. ✅ Optimize URL deduplication (#5)
2. ✅ Fix silent error swallowing (#6)
3. ✅ Add rate limiter timeout (#7)
4. ✅ Implement HTTP client cleanup (#8)

### Medium Term (This Quarter)
1. ✅ Add metrics and observability (#12)
2. ✅ Implement resource cleanup pattern
3. ✅ Add comprehensive integration tests
4. ✅ Improve error handling with typed errors

### Long Term (Next Quarter)
1. ✅ Add DNS caching (#17)
2. ✅ Implement hooks/middleware (#18)
3. ✅ Add circuit breaker pattern
4. ✅ Enhance extensibility

---

## Testing Recommendations

1. **Run Static Analysis**
   ```bash
   make lint
   make security
   go vet ./...
   staticcheck ./...
   ```

2. **Run Race Detector**
   ```bash
   go test -race ./...
   ```

3. **Profile Memory Usage**
   ```bash
   go test -memprofile mem.prof -bench .
   go tool pprof mem.prof
   ```

4. **Profile CPU Usage**
   ```bash
   go test -cpuprofile cpu.prof -bench .
   go tool pprof cpu.prof
   ```

5. **Check for Goroutine Leaks**
   ```bash
   go test -run TestConcurrent -count=1000
   # Monitor goroutine count
   ```

---

## Conclusion

The codebase has a solid foundation with good test coverage and modern Go practices. However, **critical resource management issues** need immediate attention, particularly around HTTP/3 transport cleanup and goroutine lifecycle management.

**Estimated Effort:**
- Critical fixes: 2-3 days
- High priority: 1 week
- Medium priority: 2 weeks
- Low priority: 1 month

**Risk Assessment:**
- Current state: **MEDIUM-HIGH** (resource leaks in production)
- After critical fixes: **LOW** (stable for production use)
- After all fixes: **VERY LOW** (production-ready with excellent reliability)

The tool is functional but needs these fixes before heavy production use or long-running deployments.
