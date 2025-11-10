# probeHTTP - Long-Term Features Implementation Plan

**Version:** 3.0 Roadmap
**Date:** 2025-11-10
**Status:** Planning Phase
**Estimated Timeline:** 6-12 months

---

## Executive Summary

This document outlines the implementation plan for 7 major features that will transform probeHTTP from a powerful HTTP probing tool into a comprehensive, enterprise-ready reconnaissance platform. These features are designed to be implemented incrementally, with each phase building upon the previous one.

**Target Architecture:** Modular, scalable, plugin-based system supporting distributed operations, persistent state, multi-format output, and visual verification.

---

## Table of Contents

1. [Feature Overview & Prioritization](#1-feature-overview--prioritization)
2. [Phase 1: Multiple Output Formats](#2-phase-1-multiple-output-formats)
3. [Phase 2: Response Caching](#3-phase-2-response-caching)
4. [Phase 3: Prometheus Metrics](#4-phase-3-prometheus-metrics)
5. [Phase 4: Plugin System](#5-phase-4-plugin-system)
6. [Phase 5: Resume Capability](#6-phase-5-resume-capability)
7. [Phase 6: Distributed Mode](#7-phase-6-distributed-mode)
8. [Phase 7: Screenshot Capture](#8-phase-7-screenshot-capture)
9. [Implementation Roadmap](#9-implementation-roadmap)
10. [Testing Strategy](#10-testing-strategy)
11. [Migration & Compatibility](#11-migration--compatibility)
12. [Risk Assessment](#12-risk-assessment)

---

## 1. Feature Overview & Prioritization

### 1.1 Priority Matrix

| Priority | Feature | Complexity | User Impact | Dependencies | Est. Time |
|----------|---------|------------|-------------|--------------|-----------|
| **P1** | Multiple Output Formats | Low | High | None | 2-3 weeks |
| **P2** | Response Caching | Low-Med | High | None | 2-3 weeks |
| **P3** | Prometheus Metrics | Low | Medium | None | 1-2 weeks |
| **P4** | Resume Capability | Medium | High | Caching | 3-4 weeks |
| **P5** | Plugin System | High | Medium | Output Formats | 4-6 weeks |
| **P6** | Screenshot Capture | Medium | Low | None (isolated) | 3-4 weeks |
| **P7** | Distributed Mode | Very High | Medium | All above | 8-12 weeks |

### 1.2 Implementation Phases

**Phase 1 (v3.0):** Output Formats + Caching + Metrics
**Phase 2 (v3.1):** Plugin System + Resume Capability
**Phase 3 (v3.2):** Screenshot Capture
**Phase 4 (v4.0):** Distributed Mode

### 1.3 Success Metrics

- **Performance:** No degradation for default use cases
- **Compatibility:** 100% backward compatible
- **Adoption:** 20%+ users adopt new features within 3 months
- **Stability:** <0.1% crash rate in production
- **Documentation:** 100% feature coverage in docs

---

## 2. Phase 1: Multiple Output Formats

**Priority:** P1 (High)
**Complexity:** Low
**Timeline:** 2-3 weeks
**Dependencies:** None

### 2.1 Objective

Support multiple output formats (JSON, CSV, XML, Markdown, custom templates) to integrate with various downstream tools and workflows.

### 2.2 Supported Formats

| Format | Use Case | Priority |
|--------|----------|----------|
| JSON | Default, API integration | ✅ Existing |
| CSV | Excel, data analysis | 🟢 High |
| XML | Enterprise tools | 🟡 Medium |
| Markdown | Documentation | 🟡 Medium |
| JSONL | Streaming, log aggregation | 🟢 High |
| Template | Custom formats | 🟢 High |

### 2.3 Architecture

```
internal/output/
├── formatter.go         # Formatter interface
├── json.go             # JSON formatter (existing)
├── jsonl.go            # JSON Lines formatter
├── csv.go              # CSV formatter
├── xml.go              # XML formatter
├── markdown.go         # Markdown formatter
└── template.go         # Go template-based formatter
```

#### Interface Design

```go
package output

import (
    "io"
)

// Formatter defines the interface for output formats
type Formatter interface {
    // Format formats a single result
    Format(result ProbeResult) ([]byte, error)

    // FormatBatch formats multiple results efficiently
    FormatBatch(results []ProbeResult) ([]byte, error)

    // WriteHeader writes format-specific headers (e.g., CSV headers)
    WriteHeader(w io.Writer) error

    // WriteFooter writes format-specific footers (e.g., XML closing tags)
    WriteFooter(w io.Writer) error

    // ContentType returns the MIME type
    ContentType() string

    // FileExtension returns the recommended file extension
    FileExtension() string
}

// FormatterRegistry manages available formatters
type FormatterRegistry struct {
    formatters map[string]Formatter
    mu         sync.RWMutex
}

// Register registers a new formatter
func (r *FormatterRegistry) Register(name string, formatter Formatter) error

// Get retrieves a formatter by name
func (r *FormatterRegistry) Get(name string) (Formatter, error)

// List returns all registered formatter names
func (r *FormatterRegistry) List() []string
```

### 2.4 Implementation Details

#### CSV Formatter

```go
package output

import (
    "encoding/csv"
    "fmt"
    "io"
    "strconv"
    "strings"
)

type CSVFormatter struct {
    IncludeHeaders bool
    Delimiter      rune
}

func (f *CSVFormatter) Format(result ProbeResult) ([]byte, error) {
    var buf strings.Builder
    w := csv.NewWriter(&buf)
    w.Comma = f.Delimiter

    record := []string{
        result.Timestamp,
        result.URL,
        result.FinalURL,
        strconv.Itoa(result.StatusCode),
        result.Title,
        result.WebServer,
        result.ContentType,
        strconv.Itoa(result.ContentLength),
        result.Hash.BodyMMH3,
        result.Hash.HeaderMMH3,
        strconv.Itoa(result.Words),
        strconv.Itoa(result.Lines),
        result.Time,
        result.Scheme,
        result.Host,
        result.Port,
        result.Path,
        fmt.Sprintf("%v", result.ChainStatusCodes),
        strings.Join(result.ChainHosts, ";"),
        result.Error,
    }

    if err := w.Write(record); err != nil {
        return nil, err
    }
    w.Flush()

    return []byte(buf.String()), w.Error()
}

func (f *CSVFormatter) WriteHeader(w io.Writer) error {
    if !f.IncludeHeaders {
        return nil
    }

    headers := []string{
        "timestamp", "url", "final_url", "status_code", "title",
        "webserver", "content_type", "content_length", "body_hash",
        "header_hash", "words", "lines", "response_time", "scheme",
        "host", "port", "path", "chain_status_codes", "chain_hosts", "error",
    }

    csvWriter := csv.NewWriter(w)
    csvWriter.Comma = f.Delimiter
    err := csvWriter.Write(headers)
    csvWriter.Flush()
    return err
}

func (f *CSVFormatter) ContentType() string {
    return "text/csv"
}

func (f *CSVFormatter) FileExtension() string {
    return ".csv"
}
```

#### Template Formatter

```go
package output

import (
    "bytes"
    "io"
    "text/template"
)

type TemplateFormatter struct {
    Template *template.Template
}

func NewTemplateFormatter(tmplString string) (*TemplateFormatter, error) {
    tmpl, err := template.New("output").Parse(tmplString)
    if err != nil {
        return nil, fmt.Errorf("failed to parse template: %w", err)
    }

    return &TemplateFormatter{Template: tmpl}, nil
}

func (f *TemplateFormatter) Format(result ProbeResult) ([]byte, error) {
    var buf bytes.Buffer
    if err := f.Template.Execute(&buf, result); err != nil {
        return nil, err
    }
    return buf.Bytes(), nil
}

// Example templates:
// Simple: "{{.URL}} -> {{.StatusCode}} ({{.Title}})\n"
// Detailed: "URL: {{.URL}}\nStatus: {{.StatusCode}}\nTitle: {{.Title}}\nServer: {{.WebServer}}\n\n"
// Custom JSON: '{"url":"{{.URL}}","status":{{.StatusCode}},"hash":"{{.Hash.BodyMMH3}}"}\n'
```

### 2.5 Configuration

Add to `internal/config/config.go`:

```go
type Config struct {
    // ... existing fields ...

    OutputFormat     string // json, csv, xml, markdown, jsonl, template
    OutputTemplate   string // For template format
    CSVDelimiter     rune   // CSV delimiter (default: comma)
    CSVIncludeHeader bool   // Include CSV headers
}
```

### 2.6 CLI Flags

```go
flag.StringVar(&cfg.OutputFormat, "format", "json",
    "Output format: json, jsonl, csv, xml, markdown, template")
flag.StringVar(&cfg.OutputFormat, "f", "json",
    "Output format (shorthand)")
flag.StringVar(&cfg.OutputTemplate, "template", "",
    "Custom Go template for output formatting")
flag.StringVar(&cfg.OutputTemplate, "T", "",
    "Custom Go template (shorthand)")
flag.BoolVar(&cfg.CSVIncludeHeader, "csv-header", true,
    "Include header row in CSV output")
```

### 2.7 Usage Examples

```bash
# CSV output
./probeHTTP -i urls.txt -format csv -o results.csv

# CSV with custom delimiter (TSV)
./probeHTTP -i urls.txt -format csv -o results.tsv --csv-delimiter=$'\t'

# XML output
./probeHTTP -i urls.txt -format xml -o results.xml

# JSON Lines (streaming)
./probeHTTP -i urls.txt -format jsonl | grep '"status_code":200'

# Custom template
./probeHTTP -i urls.txt -format template \
    -template '{{.URL}} | {{.StatusCode}} | {{.Title}}\n'

# Markdown table
./probeHTTP -i urls.txt -format markdown -o report.md
```

### 2.8 Testing Strategy

```go
// Test all formatters
func TestFormatters(t *testing.T) {
    result := createTestResult()

    tests := []struct {
        name      string
        formatter Formatter
        want      string
    }{
        {"JSON", &JSONFormatter{}, `{"url":"http://example.com"...}`},
        {"CSV", &CSVFormatter{}, "2025-11-10,...\n"},
        {"XML", &XMLFormatter{}, "<result><url>..."},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := tt.formatter.Format(result)
            assert.NoError(t, err)
            assert.Contains(t, string(got), tt.want)
        })
    }
}
```

### 2.9 Performance Considerations

- Use buffered writers for large outputs
- Batch formatting when possible
- Stream output for JSONL format
- Memory-efficient CSV writing

### 2.10 Migration Path

- JSON remains default (100% backward compatible)
- Existing scripts continue to work
- New formats opt-in only
- Auto-detect format from file extension

---

## 3. Phase 2: Response Caching

**Priority:** P2 (High)
**Complexity:** Low-Medium
**Timeline:** 2-3 weeks
**Dependencies:** None

### 3.1 Objective

Cache HTTP responses to avoid redundant probing of the same URLs, improving performance for repeated scans and reducing load on target servers.

### 3.2 Architecture

```
internal/cache/
├── cache.go            # Cache interface
├── memory.go           # In-memory LRU cache
├── disk.go             # Disk-based persistent cache
├── redis.go            # Redis cache (optional)
└── key.go              # Cache key generation
```

#### Cache Interface

```go
package cache

import (
    "context"
    "time"

    "probeHTTP/internal/output"
)

// Cache defines the caching interface
type Cache interface {
    // Get retrieves a cached result
    Get(ctx context.Context, key string) (*output.ProbeResult, error)

    // Set stores a result in cache
    Set(ctx context.Context, key string, result *output.ProbeResult, ttl time.Duration) error

    // Delete removes a result from cache
    Delete(ctx context.Context, key string) error

    // Clear removes all cached results
    Clear(ctx context.Context) error

    // Stats returns cache statistics
    Stats() CacheStats

    // Close closes the cache (cleanup)
    Close() error
}

// CacheStats provides cache statistics
type CacheStats struct {
    Hits       int64
    Misses     int64
    Size       int64
    MaxSize    int64
    Evictions  int64
    HitRate    float64
}

// KeyGenerator generates cache keys
type KeyGenerator interface {
    // Generate creates a cache key from URL and options
    Generate(url string, opts KeyOptions) string
}

// KeyOptions controls what factors into the cache key
type KeyOptions struct {
    IncludeUserAgent bool
    IncludeHeaders   bool
    IncludeMethod    bool
    IncludeCookies   bool
}
```

### 3.3 Implementation Details

#### Memory Cache (LRU)

```go
package cache

import (
    "container/list"
    "context"
    "sync"
    "time"

    "probeHTTP/internal/output"
)

type MemoryCache struct {
    maxSize  int
    ttl      time.Duration
    items    map[string]*cacheEntry
    lru      *list.List
    mu       sync.RWMutex
    stats    CacheStats
}

type cacheEntry struct {
    key        string
    result     *output.ProbeResult
    expiry     time.Time
    lruElement *list.Element
}

func NewMemoryCache(maxSize int, ttl time.Duration) *MemoryCache {
    c := &MemoryCache{
        maxSize: maxSize,
        ttl:     ttl,
        items:   make(map[string]*cacheEntry, maxSize),
        lru:     list.New(),
    }

    // Start background cleanup goroutine
    go c.cleanup()

    return c
}

func (c *MemoryCache) Get(ctx context.Context, key string) (*output.ProbeResult, error) {
    c.mu.RLock()
    entry, exists := c.items[key]
    c.mu.RUnlock()

    if !exists {
        c.mu.Lock()
        c.stats.Misses++
        c.mu.Unlock()
        return nil, ErrCacheMiss
    }

    // Check expiry
    if time.Now().After(entry.expiry) {
        c.Delete(ctx, key)
        c.mu.Lock()
        c.stats.Misses++
        c.mu.Unlock()
        return nil, ErrCacheMiss
    }

    // Move to front of LRU
    c.mu.Lock()
    c.lru.MoveToFront(entry.lruElement)
    c.stats.Hits++
    c.stats.HitRate = float64(c.stats.Hits) / float64(c.stats.Hits+c.stats.Misses)
    c.mu.Unlock()

    return entry.result, nil
}

func (c *MemoryCache) Set(ctx context.Context, key string, result *output.ProbeResult, ttl time.Duration) error {
    c.mu.Lock()
    defer c.mu.Unlock()

    // Check if key exists
    if entry, exists := c.items[key]; exists {
        // Update existing entry
        entry.result = result
        entry.expiry = time.Now().Add(ttl)
        c.lru.MoveToFront(entry.lruElement)
        return nil
    }

    // Evict if at capacity
    if c.lru.Len() >= c.maxSize {
        c.evictOldest()
    }

    // Add new entry
    element := c.lru.PushFront(key)
    c.items[key] = &cacheEntry{
        key:        key,
        result:     result,
        expiry:     time.Now().Add(ttl),
        lruElement: element,
    }
    c.stats.Size++

    return nil
}

func (c *MemoryCache) evictOldest() {
    if element := c.lru.Back(); element != nil {
        c.lru.Remove(element)
        key := element.Value.(string)
        delete(c.items, key)
        c.stats.Evictions++
        c.stats.Size--
    }
}

func (c *MemoryCache) cleanup() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        c.mu.Lock()
        now := time.Now()
        for key, entry := range c.items {
            if now.After(entry.expiry) {
                c.lru.Remove(entry.lruElement)
                delete(c.items, key)
                c.stats.Size--
            }
        }
        c.mu.Unlock()
    }
}
```

#### Disk Cache

```go
package cache

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "os"
    "path/filepath"
    "time"
)

type DiskCache struct {
    basePath string
    ttl      time.Duration
    stats    CacheStats
    mu       sync.RWMutex
}

func NewDiskCache(basePath string, ttl time.Duration) (*DiskCache, error) {
    if err := os.MkdirAll(basePath, 0755); err != nil {
        return nil, fmt.Errorf("failed to create cache directory: %w", err)
    }

    return &DiskCache{
        basePath: basePath,
        ttl:      ttl,
    }, nil
}

func (c *DiskCache) Get(ctx context.Context, key string) (*output.ProbeResult, error) {
    path := c.keyToPath(key)

    data, err := ioutil.ReadFile(path)
    if err != nil {
        if os.IsNotExist(err) {
            c.mu.Lock()
            c.stats.Misses++
            c.mu.Unlock()
            return nil, ErrCacheMiss
        }
        return nil, err
    }

    var entry cachedResult
    if err := json.Unmarshal(data, &entry); err != nil {
        return nil, err
    }

    // Check expiry
    if time.Now().After(entry.Expiry) {
        c.Delete(ctx, key)
        c.mu.Lock()
        c.stats.Misses++
        c.mu.Unlock()
        return nil, ErrCacheMiss
    }

    c.mu.Lock()
    c.stats.Hits++
    c.stats.HitRate = float64(c.stats.Hits) / float64(c.stats.Hits+c.stats.Misses)
    c.mu.Unlock()

    return &entry.Result, nil
}

func (c *DiskCache) Set(ctx context.Context, key string, result *output.ProbeResult, ttl time.Duration) error {
    entry := cachedResult{
        Result: *result,
        Expiry: time.Now().Add(ttl),
    }

    data, err := json.Marshal(entry)
    if err != nil {
        return err
    }

    path := c.keyToPath(key)
    dir := filepath.Dir(path)

    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }

    return ioutil.WriteFile(path, data, 0644)
}

func (c *DiskCache) keyToPath(key string) string {
    hash := sha256.Sum256([]byte(key))
    hashStr := hex.EncodeToString(hash[:])

    // Split into directories for better filesystem performance
    // e.g., ab/cd/ef/abcdef...
    return filepath.Join(
        c.basePath,
        hashStr[0:2],
        hashStr[2:4],
        hashStr[4:6],
        hashStr+".json",
    )
}

type cachedResult struct {
    Result output.ProbeResult
    Expiry time.Time
}
```

#### Cache Key Generation

```go
package cache

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "sort"
    "strings"
)

type DefaultKeyGenerator struct{}

func (g *DefaultKeyGenerator) Generate(url string, opts KeyOptions) string {
    components := []string{url}

    if opts.IncludeMethod {
        components = append(components, "GET") // or actual method
    }

    if opts.IncludeUserAgent {
        components = append(components, "ua:"+getUserAgent())
    }

    if opts.IncludeHeaders {
        // Add sorted header keys
        headers := getSortedHeaders()
        components = append(components, "headers:"+strings.Join(headers, ","))
    }

    // Generate deterministic key
    key := strings.Join(components, "|")
    hash := sha256.Sum256([]byte(key))
    return hex.EncodeToString(hash[:])
}
```

### 3.4 Configuration

```go
type Config struct {
    // ... existing fields ...

    CacheEnabled    bool
    CacheType       string        // memory, disk, redis
    CacheTTL        time.Duration // Default TTL
    CacheMaxSize    int           // Max entries (memory cache)
    CachePath       string        // Path for disk cache
    CacheRedisAddr  string        // Redis address
}
```

### 3.5 CLI Flags

```bash
--cache                    # Enable caching
--cache-type memory        # Cache type: memory, disk, redis
--cache-ttl 1h            # Cache TTL
--cache-max-size 10000    # Max cache entries
--cache-path ~/.probehttp/cache  # Disk cache path
--cache-clear             # Clear cache before run
```

### 3.6 Usage Examples

```bash
# Enable memory cache
./probeHTTP -i urls.txt --cache --cache-ttl 1h

# Use disk cache (persistent across runs)
./probeHTTP -i urls.txt --cache --cache-type disk --cache-path ./cache

# Clear cache and run
./probeHTTP -i urls.txt --cache --cache-clear

# Show cache stats
./probeHTTP -i urls.txt --cache --cache-stats
```

### 3.7 Integration with Prober

```go
func (p *Prober) ProbeURL(ctx context.Context, probeURL string, originalInput string) output.ProbeResult {
    // Try cache first
    if p.cache != nil {
        cacheKey := p.keyGen.Generate(probeURL, p.cacheOpts)
        if cached, err := p.cache.Get(ctx, cacheKey); err == nil {
            p.config.Logger.Debug("cache hit", "url", probeURL)
            return *cached
        }
    }

    // Cache miss - probe URL
    result := p.probeURLOnce(ctx, probeURL, originalInput)

    // Store in cache on success
    if p.cache != nil && result.Error == "" {
        cacheKey := p.keyGen.Generate(probeURL, p.cacheOpts)
        if err := p.cache.Set(ctx, cacheKey, &result, p.config.CacheTTL); err != nil {
            p.config.Logger.Warn("failed to cache result", "url", probeURL, "error", err)
        }
    }

    return result
}
```

### 3.8 Performance Impact

- **Cache Hit:** ~0.1ms (memory) vs ~50-500ms (HTTP request)
- **Cache Miss:** Negligible overhead (~0.5ms)
- **Memory Usage:** ~2KB per cached result
- **Disk Usage:** ~2KB per cached result

---

## 4. Phase 3: Prometheus Metrics

**Priority:** P3 (Medium)
**Complexity:** Low
**Timeline:** 1-2 weeks
**Dependencies:** None

### 4.1 Objective

Export detailed metrics to Prometheus for monitoring, alerting, and performance analysis in production environments.

### 4.2 Architecture

```
internal/metrics/
├── metrics.go          # Metrics definitions
├── collector.go        # Custom collectors
└── server.go           # HTTP metrics endpoint
```

### 4.3 Metrics Design

```go
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Request metrics
    RequestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "probehttp_requests_total",
            Help: "Total number of HTTP probe requests",
        },
        []string{"status_code", "scheme", "method"},
    )

    RequestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "probehttp_request_duration_seconds",
            Help: "HTTP request duration in seconds",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to 16s
        },
        []string{"status_code", "scheme"},
    )

    RequestSize = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "probehttp_request_size_bytes",
            Help: "HTTP request size in bytes",
            Buckets: prometheus.ExponentialBuckets(100, 10, 8), // 100B to 100MB
        },
        []string{"scheme"},
    )

    ResponseSize = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "probehttp_response_size_bytes",
            Help: "HTTP response size in bytes",
            Buckets: prometheus.ExponentialBuckets(100, 10, 8),
        },
        []string{"status_code", "scheme"},
    )

    // Error metrics
    ErrorsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "probehttp_errors_total",
            Help: "Total number of probe errors",
        },
        []string{"error_type"},
    )

    // Redirect metrics
    RedirectsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "probehttp_redirects_total",
            Help: "Total number of HTTP redirects followed",
        },
        []string{"from_status", "cross_host"},
    )

    RedirectChainLength = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name: "probehttp_redirect_chain_length",
            Help: "Length of redirect chains",
            Buckets: []float64{0, 1, 2, 3, 5, 10, 20},
        },
    )

    // Cache metrics
    CacheHitsTotal = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "probehttp_cache_hits_total",
            Help: "Total number of cache hits",
        },
    )

    CacheMissesTotal = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "probehttp_cache_misses_total",
            Help: "Total number of cache misses",
        },
    )

    CacheSize = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "probehttp_cache_size_entries",
            Help: "Current number of entries in cache",
        },
    )

    CacheEvictionsTotal = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "probehttp_cache_evictions_total",
            Help: "Total number of cache evictions",
        },
    )

    // Worker metrics
    ActiveWorkers = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "probehttp_active_workers",
            Help: "Number of currently active worker goroutines",
        },
    )

    QueuedURLs = promauto.NewGauge(
        prometheus.GaugeOpts{
            Name: "probehttp_queued_urls",
            Help: "Number of URLs waiting in queue",
        },
    )

    ProcessedURLsTotal = promauto.NewCounter(
        prometheus.CounterOpts{
            Name: "probehttp_processed_urls_total",
            Help: "Total number of URLs processed",
        },
    )

    // Rate limiting metrics
    RateLimitWaitsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "probehttp_rate_limit_waits_total",
            Help: "Total number of rate limit waits",
        },
        []string{"host"},
    )

    RateLimitWaitDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "probehttp_rate_limit_wait_duration_seconds",
            Help: "Duration of rate limit waits",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
        },
        []string{"host"},
    )

    // Title extraction metrics
    TitlesExtractedTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "probehttp_titles_extracted_total",
            Help: "Total number of titles extracted",
        },
        []string{"source"}, // title, og:title, twitter:title
    )

    // TLS metrics
    TLSVersions = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "probehttp_tls_versions_total",
            Help: "TLS versions encountered",
        },
        []string{"version"},
    )
)
```

### 4.4 Integration Points

```go
// In prober.go
func (p *Prober) probeURLOnce(ctx context.Context, probeURL string, originalInput string) output.ProbeResult {
    startTime := time.Now()

    // ... existing code ...

    // Record metrics
    duration := time.Since(startTime).Seconds()
    metrics.RequestsTotal.WithLabelValues(
        strconv.Itoa(result.StatusCode),
        result.Scheme,
        "GET",
    ).Inc()

    metrics.RequestDuration.WithLabelValues(
        strconv.Itoa(result.StatusCode),
        result.Scheme,
    ).Observe(duration)

    if result.Error != "" {
        metrics.ErrorsTotal.WithLabelValues(getErrorType(result.Error)).Inc()
    }

    if len(result.ChainStatusCodes) > 1 {
        metrics.RedirectChainLength.Observe(float64(len(result.ChainStatusCodes) - 1))
    }

    metrics.ResponseSize.WithLabelValues(
        strconv.Itoa(result.StatusCode),
        result.Scheme,
    ).Observe(float64(result.ContentLength))

    metrics.ProcessedURLsTotal.Inc()

    return result
}
```

### 4.5 Metrics Server

```go
package metrics

import (
    "net/http"

    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// Server runs the metrics HTTP server
func Server(addr string) error {
    http.Handle("/metrics", promhttp.Handler())
    return http.ListenAndServe(addr, nil)
}
```

### 4.6 Configuration

```go
type Config struct {
    // ... existing fields ...

    MetricsEnabled bool
    MetricsAddr    string // e.g., ":9090"
}
```

### 4.7 CLI Flags

```bash
--metrics               # Enable Prometheus metrics
--metrics-addr :9090   # Metrics endpoint address
```

### 4.8 Usage

```bash
# Enable metrics endpoint
./probeHTTP -i urls.txt --metrics --metrics-addr :9090

# In another terminal, scrape metrics
curl http://localhost:9090/metrics

# Or configure Prometheus scrape config
# prometheus.yml:
scrape_configs:
  - job_name: 'probehttp'
    static_configs:
      - targets: ['localhost:9090']
```

### 4.9 Example Queries

```promql
# Request rate
rate(probehttp_requests_total[5m])

# Error rate
rate(probehttp_errors_total[5m]) / rate(probehttp_requests_total[5m])

# P95 latency
histogram_quantile(0.95, rate(probehttp_request_duration_seconds_bucket[5m]))

# Cache hit rate
rate(probehttp_cache_hits_total[5m]) /
  (rate(probehttp_cache_hits_total[5m]) + rate(probehttp_cache_misses_total[5m]))

# Status code distribution
sum by (status_code) (rate(probehttp_requests_total[5m]))

# Slow requests (>2s)
count(rate(probehttp_request_duration_seconds_bucket{le="2"}[5m]) == 0)
```

### 4.10 Grafana Dashboard

Create a dashboard with:
- Request rate and latency
- Error rates by type
- Status code distribution
- Cache performance
- Worker utilization
- Queue depth
- Rate limit metrics

---

## 5. Phase 4: Plugin System

**Priority:** P5 (Medium)
**Complexity:** High
**Timeline:** 4-6 weeks
**Dependencies:** Output formats

### 5.1 Objective

Enable users to extend probeHTTP with custom extractors, processors, and output handlers without modifying core code.

### 5.2 Architecture

```
internal/plugin/
├── plugin.go           # Plugin interface and manager
├── loader.go           # Plugin loading (Go plugins or WASM)
├── hooks.go            # Lifecycle hooks
├── registry.go         # Plugin registry
└── examples/
    ├── extractor/      # Example extractor plugin
    ├── processor/      # Example processor plugin
    └── output/         # Example output plugin
```

### 5.3 Plugin Types

| Type | Purpose | Hook Points |
|------|---------|-------------|
| **Extractor** | Extract custom metadata from responses | After response received |
| **Processor** | Transform/enrich results | After extraction, before output |
| **Output** | Custom output formats | During result writing |
| **Validator** | Custom input validation | Before probing |
| **Middleware** | Request/response modification | Before/after HTTP call |

### 5.4 Plugin Interface

```go
package plugin

import (
    "context"
    "net/http"

    "probeHTTP/internal/output"
)

// Plugin is the base interface all plugins must implement
type Plugin interface {
    // Name returns the plugin name
    Name() string

    // Version returns the plugin version
    Version() string

    // Init initializes the plugin with configuration
    Init(config map[string]interface{}) error

    // Close cleans up plugin resources
    Close() error
}

// Extractor plugins extract custom metadata from responses
type Extractor interface {
    Plugin

    // Extract extracts metadata from response
    Extract(ctx context.Context, resp *http.Response, body []byte) (map[string]interface{}, error)

    // Fields returns the list of fields this extractor adds
    Fields() []string
}

// Processor plugins transform or enrich probe results
type Processor interface {
    Plugin

    // Process transforms the result
    Process(ctx context.Context, result *output.ProbeResult) (*output.ProbeResult, error)
}

// Validator plugins provide custom URL validation
type Validator interface {
    Plugin

    // Validate validates a URL
    Validate(ctx context.Context, url string) error
}

// Middleware plugins can modify requests and responses
type Middleware interface {
    Plugin

    // BeforeRequest modifies the request before sending
    BeforeRequest(ctx context.Context, req *http.Request) (*http.Request, error)

    // AfterResponse processes the response after receiving
    AfterResponse(ctx context.Context, resp *http.Response) (*http.Response, error)
}

// OutputHandler plugins provide custom output formats
type OutputHandler interface {
    Plugin

    // Handle writes the result in custom format
    Handle(ctx context.Context, result *output.ProbeResult) error
}
```

### 5.5 Plugin Manager

```go
package plugin

import (
    "context"
    "fmt"
    "sync"
)

type Manager struct {
    extractors  []Extractor
    processors  []Processor
    validators  []Validator
    middlewares []Middleware
    outputs     []OutputHandler
    mu          sync.RWMutex
}

func NewManager() *Manager {
    return &Manager{
        extractors:  make([]Extractor, 0),
        processors:  make([]Processor, 0),
        validators:  make([]Validator, 0),
        middlewares: make([]Middleware, 0),
        outputs:     make([]OutputHandler, 0),
    }
}

// RegisterExtractor registers an extractor plugin
func (m *Manager) RegisterExtractor(e Extractor) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.extractors = append(m.extractors, e)
    return nil
}

// RunExtractors runs all registered extractors
func (m *Manager) RunExtractors(ctx context.Context, resp *http.Response, body []byte) (map[string]interface{}, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    combined := make(map[string]interface{})

    for _, extractor := range m.extractors {
        data, err := extractor.Extract(ctx, resp, body)
        if err != nil {
            return nil, fmt.Errorf("extractor %s failed: %w", extractor.Name(), err)
        }

        // Merge results
        for k, v := range data {
            combined[k] = v
        }
    }

    return combined, nil
}

// Similar methods for processors, validators, etc.
```

### 5.6 Example Plugins

#### Security Headers Extractor

```go
package main

import (
    "context"
    "net/http"

    "probeHTTP/internal/plugin"
)

type SecurityHeadersExtractor struct {
    config map[string]interface{}
}

func (e *SecurityHeadersExtractor) Name() string {
    return "security-headers"
}

func (e *SecurityHeadersExtractor) Version() string {
    return "1.0.0"
}

func (e *SecurityHeadersExtractor) Init(config map[string]interface{}) error {
    e.config = config
    return nil
}

func (e *SecurityHeadersExtractor) Close() error {
    return nil
}

func (e *SecurityHeadersExtractor) Extract(ctx context.Context, resp *http.Response, body []byte) (map[string]interface{}, error) {
    headers := map[string]interface{}{
        "has_hsts":            resp.Header.Get("Strict-Transport-Security") != "",
        "has_csp":             resp.Header.Get("Content-Security-Policy") != "",
        "has_x_frame":         resp.Header.Get("X-Frame-Options") != "",
        "has_x_content_type":  resp.Header.Get("X-Content-Type-Options") != "",
        "has_referrer_policy": resp.Header.Get("Referrer-Policy") != "",
        "hsts_value":          resp.Header.Get("Strict-Transport-Security"),
        "csp_value":           resp.Header.Get("Content-Security-Policy"),
    }

    return map[string]interface{}{
        "security_headers": headers,
    }, nil
}

func (e *SecurityHeadersExtractor) Fields() []string {
    return []string{"security_headers"}
}

// Export plugin
var Plugin plugin.Extractor = &SecurityHeadersExtractor{}
```

#### Technology Detection Extractor

```go
type TechnologyExtractor struct {
    patterns map[string]*regexp.Regexp
}

func (e *TechnologyExtractor) Extract(ctx context.Context, resp *http.Response, body []byte) (map[string]interface{}, error) {
    detected := []string{}

    bodyStr := string(body)

    // Check headers
    server := resp.Header.Get("Server")
    xPoweredBy := resp.Header.Get("X-Powered-By")

    if server != "" {
        detected = append(detected, "server:"+server)
    }
    if xPoweredBy != "" {
        detected = append(detected, "powered-by:"+xPoweredBy)
    }

    // Check body for patterns
    techs := map[string]*regexp.Regexp{
        "WordPress":  regexp.MustCompile(`wp-content|wp-includes`),
        "jQuery":     regexp.MustCompile(`jquery\.js|jquery\.min\.js`),
        "React":      regexp.MustCompile(`react\.js|react\.min\.js|__REACT`),
        "Vue":        regexp.MustCompile(`vue\.js|vue\.min\.js|__VUE__`),
        "Angular":    regexp.MustCompile(`angular\.js|ng-app|ng-controller`),
        "Bootstrap":  regexp.MustCompile(`bootstrap\.css|bootstrap\.js`),
        "Django":     regexp.MustCompile(`csrfmiddlewaretoken`),
        "Laravel":    regexp.MustCompile(`laravel_session`),
    }

    for tech, pattern := range techs {
        if pattern.Match([]byte(bodyStr)) {
            detected = append(detected, tech)
        }
    }

    return map[string]interface{}{
        "technologies": detected,
    }, nil
}
```

#### GeoIP Enrichment Processor

```go
type GeoIPProcessor struct {
    db *geoip2.Reader
}

func (p *GeoIPProcessor) Process(ctx context.Context, result *output.ProbeResult) (*output.ProbeResult, error) {
    // Resolve IP from hostname
    ips, err := net.LookupIP(result.Host)
    if err != nil {
        return result, nil // Non-fatal
    }

    if len(ips) == 0 {
        return result, nil
    }

    // Lookup GeoIP
    record, err := p.db.City(ips[0])
    if err != nil {
        return result, nil // Non-fatal
    }

    // Add geo data to result
    result.CustomFields = map[string]interface{}{
        "geo_country":   record.Country.Names["en"],
        "geo_city":      record.City.Names["en"],
        "geo_latitude":  record.Location.Latitude,
        "geo_longitude": record.Location.Longitude,
        "geo_timezone":  record.Location.TimeZone,
    }

    return result, nil
}
```

### 5.7 Plugin Loading

#### Option 1: Go Plugins (Shared Libraries)

```go
package plugin

import (
    "fmt"
    "plugin"
)

func LoadPlugin(path string) (Plugin, error) {
    p, err := plugin.Open(path)
    if err != nil {
        return nil, fmt.Errorf("failed to open plugin: %w", err)
    }

    symPlugin, err := p.Lookup("Plugin")
    if err != nil {
        return nil, fmt.Errorf("plugin does not export 'Plugin': %w", err)
    }

    plug, ok := symPlugin.(Plugin)
    if !ok {
        return nil, fmt.Errorf("symbol 'Plugin' does not implement Plugin interface")
    }

    return plug, nil
}
```

#### Option 2: WASM Plugins (More portable)

```go
// Using wazero or similar WASM runtime
import "github.com/tetratelabs/wazero"

func LoadWASMPlugin(ctx context.Context, path string) (Plugin, error) {
    runtime := wazero.NewRuntime(ctx)
    defer runtime.Close(ctx)

    wasm, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    module, err := runtime.InstantiateModuleFromBinary(ctx, wasm)
    if err != nil {
        return nil, err
    }

    // Create plugin wrapper that calls WASM functions
    return &WASMPlugin{
        module:  module,
        runtime: runtime,
    }, nil
}
```

### 5.8 Plugin Configuration

```yaml
# plugins.yaml
plugins:
  - name: security-headers
    type: extractor
    path: ./plugins/security-headers.so
    enabled: true
    config:
      strict_mode: true

  - name: technology-detection
    type: extractor
    path: ./plugins/tech-detect.so
    enabled: true

  - name: geoip-enrichment
    type: processor
    path: ./plugins/geoip.so
    enabled: true
    config:
      database: /path/to/GeoLite2-City.mmdb

  - name: elasticsearch-output
    type: output
    path: ./plugins/elasticsearch.so
    enabled: true
    config:
      hosts:
        - http://localhost:9200
      index: probehttp-results
```

### 5.9 CLI Integration

```bash
# Load plugins from directory
./probeHTTP -i urls.txt --plugins-dir ./plugins

# Load specific plugin
./probeHTTP -i urls.txt --plugin ./plugins/security-headers.so

# Plugin configuration
./probeHTTP -i urls.txt --plugins-config plugins.yaml

# List available plugins
./probeHTTP --list-plugins
```

### 5.10 Enhanced Result Structure

```go
type ProbeResult struct {
    // ... existing fields ...

    // CustomFields holds plugin-extracted data
    CustomFields map[string]interface{} `json:"custom_fields,omitempty"`
}
```

---

## 6. Phase 5: Resume Capability

**Priority:** P4 (High)
**Complexity:** Medium
**Timeline:** 3-4 weeks
**Dependencies:** Caching

### 6.1 Objective

Enable resuming interrupted scans from the last checkpoint, preserving progress and avoiding redundant work.

### 6.2 Architecture

```
internal/resume/
├── state.go            # State management
├── checkpoint.go       # Checkpointing logic
├── recovery.go         # Recovery logic
└── storage.go          # State persistence
```

### 6.3 State Structure

```go
package resume

import (
    "time"

    "probeHTTP/internal/output"
)

// ScanState represents the complete state of a scan
type ScanState struct {
    ID          string                    `json:"id"`
    Started     time.Time                 `json:"started"`
    Updated     time.Time                 `json:"updated"`
    Config      StateConfig               `json:"config"`
    URLs        URLState                  `json:"urls"`
    Results     []output.ProbeResult      `json:"results"`
    Stats       ScanStats                 `json:"stats"`
    Checkpoints []Checkpoint              `json:"checkpoints"`
}

// StateConfig stores the configuration used for this scan
type StateConfig struct {
    Concurrency    int      `json:"concurrency"`
    Timeout        int      `json:"timeout"`
    FollowRedirects bool    `json:"follow_redirects"`
    AllSchemes     bool     `json:"all_schemes"`
    CustomPorts    string   `json:"custom_ports"`
    // ... other relevant config
}

// URLState tracks URL processing status
type URLState struct {
    Total      int                 `json:"total"`
    Processed  int                 `json:"processed"`
    Failed     int                 `json:"failed"`
    Pending    []string            `json:"pending"`
    Completed  map[string]bool     `json:"completed"`
    Failed     map[string]string   `json:"failed_urls"`  // URL -> error
}

// ScanStats provides scan statistics
type ScanStats struct {
    TotalRequests    int           `json:"total_requests"`
    SuccessfulProbes int           `json:"successful_probes"`
    FailedProbes     int           `json:"failed_probes"`
    CacheHits        int           `json:"cache_hits"`
    TotalDuration    time.Duration `json:"total_duration"`
    AverageDuration  time.Duration `json:"average_duration"`
}

// Checkpoint represents a point-in-time snapshot
type Checkpoint struct {
    Timestamp   time.Time `json:"timestamp"`
    Processed   int       `json:"processed"`
    Remaining   int       `json:"remaining"`
    ElapsedTime time.Duration `json:"elapsed_time"`
}
```

### 6.4 State Manager

```go
package resume

import (
    "context"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sync"
    "time"
)

type StateManager struct {
    state          *ScanState
    statePath      string
    checkpointFreq time.Duration
    mu             sync.RWMutex
    stopCheckpoint chan struct{}
}

func NewStateManager(statePath string, checkpointFreq time.Duration) *StateManager {
    return &StateManager{
        statePath:      statePath,
        checkpointFreq: checkpointFreq,
        stopCheckpoint: make(chan struct{}),
    }
}

// Initialize creates a new scan state
func (m *StateManager) Initialize(urls []string, config StateConfig) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.state = &ScanState{
        ID:      generateScanID(),
        Started: time.Now(),
        Updated: time.Now(),
        Config:  config,
        URLs: URLState{
            Total:     len(urls),
            Processed: 0,
            Pending:   urls,
            Completed: make(map[string]bool),
            Failed:    make(map[string]string),
        },
        Results:     make([]output.ProbeResult, 0, len(urls)),
        Checkpoints: make([]Checkpoint, 0),
    }

    return m.save()
}

// Load loads an existing scan state
func (m *StateManager) Load() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    data, err := os.ReadFile(m.statePath)
    if err != nil {
        return fmt.Errorf("failed to read state file: %w", err)
    }

    var state ScanState
    if err := json.Unmarshal(data, &state); err != nil {
        return fmt.Errorf("failed to unmarshal state: %w", err)
    }

    m.state = &state
    return nil
}

// MarkCompleted marks a URL as completed
func (m *StateManager) MarkCompleted(url string, result output.ProbeResult) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.state.URLs.Completed[url] = true
    m.state.URLs.Processed++
    m.state.Results = append(m.state.Results, result)
    m.state.Updated = time.Now()

    if result.Error == "" {
        m.state.Stats.SuccessfulProbes++
    } else {
        m.state.Stats.FailedProbes++
        m.state.URLs.Failed[url] = result.Error
    }

    return nil
}

// MarkFailed marks a URL as failed
func (m *StateManager) MarkFailed(url string, err error) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.state.URLs.Failed[url] = err.Error()
    m.state.URLs.Processed++
    m.state.Stats.FailedProbes++
    m.state.Updated = time.Now()

    return nil
}

// GetPendingURLs returns URLs that haven't been processed
func (m *StateManager) GetPendingURLs() []string {
    m.mu.RLock()
    defer m.mu.RUnlock()

    pending := make([]string, 0)
    for _, url := range m.state.URLs.Pending {
        if !m.state.URLs.Completed[url] {
            pending = append(pending, url)
        }
    }

    return pending
}

// StartAutoCheckpoint starts automatic checkpointing
func (m *StateManager) StartAutoCheckpoint(ctx context.Context) {
    ticker := time.NewTicker(m.checkpointFreq)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            if err := m.Checkpoint(); err != nil {
                // Log error
            }
        case <-m.stopCheckpoint:
            return
        case <-ctx.Done():
            return
        }
    }
}

// Checkpoint creates a checkpoint and saves state
func (m *StateManager) Checkpoint() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    checkpoint := Checkpoint{
        Timestamp:   time.Now(),
        Processed:   m.state.URLs.Processed,
        Remaining:   m.state.URLs.Total - m.state.URLs.Processed,
        ElapsedTime: time.Since(m.state.Started),
    }

    m.state.Checkpoints = append(m.state.Checkpoints, checkpoint)
    m.state.Updated = time.Now()

    return m.save()
}

// save writes the state to disk
func (m *StateManager) save() error {
    // Create directory if it doesn't exist
    dir := filepath.Dir(m.statePath)
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }

    // Marshal state
    data, err := json.MarshalIndent(m.state, "", "  ")
    if err != nil {
        return err
    }

    // Write atomically (write to temp file, then rename)
    tmpPath := m.statePath + ".tmp"
    if err := os.WriteFile(tmpPath, data, 0644); err != nil {
        return err
    }

    return os.Rename(tmpPath, m.statePath)
}

// GetProgress returns current progress information
func (m *StateManager) GetProgress() ProgressInfo {
    m.mu.RLock()
    defer m.mu.RUnlock()

    elapsed := time.Since(m.state.Started)
    remaining := m.state.URLs.Total - m.state.URLs.Processed
    var eta time.Duration

    if m.state.URLs.Processed > 0 {
        avgTime := elapsed / time.Duration(m.state.URLs.Processed)
        eta = avgTime * time.Duration(remaining)
    }

    return ProgressInfo{
        Total:       m.state.URLs.Total,
        Processed:   m.state.URLs.Processed,
        Remaining:   remaining,
        Progress:    float64(m.state.URLs.Processed) / float64(m.state.URLs.Total) * 100,
        Elapsed:     elapsed,
        ETA:         eta,
        Successful:  m.state.Stats.SuccessfulProbes,
        Failed:      m.state.Stats.FailedProbes,
    }
}

type ProgressInfo struct {
    Total      int
    Processed  int
    Remaining  int
    Progress   float64       // Percentage
    Elapsed    time.Duration
    ETA        time.Duration
    Successful int
    Failed     int
}
```

### 6.5 Integration with Main

```go
// In cmd/probehttp/main.go

func main() {
    cfg, err := config.ParseFlags()
    // ... setup ...

    var stateManager *resume.StateManager
    var pendingURLs []string

    if cfg.ResumeEnabled {
        stateManager = resume.NewStateManager(cfg.StatePath, 30*time.Second)

        if cfg.Resume {
            // Resume existing scan
            if err := stateManager.Load(); err != nil {
                cfg.Logger.Error("failed to load state", "error", err)
                os.Exit(1)
            }

            pendingURLs = stateManager.GetPendingURLs()
            cfg.Logger.Info("resuming scan",
                "total", stateManager.state.URLs.Total,
                "completed", stateManager.state.URLs.Processed,
                "remaining", len(pendingURLs))
        } else {
            // New scan
            urls := readURLs(inputReader)
            if err := stateManager.Initialize(urls, cfg.ToStateConfig()); err != nil {
                cfg.Logger.Error("failed to initialize state", "error", err)
                os.Exit(1)
            }
            pendingURLs = urls
        }

        // Start auto-checkpointing
        go stateManager.StartAutoCheckpoint(ctx)
    } else {
        // Normal mode without resume
        pendingURLs = readURLs(inputReader)
    }

    // ... expand URLs and probe ...

    // Update state for each result
    for result := range results {
        if stateManager != nil {
            if result.Error != "" {
                stateManager.MarkFailed(result.Input, fmt.Errorf(result.Error))
            } else {
                stateManager.MarkCompleted(result.Input, result)
            }
        }

        // ... output result ...
    }

    // Final checkpoint
    if stateManager != nil {
        stateManager.Checkpoint()
    }
}
```

### 6.6 Configuration

```go
type Config struct {
    // ... existing fields ...

    ResumeEnabled bool
    Resume        bool          // Resume from existing state
    StatePath     string        // Path to state file
    CheckpointFreq time.Duration // Checkpoint frequency
}
```

### 6.7 CLI Flags

```bash
--state-file ./scan-state.json    # State file path
--resume                           # Resume from state file
--checkpoint-freq 30s              # Checkpoint frequency
--no-resume                        # Disable resume (default)
```

### 6.8 Usage Examples

```bash
# Start a new scan with resume capability
./probeHTTP -i large-urls.txt --state-file ./scan.state --checkpoint-freq 30s

# Interrupt with Ctrl+C
^C

# Resume the scan
./probeHTTP --resume --state-file ./scan.state

# Check progress
./probeHTTP --state-file ./scan.state --show-progress

# Clear state and start fresh
./probeHTTP -i urls.txt --state-file ./scan.state --clear-state
```

### 6.9 Progress Display

```go
// Show live progress
func showProgress(stateManager *resume.StateManager) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for range ticker.C {
        progress := stateManager.GetProgress()

        fmt.Printf("\r[%s] Progress: %.2f%% (%d/%d) | Success: %d | Failed: %d | ETA: %s",
            time.Now().Format("15:04:05"),
            progress.Progress,
            progress.Processed,
            progress.Total,
            progress.Successful,
            progress.Failed,
            progress.ETA.Round(time.Second),
        )
    }
}
```

---

## 7. Phase 6: Distributed Mode

**Priority:** P7 (Low)
**Complexity:** Very High
**Timeline:** 8-12 weeks
**Dependencies:** All previous features

### 7.1 Objective

Enable distributed scanning across multiple nodes for large-scale operations, with coordinated task distribution and result aggregation.

### 7.2 Architecture

```
distributed/
├── coordinator/
│   ├── coordinator.go      # Central coordinator
│   ├── scheduler.go        # Task scheduling
│   └── aggregator.go       # Result aggregation
├── worker/
│   ├── worker.go           # Worker node
│   ├── executor.go         # Task execution
│   └── reporter.go         # Result reporting
├── queue/
│   ├── queue.go            # Queue interface
│   ├── redis.go            # Redis queue
│   ├── rabbitmq.go         # RabbitMQ queue
│   └── kafka.go            # Kafka queue (optional)
└── discovery/
    ├── discovery.go        # Service discovery
    └── health.go           # Health checking
```

### 7.3 Components

#### Message Queue Interface

```go
package queue

import "context"

type Queue interface {
    // Producer methods
    Enqueue(ctx context.Context, task Task) error
    EnqueueBatch(ctx context.Context, tasks []Task) error

    // Consumer methods
    Dequeue(ctx context.Context) (Task, error)
    Ack(ctx context.Context, taskID string) error
    Nack(ctx context.Context, taskID string, requeue bool) error

    // Management
    Size(ctx context.Context) (int64, error)
    Purge(ctx context.Context) error
    Close() error
}

type Task struct {
    ID          string
    URL         string
    Priority    int
    Retries     int
    MaxRetries  int
    Config      TaskConfig
    CreatedAt   time.Time
    StartedAt   time.Time
    CompletedAt time.Time
}

type TaskConfig struct {
    Timeout         int
    FollowRedirects bool
    CustomHeaders   map[string]string
    // ... other config
}
```

#### Coordinator

```go
package coordinator

type Coordinator struct {
    queue        queue.Queue
    resultStore  ResultStore
    workers      map[string]*WorkerInfo
    scheduler    *Scheduler
    aggregator   *Aggregator
}

func (c *Coordinator) Start(ctx context.Context) error {
    // Start scheduler
    go c.scheduler.Run(ctx)

    // Start aggregator
    go c.aggregator.Run(ctx)

    // Start health checker
    go c.healthCheck(ctx)

    return nil
}

func (c *Coordinator) SubmitScan(urls []string, config ScanConfig) (string, error) {
    scanID := generateScanID()

    // Create tasks
    tasks := make([]queue.Task, len(urls))
    for i, url := range urls {
        tasks[i] = queue.Task{
            ID:         fmt.Sprintf("%s-%d", scanID, i),
            URL:        url,
            Priority:   config.Priority,
            MaxRetries: config.MaxRetries,
            Config:     config.ToTaskConfig(),
            CreatedAt:  time.Now(),
        }
    }

    // Enqueue tasks
    if err := c.queue.EnqueueBatch(ctx, tasks); err != nil {
        return "", err
    }

    return scanID, nil
}
```

#### Worker

```go
package worker

type Worker struct {
    id          string
    coordinator string
    queue       queue.Queue
    resultChan  chan Result
    prober      *probe.Prober
}

func (w *Worker) Start(ctx context.Context) error {
    // Register with coordinator
    if err := w.register(); err != nil {
        return err
    }

    // Start processing loop
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := w.processTask(ctx); err != nil {
                // Handle error
            }
        }
    }
}

func (w *Worker) processTask(ctx context.Context) error {
    // Dequeue task
    task, err := w.queue.Dequeue(ctx)
    if err != nil {
        return err
    }

    // Execute probe
    result := w.prober.ProbeURL(ctx, task.URL, task.URL)

    // Send result
    w.resultChan <- Result{
        TaskID: task.ID,
        Result: result,
    }

    // Acknowledge task
    return w.queue.Ack(ctx, task.ID)
}
```

### 7.4 Queue Implementations

#### Redis Queue

```go
package queue

import (
    "context"
    "encoding/json"

    "github.com/go-redis/redis/v8"
)

type RedisQueue struct {
    client     *redis.Client
    queueName  string
    processing string
}

func NewRedisQueue(addr string, queueName string) (*RedisQueue, error) {
    client := redis.NewClient(&redis.Options{
        Addr: addr,
    })

    return &RedisQueue{
        client:     client,
        queueName:  queueName,
        processing: queueName + ":processing",
    }, nil
}

func (q *RedisQueue) Enqueue(ctx context.Context, task Task) error {
    data, err := json.Marshal(task)
    if err != nil {
        return err
    }

    return q.client.RPush(ctx, q.queueName, data).Err()
}

func (q *RedisQueue) Dequeue(ctx context.Context) (Task, error) {
    // Blocking pop with timeout
    result, err := q.client.BLMove(ctx, q.queueName, q.processing, "RIGHT", "LEFT", 5*time.Second).Result()
    if err != nil {
        return Task{}, err
    }

    var task Task
    if err := json.Unmarshal([]byte(result), &task); err != nil {
        return Task{}, err
    }

    return task, nil
}

func (q *RedisQueue) Ack(ctx context.Context, taskID string) error {
    // Remove from processing queue
    return q.client.LRem(ctx, q.processing, 1, taskID).Err()
}
```

### 7.5 CLI Commands

#### Coordinator Mode

```bash
# Start coordinator
./probeHTTP coordinator \
    --redis-addr localhost:6379 \
    --http-addr :8080 \
    --workers-min 1 \
    --workers-max 100

# Submit scan via API
curl -X POST http://localhost:8080/scans \
  -H "Content-Type: application/json" \
  -d '{
    "urls": ["http://example.com", ...],
    "config": {
      "concurrency": 10,
      "timeout": 30
    }
  }'

# Check scan status
curl http://localhost:8080/scans/scan-123

# Get results
curl http://localhost:8080/scans/scan-123/results
```

#### Worker Mode

```bash
# Start worker
./probeHTTP worker \
    --coordinator http://coordinator:8080 \
    --redis-addr localhost:6379 \
    --worker-id worker-1 \
    --concurrency 10

# Start multiple workers
for i in {1..5}; do
    ./probeHTTP worker \
        --coordinator http://coordinator:8080 \
        --worker-id worker-$i &
done
```

### 7.6 Docker Deployment

```yaml
# docker-compose.yml
version: '3.8'

services:
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis-data:/data

  coordinator:
    build: .
    command: coordinator --redis-addr redis:6379 --http-addr :8080
    ports:
      - "8080:8080"
    depends_on:
      - redis
    environment:
      - LOG_LEVEL=info

  worker:
    build: .
    command: worker --coordinator http://coordinator:8080 --redis-addr redis:6379
    depends_on:
      - coordinator
      - redis
    deploy:
      replicas: 5
    environment:
      - LOG_LEVEL=info

volumes:
  redis-data:
```

### 7.7 Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: probehttp-coordinator
spec:
  replicas: 1
  selector:
    matchLabels:
      app: probehttp-coordinator
  template:
    metadata:
      labels:
        app: probehttp-coordinator
    spec:
      containers:
      - name: coordinator
        image: probehttp:latest
        args:
          - coordinator
          - --redis-addr=redis:6379
          - --http-addr=:8080
        ports:
        - containerPort: 8080
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: probehttp-worker
spec:
  replicas: 10
  selector:
    matchLabels:
      app: probehttp-worker
  template:
    metadata:
      labels:
        app: probehttp-worker
    spec:
      containers:
      - name: worker
        image: probehttp:latest
        args:
          - worker
          - --coordinator=http://coordinator:8080
          - --redis-addr=redis:6379
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: probehttp-worker-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: probehttp-worker
  minReplicas: 5
  maxReplicas: 100
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

---

## 8. Phase 7: Screenshot Capture

**Priority:** P6 (Low)
**Complexity:** Medium
**Timeline:** 3-4 weeks
**Dependencies:** None (isolated feature)

### 8.1 Objective

Capture screenshots of web pages for visual verification and analysis, storing them alongside probe results.

### 8.2 Architecture

```
internal/screenshot/
├── capture.go          # Screenshot capture logic
├── browser.go          # Browser pool management
└── storage.go          # Screenshot storage
```

### 8.3 Implementation

```go
package screenshot

import (
    "context"
    "fmt"
    "time"

    "github.com/chromedp/chromedp"
)

type Capturer struct {
    browserCtx context.Context
    cancel     context.CancelFunc
    config     Config
}

type Config struct {
    Enabled       bool
    StoragePath   string
    Format        string // png, jpeg
    Quality       int    // For JPEG
    FullPage      bool   // Capture full page vs viewport
    Width         int
    Height        int
    WaitTimeout   time.Duration
    WaitForLoad   bool
}

func NewCapturer(config Config) (*Capturer, error) {
    opts := append(chromedp.DefaultExecAllocatorOptions[:],
        chromedp.Flag("headless", true),
        chromedp.Flag("disable-gpu", true),
        chromedp.Flag("no-sandbox", true),
        chromedp.WindowSize(config.Width, config.Height),
    )

    allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
    browserCtx, _ := chromedp.NewContext(allocCtx)

    return &Capturer{
        browserCtx: browserCtx,
        cancel:     cancel,
        config:     config,
    }, nil
}

func (c *Capturer) Capture(ctx context.Context, url string) ([]byte, error) {
    var buf []byte

    tasks := chromedp.Tasks{
        chromedp.Navigate(url),
    }

    if c.config.WaitForLoad {
        tasks = append(tasks, chromedp.WaitReady("body"))
    }

    if c.config.FullPage {
        tasks = append(tasks, chromedp.FullScreenshot(&buf, 90))
    } else {
        tasks = append(tasks, chromedp.CaptureScreenshot(&buf))
    }

    timeoutCtx, cancel := context.WithTimeout(ctx, c.config.WaitTimeout)
    defer cancel()

    if err := chromedp.Run(timeoutCtx, tasks); err != nil {
        return nil, fmt.Errorf("failed to capture screenshot: %w", err)
    }

    return buf, nil
}

func (c *Capturer) Close() {
    c.cancel()
}
```

### 8.4 Integration

```go
// In prober.go
func (p *Prober) ProbeURL(ctx context.Context, probeURL string, originalInput string) output.ProbeResult {
    result := p.probeURLOnce(ctx, probeURL, originalInput)

    // Capture screenshot if enabled
    if p.screenshotter != nil && result.Error == "" {
        screenshot, err := p.screenshotter.Capture(ctx, probeURL)
        if err != nil {
            p.config.Logger.Warn("failed to capture screenshot",
                "url", probeURL,
                "error", err)
        } else {
            // Save screenshot
            filename := generateScreenshotFilename(probeURL)
            path := filepath.Join(p.config.ScreenshotPath, filename)
            if err := os.WriteFile(path, screenshot, 0644); err != nil {
                p.config.Logger.Warn("failed to save screenshot",
                    "url", probeURL,
                    "error", err)
            } else {
                result.ScreenshotPath = path
            }
        }
    }

    return result
}
```

### 8.5 Configuration

```go
type Config struct {
    // ... existing fields ...

    ScreenshotEnabled  bool
    ScreenshotPath     string
    ScreenshotFormat   string
    ScreenshotFullPage bool
    ScreenshotWidth    int
    ScreenshotHeight   int
}
```

### 8.6 CLI Flags

```bash
--screenshot                    # Enable screenshots
--screenshot-path ./screenshots # Screenshot directory
--screenshot-format png         # Format: png, jpeg
--screenshot-full-page          # Capture full page
--screenshot-width 1920         # Viewport width
--screenshot-height 1080        # Viewport height
```

### 8.7 Usage

```bash
# Basic screenshot capture
./probeHTTP -i urls.txt --screenshot --screenshot-path ./shots

# Full page screenshots
./probeHTTP -i urls.txt --screenshot --screenshot-full-page

# Custom dimensions
./probeHTTP -i urls.txt --screenshot \
    --screenshot-width 2560 --screenshot-height 1440
```

---

## 9. Implementation Roadmap

### Quarter 1 (Months 1-3): Foundation

**Month 1:**
- ✅ Week 1-2: Multiple output formats (JSON, JSONL, CSV)
- ✅ Week 3-4: XML and Markdown formatters

**Month 2:**
- ✅ Week 1-2: Memory cache implementation
- ✅ Week 3-4: Disk cache implementation

**Month 3:**
- ✅ Week 1-2: Prometheus metrics integration
- ✅ Week 3-4: Testing, documentation, v3.0 release

### Quarter 2 (Months 4-6): Advanced Features

**Month 4:**
- ⏳ Week 1-3: Plugin system architecture and interfaces
- ⏳ Week 4: Example plugins (security headers, tech detection)

**Month 5:**
- ⏳ Week 1-2: Plugin loader and registry
- ⏳ Week 3-4: Plugin documentation and SDK

**Month 6:**
- ⏳ Week 1-2: Resume capability implementation
- ⏳ Week 3-4: Testing, documentation, v3.1 release

### Quarter 3 (Months 7-9): Visual & Distribution Prep

**Month 7:**
- ⏳ Week 1-3: Screenshot capture with chromedp
- ⏳ Week 4: Screenshot storage and optimization

**Month 8:**
- ⏳ Week 1-4: Testing, optimization, v3.2 release

**Month 9:**
- ⏳ Week 1-4: Distributed mode architecture design

### Quarter 4 (Months 10-12): Distributed Mode

**Month 10:**
- ⏳ Week 1-2: Queue implementations (Redis, RabbitMQ)
- ⏳ Week 3-4: Coordinator implementation

**Month 11:**
- ⏳ Week 1-2: Worker implementation
- ⏳ Week 3-4: Service discovery and health checking

**Month 12:**
- ⏳ Week 1-2: Integration testing
- ⏳ Week 3-4: Documentation, deployment guides, v4.0 release

---

## 10. Testing Strategy

### 10.1 Unit Testing

Each feature must have:
- **Coverage:** >80% line coverage
- **Benchmarks:** Performance regression tests
- **Fuzz tests:** Input validation
- **Race detection:** `go test -race`

### 10.2 Integration Testing

- **End-to-end tests** for each feature
- **Compatibility tests** with existing features
- **Performance tests** under load
- **Failure scenario tests**

### 10.3 Load Testing

```bash
# Test with 100k URLs
seq 1 100000 | xargs -I {} echo "http://example{}.com" > urls.txt
./probeHTTP -i urls.txt -c 100 --cache

# Distributed load test
vegeta attack -rate=1000 -duration=60s -targets=targets.txt
```

### 10.4 Security Testing

- **Vulnerability scanning:** `govulncheck`
- **Static analysis:** `gosec`
- **Dependency auditing:** `nancy`
- **Penetration testing:** Manual security review

---

## 11. Migration & Compatibility

### 11.1 Versioning Strategy

**Semantic Versioning:**
- v3.0: Output formats + Caching + Metrics
- v3.1: Plugin system + Resume
- v3.2: Screenshots
- v4.0: Distributed mode (breaking changes allowed)

### 11.2 Deprecation Policy

- Features deprecated in v3.x
- Removal allowed in v4.0
- 6-month deprecation notice minimum

### 11.3 Breaking Changes

**Allowed in v4.0 only:**
- Configuration format changes
- CLI flag restructuring
- API endpoint changes (distributed mode)

**Never breaking:**
- JSON output format (unless opted-in)
- Basic CLI flags (-i, -o, -c, -t)

---

## 12. Risk Assessment

### 12.1 Technical Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Performance degradation | Medium | High | Benchmarks, profiling |
| Memory leaks in plugins | Medium | High | Resource limits, monitoring |
| Cache corruption | Low | Medium | Checksums, validation |
| Distributed sync issues | High | High | Idempotency, retries |
| Screenshot timeout/hangs | Medium | Low | Timeouts, circuit breakers |

### 12.2 Operational Risks

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Increased complexity | High | Medium | Documentation, examples |
| Plugin ecosystem quality | High | Medium | Plugin validation, reviews |
| Distributed deployment complexity | High | High | Docker/K8s templates |
| Resource exhaustion | Medium | High | Resource limits, quotas |

### 12.3 Mitigation Strategies

**Performance:**
- Comprehensive benchmarking
- Profiling (CPU, memory, goroutines)
- Load testing before release

**Reliability:**
- Extensive error handling
- Graceful degradation
- Circuit breakers for external services

**Security:**
- Plugin sandboxing (WASM preferred)
- Resource limits per plugin
- Security audits

**Complexity:**
- Modular design (features can be disabled)
- Progressive enhancement
- Comprehensive documentation

---

## 13. Success Criteria

### 13.1 Performance Targets

- **No regression:** <5% performance impact with all features disabled
- **Cache efficiency:** >60% hit rate for repeated scans
- **Screenshot latency:** <2s per screenshot
- **Distributed throughput:** Linear scaling up to 100 workers

### 13.2 Reliability Targets

- **Uptime:** 99.9% (distributed coordinator)
- **Crash rate:** <0.1%
- **Data loss:** 0% (state persistence)
- **Recovery time:** <1min (from coordinator failure)

### 13.3 Adoption Targets

- **Downloads:** 10k+ in first 3 months
- **Plugin ecosystem:** 10+ community plugins by Q4
- **Production deployments:** 50+ enterprises by Q4

---

## 14. Documentation Plan

### 14.1 User Documentation

- Feature guides for each major feature
- Migration guides for each version
- Best practices and recipes
- Troubleshooting guide

### 14.2 Developer Documentation

- Plugin development guide
- API reference (distributed mode)
- Architecture overview
- Contributing guidelines

### 14.3 Operational Documentation

- Deployment guides (Docker, K8s, bare metal)
- Monitoring and alerting setup
- Performance tuning guide
- Disaster recovery procedures

---

## 15. Conclusion

This implementation plan provides a comprehensive roadmap for transforming probeHTTP into an enterprise-ready, distributed reconnaissance platform. The phased approach ensures:

1. **Incremental value delivery** - Each phase provides standalone value
2. **Risk mitigation** - Early features inform later design decisions
3. **Backward compatibility** - Existing users unaffected
4. **Flexibility** - Features can be implemented independently
5. **Scalability** - Architecture supports massive scale

**Estimated Total Effort:** 30-40 weeks (7.5-10 months)
**Estimated Team Size:** 2-3 engineers
**Target Completion:** Q4 2025

**Next Steps:**
1. Review and approval of this plan
2. Assign engineering resources
3. Begin Phase 1 implementation
4. Set up project tracking (GitHub Projects)
5. Establish monthly review cadence

---

**Document Version:** 1.0
**Last Updated:** 2025-11-10
**Status:** Draft - Pending Review
