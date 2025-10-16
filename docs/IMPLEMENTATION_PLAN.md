# Implementation Plan: Multi-Scheme and Multi-Port Probing

## Current State Analysis

### What Exists:
- ✅ Single URL probing (one input → one probe)
- ✅ Basic URL validation and scheme defaulting (adds `http://` if missing)
- ✅ Concurrent processing with worker pool
- ✅ CLI flag infrastructure

### What's Missing:
- ❌ URL expansion logic (one input → multiple probe URLs)
- ❌ Multi-scheme support flags
- ❌ Multi-port support flags
- ❌ Common ports configuration
- ❌ Custom ports parsing
- ❌ URL component extraction and reconstruction
- ❌ Tests for URL expansion
- ❌ Default behavior for scheme/port combinations

## Requirements Breakdown

### 1. Default Behavior (No Flags)
**Current:** `secinto.com` → `http://secinto.com` (single probe)
**New:** `secinto.com` → `http://secinto.com`, `https://secinto.com` (two probes)

**Current:** `secinto.com:80` → `http://secinto.com:80` (single probe)
**New:** `secinto.com:80` → `http://secinto.com:80`, `https://secinto.com:80` (two probes)

### 2. All Schemes Flag (`--all-schemes` / `-as`)
**Input:** `https://secinto.com:443`
**Output:** `https://secinto.com:443`, `http://secinto.com:443`

**Input:** `http://example.com`
**Output:** `http://example.com`, `https://example.com`

### 3. Ignore Ports Flag (`--ignore-ports` / `-ip`)
**Input:** `https://secinto.com:443`
**Output:**
- `https://secinto.com:80`
- `https://secinto.com:443`
- `https://secinto.com:8080`
- `https://secinto.com:8443`
- `https://secinto.com:10443`
- (additional common HTTPS ports)

**Input:** `secinto.com:9999`
**Output:** Test with all common HTTP + HTTPS ports (ignores 9999)

### 4. Combined Flags (`--all-schemes` + `--ignore-ports`)
**Input:** `https://secinto.com:443`
**Output:** All combinations of {http, https} × {common ports}
- `http://secinto.com:80`
- `http://secinto.com:8080`
- `http://secinto.com:8000`
- `https://secinto.com:443`
- `https://secinto.com:8443`
- `https://secinto.com:10443`
- etc.

### 5. Custom Ports Flag (`--ports` / `-p`)
**Input:** `secinto.com` with `--ports "80,443,8080,9000"`
**Output:** Test specified ports only
- `http://secinto.com:80`
- `https://secinto.com:80`
- `http://secinto.com:443`
- `https://secinto.com:443`
- etc.

## Implementation Design

### 1. Configuration Changes

```go
type Config struct {
    // Existing fields...
    InputFile       string
    OutputFile      string
    FollowRedirects bool
    MaxRedirects    int
    Timeout         int
    Concurrency     int
    Silent          bool

    // NEW fields
    AllSchemes      bool   // Test both HTTP and HTTPS
    IgnorePorts     bool   // Ignore input port, test common ports
    CustomPorts     string // Comma-separated port list
}
```

### 2. Default Port Configuration

```go
var (
    // Default common HTTP ports
    DefaultHTTPPorts = []string{"80", "8000", "8080", "8888"}

    // Default common HTTPS ports
    DefaultHTTPSPorts = []string{"443", "8443", "10443", "8444"}
)
```

### 3. URL Parsing Structure

```go
type ParsedURL struct {
    Original string // Original input
    Scheme   string // http, https, or empty
    Host     string // hostname only (no port)
    Port     string // port number or empty
    Path     string // path component
}
```

### 4. URL Expansion Function

```go
// expandURLs takes an input URL and returns multiple URLs to probe
// based on configuration flags
func expandURLs(inputURL string) []string {
    parsed := parseInputURL(inputURL)

    schemes := getSchemesToTest(parsed)
    ports := getPortsToTest(parsed)

    var urls []string
    for _, scheme := range schemes {
        for _, port := range ports {
            urls = append(urls, buildURL(scheme, parsed.Host, port, parsed.Path))
        }
    }

    return urls
}

// getSchemesToTest returns schemes to test based on flags
func getSchemesToTest(parsed ParsedURL) []string {
    // Default behavior: test both http and https
    if config.AllSchemes || parsed.Scheme == "" {
        return []string{"http", "https"}
    }

    // If scheme provided and AllSchemes is true, test both
    if config.AllSchemes {
        return []string{"http", "https"}
    }

    // Otherwise use input scheme only
    return []string{parsed.Scheme}
}

// getPortsToTest returns ports to test based on flags
func getPortsToTest(parsed ParsedURL) []string {
    // Custom ports override everything
    if config.CustomPorts != "" {
        return parsePortList(config.CustomPorts)
    }

    // Ignore ports: use default common ports
    if config.IgnorePorts {
        return getDefaultPorts()
    }

    // If port specified in input, use it
    if parsed.Port != "" {
        return []string{parsed.Port}
    }

    // Default: use standard ports for each scheme
    // This will be handled per-scheme in expansion
    return []string{"default"}
}
```

### 5. Integration Points

**Main flow changes:**
```go
// main.go - line ~111
urls := readURLs(inputReader)

// NEW: Expand URLs based on configuration
expandedURLs := []string{}
for _, url := range urls {
    expandedURLs = append(expandedURLs, expandURLs(url)...)
}

// Process expanded URLs with worker pool
results := processURLs(expandedURLs, config.Concurrency)
```

### 6. Behavior Matrix

| Input | AllSchemes | IgnorePorts | CustomPorts | Output |
|-------|-----------|-------------|-------------|--------|
| `example.com` | false | false | - | `http://example.com`, `https://example.com` |
| `example.com` | true | false | - | `http://example.com`, `https://example.com` |
| `http://example.com` | false | false | - | `http://example.com`, `https://example.com` (default) |
| `http://example.com` | true | false | - | `http://example.com`, `https://example.com` |
| `example.com:80` | false | false | - | `http://example.com:80`, `https://example.com:80` |
| `example.com:80` | false | true | - | All schemes × common ports |
| `example.com` | false | false | "80,443" | `http://example.com:80`, `https://example.com:443` |
| `https://example.com:443` | true | true | - | All schemes × common ports |

## Testing Requirements

### 1. New Unit Tests (url_expansion_test.go)

```
TestParseInputURL
- URL with scheme, host, port, path
- URL with only host
- URL with host and port
- URL with scheme and host
- Malformed URLs

TestExpandURLs_DefaultBehavior
- Input: example.com → http://example.com, https://example.com
- Input: example.com:80 → http://example.com:80, https://example.com:80

TestExpandURLs_AllSchemes
- Input: http://example.com → http://example.com, https://example.com
- Input: https://example.com:443 → http://example.com:443, https://example.com:443

TestExpandURLs_IgnorePorts
- Input: example.com:9999 → All common ports for both schemes
- Input: https://example.com:443 → All common HTTPS ports

TestExpandURLs_CustomPorts
- Input: example.com with ports "80,443,8080"
- Verify only specified ports are used

TestExpandURLs_Combined
- AllSchemes + IgnorePorts
- AllSchemes + CustomPorts
- All three flags together

TestGetSchemesToTest
- No scheme provided
- HTTP scheme with AllSchemes=false
- HTTPS scheme with AllSchemes=true

TestGetPortsToTest
- No port with IgnorePorts=false
- Port 80 with IgnorePorts=true
- CustomPorts parsing
```

### 2. Updated Integration Tests

```
TestProbeURL_MultiScheme
- Verify both HTTP and HTTPS probing works
- Verify different schemes produce different results

TestProbeURL_MultiPort
- Test same host on different ports
- Verify port-specific behavior

TestEndToEnd_URLExpansion
- Test expansion with mock servers on multiple ports
- Verify all combinations are probed
```

### 3. Updated System Tests

```
TestBinary_AllSchemes
- Run binary with --all-schemes flag
- Verify output contains multiple results

TestBinary_IgnorePorts
- Run binary with --ignore-ports flag
- Verify common ports are tested

TestBinary_CustomPorts
- Run binary with --ports flag
- Verify only specified ports are tested
```

## Implementation Steps

1. ✅ **Phase 1: Design & Planning**
   - Document requirements
   - Create implementation plan
   - Define test cases

2. **Phase 2: Core Changes**
   - Update Config struct with new fields
   - Add CLI flags for AllSchemes, IgnorePorts, CustomPorts
   - Define default port constants

3. **Phase 3: URL Expansion**
   - Implement ParsedURL struct
   - Implement parseInputURL() function
   - Implement getSchemesToTest() function
   - Implement getPortsToTest() function
   - Implement expandURLs() function
   - Implement buildURL() function

4. **Phase 4: Integration**
   - Update main() to use URL expansion
   - Update ProbeResult to preserve original input
   - Test basic functionality manually

5. **Phase 5: Testing**
   - Create url_expansion_test.go with unit tests
   - Update existing tests to handle new behavior
   - Add integration tests for multi-scheme/port
   - Add system tests for CLI flags

6. **Phase 6: Documentation**
   - Update README.md with new flags
   - Update TESTING.md with new test coverage
   - Add examples for common use cases

## Edge Cases to Handle

1. **Invalid port specifications:**
   - Non-numeric ports
   - Ports out of range (1-65535)
   - Empty port list

2. **URL parsing edge cases:**
   - IPv6 addresses
   - URLs with authentication (user:pass@host)
   - URLs with query parameters
   - URLs with fragments

3. **Default behavior clarification:**
   - What happens when no flags are set?
   - Should we test both schemes by default?

4. **Performance considerations:**
   - With IgnorePorts, a single input could generate 20+ probes
   - Need to ensure concurrency settings are respected
   - Potential for result explosion with large input files

## Breaking Changes

⚠️ **Default behavior change:**
- OLD: `example.com` → `http://example.com` (1 probe)
- NEW: `example.com` → `http://example.com`, `https://example.com` (2 probes)

**Migration path:**
- Add `--single-probe` or `--no-expand` flag to preserve old behavior
- Document breaking change in release notes
- Provide migration guide

## Questions for Clarification

1. **Default behavior:** Should we test both schemes by default, or require explicit `--all-schemes`?
   - Proposed: Test both by default (more thorough scanning)

2. **Port defaults:** When scheme is provided but port isn't, should we:
   - Use standard port for that scheme (80 for http, 443 for https)?
   - Test all common ports for that scheme?
   - Proposed: Use standard port only

3. **Custom ports format:** Should we support port ranges?
   - Example: `--ports "80,443,8000-9000"`
   - Proposed: Start with comma-separated list, add ranges later

4. **Output deduplication:** If expansion creates duplicate URLs, should we:
   - Probe each duplicate (potential waste)
   - Deduplicate before probing (recommended)
   - Proposed: Deduplicate

5. **Input preservation:** Should `ProbeResult.Input` show:
   - Original input before expansion?
   - Expanded URL being probed?
   - Proposed: Original input for traceability
