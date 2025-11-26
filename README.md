# probeHTTP

A comprehensive, high-performance HTTP/HTTPS probing tool written in Go that performs intelligent web reconnaissance with metadata extraction, hashing, and content analysis.

> **✨ Version 2.0 - Major Refactoring & Performance Enhancement**
> 
> This version includes significant performance improvements, security hardening, parallel TLS/protocol attempts, and architectural enhancements. See [docs/IMPLEMENTATION_SUMMARY.md](docs/IMPLEMENTATION_SUMMARY.md) for complete details.

## Features

### Core Capabilities
- **Multi-scheme and multi-port probing** - Test HTTP and HTTPS across multiple ports simultaneously
- **Parallel TLS/Protocol attempts** - Automatically tries TLS 1.3/1.2/1.1/1.0 with HTTP/3, HTTP/2, and HTTP/1.1
- **HTTP/3 (QUIC) support** - First-class support for modern QUIC protocol with automatic fallback
- **MMH3 hash calculation** for response body and headers (content fingerprinting)
- **HTML title extraction** with intelligent fallback support (og:title, twitter:title, etc.)
- **Web server fingerprinting** and content-type detection
- **Concurrent processing** with optimized worker pool and connection pooling
- **JSON output format** with comprehensive metadata including TLS details
- **Flexible I/O** - stdin/stdout or file-based operation
- **Port range support** - e.g., `8000-8010` for scanning ranges

### New in v2.0
- 🚀 **90% faster title extraction** via regex pre-compilation
- 🔒 **TLS 1.2+ enforcement** with strong cipher suites by default
- ⚡ **Connection pooling** (40% improvement in concurrent scenarios)
- 🛡️ **Input validation** with private IP blocking option
- 🔄 **Retry mechanism** with exponential backoff
- ⏱️ **Rate limiting** per host (10 req/s default, prevents server overload)
- 📝 **Structured logging** with slog (JSON output for stderr)
- 🛑 **Graceful shutdown** on Ctrl+C with context cancellation
- 💾 **Response body size limits** (10MB default, prevents DoS)
- 🎯 **Context-based cancellation** throughout the codebase
- 🌐 **HTTP/3 (QUIC) support** - Parallel protocol attempts with automatic fallback
- 🔐 **Parallel TLS attempts** - Tries multiple TLS versions and cipher suites simultaneously
- 📊 **TLS metadata reporting** - Reports TLS version, cipher suite, and protocol used
- 🔍 **URL deduplication** - Automatically removes duplicate endpoints
- 📦 **Comprehensive test suite** - 11 test files with integration, fuzzing, and benchmarks

## Installation

### From Source

```bash
# Clone repository
git clone https://github.com/secinto/probeHTTP
cd probeHTTP

# Build using Makefile
make build

# Or build manually
go build -o probeHTTP ./cmd/probehttp
```

### Pre-built Binaries

Download from [Releases](https://github.com/secinto/probeHTTP/releases) or build for all platforms:

```bash
make build-all  # Creates binaries in dist/ for Linux, macOS, Windows (amd64 & arm64)

### Version Information

Build with custom version information:

```bash
# Build with specific version
VERSION=1.2.3 make build

# Check version
./probeHTTP --version
# Output: probeHTTP 1.2.3 (commit: abc1234, built: 2024-01-01T12:00:00Z, go: go1.24.0)

# View build-time version info
make version
```

The binary automatically includes:
- Semantic version (default: `1.0.0`, customizable via `VERSION` env var)
- Git commit SHA (automatically detected)
- Build date (ISO 8601 format)
- Go version used for compilation

```

### Dependencies

- Go 1.24+
- `github.com/twmb/murmur3` - MMH3 hashing
- `golang.org/x/net/html` - HTML parsing
- `github.com/quic-go/quic-go` - HTTP/3 (QUIC) support
- `golang.org/x/time` - Rate limiting

## Usage

### Basic Usage

```bash
# Probe from stdin
echo "https://example.com" | ./probeHTTP

# Probe from file
./probeHTTP -i urls.txt

# Output to file (JSON to file, successful URLs to stdout)
./probeHTTP -i urls.txt -o results.json

# Multiple URLs
echo -e "https://example.com\nhttps://github.com" | ./probeHTTP
```

### Command-Line Flags

| Flag | Short | Description | Default |
|------|-------|-------------|---------|
| `--input` | `-i` | Input file path | stdin |
| `--output` | `-o` | Output file path | stdout |
| `--follow-redirects` | `-fr` | Follow HTTP redirects | true |
| `--max-redirects` | `-maxr` | Maximum number of redirects | 10 |
| `--timeout` | `-t` | Request timeout in seconds | 30 |
| `--concurrency` | `-c` | Number of concurrent requests | 20 |
| `--silent` | | Silent mode (errors only to stderr) | false |
| `--debug` | `-d` | Debug mode (verbose stderr output) | false |
| `--all-schemes` | `-as` | Test both HTTP and HTTPS (overrides input scheme) | false |
| `--ignore-ports` | `-ip` | Ignore input ports and test common HTTP/HTTPS ports | false |
| `--ports` | `-p` | Custom port list (comma-separated, supports ranges) | - |
| `--user-agent` | `-ua` | Custom User-Agent header | (default browser UA) |
| `--random-user-agent` | `-rua` | Use random User-Agent from pool | false |
| `--same-host-only` | `-sho` | Only follow redirects to same hostname | false |
| `--insecure` | `-k` | Skip TLS certificate verification | false |
| `--allow-private` | | Allow scanning private IP addresses | false |
| `--retries` | | Maximum number of retries for failed requests | 0 |
| `--tls-timeout` | | Timeout for TLS handshake attempts in seconds | 10 |
| `--tls-handshake-timeout` | | Alias for --tls-timeout | 10 |
| `--disable-http3` | | Disable HTTP/3 (QUIC) support | false |
| `--debug-log` | | Write detailed debug logs to file | - |
| `--version` | `-v` | Show version information | - |

### Examples

#### Basic Examples

```bash
# Probe with custom timeout and concurrency
echo "https://example.com" | ./probeHTTP -t 10 -c 5

# Don't follow redirects
echo "https://google.com" | ./probeHTTP -fr=false

# Save output to file, print successful URLs to console
./probeHTTP -i urls.txt -o results.json

# Silent mode (suppress info logs)
./probeHTTP -i urls.txt -silent

# Debug mode with detailed logging to file
./probeHTTP -i urls.txt --debug-log debug.log
```

#### Multi-Scheme and Multi-Port Examples

```bash
# Default behavior: Test both HTTP and HTTPS on standard ports
echo "example.com" | ./probeHTTP
# Probes: http://example.com:80 and https://example.com:443

# Test both schemes even when scheme is specified
echo "https://example.com" | ./probeHTTP --all-schemes
# Probes: http://example.com:80 and https://example.com:443

# Ignore input port and test common HTTP/HTTPS ports
echo "example.com:9999" | ./probeHTTP --ignore-ports
# Probes: http://example.com on ports 80,8000,8080,8888
#         https://example.com on ports 443,8443,10443,8444

# Test custom ports
echo "example.com" | ./probeHTTP --ports "80,443,8080"
# Probes: http://example.com:80, https://example.com:80,
#         http://example.com:443, https://example.com:443,
#         http://example.com:8080, https://example.com:8080

# Test port range
echo "example.com" | ./probeHTTP --ports "8000-8005"
# Probes: Ports 8000,8001,8002,8003,8004,8005 on both HTTP and HTTPS

# Combine flags: Test all schemes with custom ports
echo "https://example.com" | ./probeHTTP --all-schemes --ports "80,443,8080-8082"
# Probes: All combinations of {http,https} × {80,443,8080,8081,8082}

# Combine flags: Test all schemes with common ports
echo "https://example.com" | ./probeHTTP --all-schemes --ignore-ports
# Probes: Both HTTP and HTTPS on all common ports
```

#### Advanced Examples

```bash
# Retry failed requests with exponential backoff
./probeHTTP -i urls.txt --retries 3

# Custom TLS timeout for slow servers
./probeHTTP -i urls.txt --tls-timeout 20

# Disable HTTP/3 for compatibility
./probeHTTP -i urls.txt --disable-http3

# Skip TLS certificate verification (testing only!)
./probeHTTP -i urls.txt -k

# Allow scanning private IPs (security testing)
./probeHTTP -i urls.txt --allow-private
```

## Output Format

The tool outputs JSON for each successfully probed URL:

```json
{
  "timestamp": "2024-11-26T07:58:48+01:00",
  "hash": {
    "body_mmh3": "3570969655",
    "header_mmh3": "3370267568"
  },
  "port": "443",
  "url": "https://example.com",
  "input": "example.com",
  "final_url": "https://example.com/",
  "title": "Example Domain",
  "scheme": "https",
  "webserver": "",
  "content_type": "text/html; charset=UTF-8",
  "method": "GET",
  "host": "example.com",
  "path": "/",
  "time": "470.210792ms",
  "chain_status_codes": [200],
  "chain_hosts": ["example.com"],
  "words": 25,
  "lines": 2,
  "status_code": 200,
  "content_length": 1256,
  "tls_version": "1.3",
  "cipher_suite": "TLS_AES_128_GCM_SHA256",
  "protocol": "HTTP/2",
  "tls_config_strategy": "TLS 1.3 Modern"
}
```

### Output Fields

| Field | Description |
|-------|-------------|
| `timestamp` | RFC3339 formatted timestamp |
| `hash.body_mmh3` | MMH3 hash of response body (for content fingerprinting) |
| `hash.header_mmh3` | MMH3 hash of concatenated headers |
| `port` | Port number used for the request |
| `url` | Original request URL |
| `input` | Original input from user (before expansion) |
| `final_url` | Final URL after following redirects |
| `title` | HTML page title (with fallback to og:title, twitter:title) |
| `scheme` | URL scheme (http/https) |
| `webserver` | Server header value (for fingerprinting) |
| `content_type` | Content-Type header value |
| `method` | HTTP method used (always GET) |
| `host` | Hostname from URL |
| `path` | URL path |
| `time` | Response time duration |
| `chain_status_codes` | Array of status codes through redirect chain |
| `chain_hosts` | Array of hostnames through redirect chain |
| `words` | Word count in response body |
| `lines` | Line count in response body |
| `status_code` | Final HTTP status code |
| `content_length` | Response body size in bytes |
| `tls_version` | TLS version used (e.g., "1.3", "1.2") - HTTPS only |
| `cipher_suite` | Cipher suite name - HTTPS only |
| `protocol` | HTTP protocol (HTTP/1.1, HTTP/2, HTTP/3) - HTTPS only |
| `tls_config_strategy` | Which TLS strategy succeeded - HTTPS only |
| `error` | Error message (only present if request failed) |

**Note:** Failed requests are not included in the JSON output by default. Errors are logged to stderr.

## Input Format

- One URL per line
- Lines starting with `#` are treated as comments and ignored
- Empty lines are skipped
- URLs are automatically deduplicated

### Default Behavior

- **Hostname only** (e.g., `example.com`): Tests both HTTP and HTTPS on standard ports (80 and 443)
- **Hostname with port** (e.g., `example.com:8080`): Tests both HTTP and HTTPS on the specified port
- **URL with scheme** (e.g., `https://example.com`): Tests only the specified scheme on the standard port
- Use `--all-schemes` to override explicit schemes and test both HTTP and HTTPS

Example input file (`urls.txt`):
```
# Production servers
https://example.com       # Tests: https://example.com:443
https://api.example.com   # Tests: https://api.example.com:443

# Development servers (test both schemes)
dev.example.com           # Tests: http://dev.example.com:80, https://dev.example.com:443

# Custom port
staging.example.com:3000  # Tests: http://staging.example.com:3000, https://staging.example.com:3000

# IPv4 addresses work too
192.168.1.100             # Tests: http://192.168.1.100:80, https://192.168.1.100:443
```

## Multi-Scheme and Multi-Port Probing

### Overview

probeHTTP supports testing multiple schemes (HTTP/HTTPS) and ports from a single input URL, making it efficient for comprehensive web server reconnaissance.

### Scheme Behavior

| Input | Default Behavior | With `--all-schemes` |
|-------|------------------|---------------------|
| `example.com` | Both HTTP and HTTPS | Both HTTP and HTTPS |
| `http://example.com` | HTTP only | Both HTTP and HTTPS |
| `https://example.com` | HTTPS only | Both HTTP and HTTPS |

### Port Behavior

| Input | Default | With `--ignore-ports` | With `--ports "80,443,8080"` |
|-------|---------|---------------------|----------------------------|
| `example.com` | HTTP:80, HTTPS:443 | All common ports | Specified ports |
| `example.com:8080` | HTTP:8080, HTTPS:8080 | All common ports | Specified ports |
| `https://example.com:443` | HTTPS:443 | All common HTTPS ports | Specified ports |

### Common Ports

**Default HTTP ports:** 80, 8000, 8080, 8888  
**Default HTTPS ports:** 443, 8443, 10443, 8444

These ports are used when `--ignore-ports` flag is set.

### Port Range Syntax

The `--ports` flag supports:
- **Single ports:** `--ports "80,443,8080"`
- **Port ranges:** `--ports "8000-8010"`
- **Mixed:** `--ports "80,443,8000-8010,9000"`

### Flag Priority

1. `--ports` (highest priority) - Overrides all port logic
2. `--ignore-ports` - Uses common ports for each scheme
3. Input port - Uses the port specified in input
4. Default - Uses standard port for each scheme (80 for HTTP, 443 for HTTPS)

### Examples of URL Expansion

```
Input: example.com
Output: http://example.com:80, https://example.com:443

Input: https://example.com (with --all-schemes)
Output: http://example.com:80, https://example.com:443

Input: example.com (with --ignore-ports)
Output: 8 URLs (4 HTTP ports + 4 HTTPS ports)

Input: example.com (with --ports "80,443,8080-8082")
Output: 10 URLs (2 schemes × 5 ports)

Input: https://example.com (with --all-schemes --ignore-ports)
Output: 8 URLs (2 schemes × 4 common ports each)
```

### URL Deduplication

probeHTTP automatically deduplicates URLs that resolve to the same endpoint:
- `http://example.com` and `http://example.com:80` → Only one request
- `https://example.com` and `https://example.com:443` → Only one request

This prevents unnecessary duplicate requests and improves performance.

## Parallel TLS and Protocol Attempts

probeHTTP automatically tries multiple TLS configurations and HTTP protocols **in parallel** for HTTPS URLs to maximize compatibility, speed, and success rate.

### How It Works

For HTTPS URLs, probeHTTP uses a sophisticated batched approach:

**Batch 1 (Modern - 3 parallel attempts):**
1. **TLS 1.3** with **HTTP/3** (QUIC) - Fastest modern protocol
2. **TLS 1.2 Secure** with **HTTP/2** - Strong cipher suites only
3. **TLS 1.2 Compatible** with **HTTP/1.1** - Broader cipher suite support

If all Batch 1 attempts fail, fallback to:

**Batch 2 (Legacy - 2 parallel attempts):**
4. **TLS 1.1** with HTTP/1.1 - Deprecated but still used
5. **TLS 1.0** with HTTP/1.1 - Legacy support

### Strategy Details

| Strategy | TLS Version | Protocol | Cipher Suites | Use Case |
|----------|-------------|----------|---------------|----------|
| TLS 1.3 Modern | 1.3 | HTTP/3 | Modern (AES-GCM, ChaCha20) | Modern servers |
| TLS 1.2 Secure | 1.2 | HTTP/2 | Strong only (ECDHE, AES-GCM) | Secure servers |
| TLS 1.2 Compatible | 1.2 | HTTP/1.1 | Broader set | Legacy compatibility |
| TLS 1.1 Legacy | 1.1 | HTTP/1.1 | Wider range | Very old servers |
| TLS 1.0 Legacy | 1.0 | HTTP/1.1 | All available | Ancient servers |

### Advantages

- ⚡ **Speed**: First successful response wins and cancels remaining attempts
- 🎯 **Compatibility**: Automatically finds the best TLS/protocol combination
- 🔒 **Security**: Prefers modern protocols but falls back when needed
- 📊 **Visibility**: Reports which TLS version and protocol was actually used

### Output Fields

The JSON output includes TLS metadata for HTTPS requests:
- `tls_version`: TLS version used (e.g., "1.3", "1.2", "1.1", "1.0")
- `cipher_suite`: Cipher suite name (e.g., "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384")
- `protocol`: HTTP protocol used ("HTTP/1.1", "HTTP/2", "HTTP/3")
- `tls_config_strategy`: Which TLS strategy succeeded (e.g., "TLS 1.2 Secure")

### Security Considerations

⚠️ **Warning:** TLS 1.0 and 1.1 are deprecated and vulnerable to attacks (POODLE, BEAST). Legacy ciphers (3DES) are weak. These are only used for:
- Security testing and vulnerability assessment
- Discovering server capabilities
- Legacy system compatibility testing

**Not** recommended for secure communication in production environments.

### HTTP/3 (QUIC) Support

HTTP/3 uses UDP instead of TCP and provides:
- Better performance on high-latency networks
- Built-in encryption (always uses TLS 1.3)
- Faster connection establishment (0-RTT)
- Better handling of packet loss

**Note:**
- Requires UDP connectivity (port 443 UDP, may be blocked by firewalls)
- Fewer servers support HTTP/3 compared to HTTP/2
- Automatically falls back to HTTP/2 or HTTP/1.1 if HTTP/3 fails

#### UDP Buffer Size Warning

If you see a warning about UDP buffer sizes:
```
failed to sufficiently increase receive buffer size (was: 208 kiB, wanted: 7168 kiB, got: 416 kiB)
```

This is a system-level limitation that can affect HTTP/3 performance. To fix:

**Linux:**
```bash
# Increase UDP buffer size (requires root)
sudo sysctl -w net.core.rmem_max=8388608
sudo sysctl -w net.core.rmem_default=8388608

# Make permanent
echo "net.core.rmem_max=8388608" | sudo tee -a /etc/sysctl.conf
echo "net.core.rmem_default=8388608" | sudo tee -a /etc/sysctl.conf
```

**macOS:**
```bash
# Increase UDP buffer size (requires root)
sudo sysctl -w kern.ipc.maxsockbuf=8388608

# Make permanent
echo "kern.ipc.maxsockbuf=8388608" | sudo tee -a /etc/sysctl.conf
```

**Disable HTTP/3:**
If you don't want to configure UDP buffers, disable HTTP/3:
```bash
./probeHTTP --disable-http3 -i urls.txt
```

### Configuring TLS Timeout

Adjust the timeout for TLS handshake attempts:
```bash
# Default: 10 seconds per TLS attempt
./probeHTTP -i urls.txt --tls-timeout 15

# For slow networks
./probeHTTP -i urls.txt --tls-timeout 30
```

### Viewing TLS Metadata

Extract TLS information from results:
```bash
# View TLS version and protocol
./probeHTTP -i urls.txt | jq '{url: .url, tls: .tls_version, protocol: .protocol}'

# Find servers using TLS 1.3
./probeHTTP -i urls.txt | jq 'select(.tls_version == "1.3")'

# Find HTTP/3 servers
./probeHTTP -i urls.txt | jq 'select(.protocol == "HTTP/3")'
```

### Debug Logging

Enable detailed debug logging to see TLS strategy attempts:
```bash
./probeHTTP -i urls.txt --debug-log debug.log

# Debug log shows:
# - Which TLS strategies are attempted
# - Which protocols (HTTP/3, HTTP/2, HTTP/1.1) are tried
# - Detailed error messages for failed attempts
# - TLS connection details when successful
```

## Error Handling

- Connection errors, timeouts, and invalid URLs are handled gracefully
- By default, errors are written to stderr in JSON format (structured logging)
- Use `--silent` flag to suppress info logs (only shows errors)
- Failed requests are **not** included in JSON output
- Use `--debug-log` for detailed debugging information

### Retry Mechanism

probeHTTP supports automatic retries with exponential backoff:
```bash
# Retry failed requests up to 3 times
./probeHTTP -i urls.txt --retries 3

# Backoff schedule: 1s, 2s, 4s, 8s, 16s, 30s (max)
```

Retries are only triggered for network errors, not HTTP errors (4xx/5xx status codes).

## Rate Limiting

probeHTTP implements per-host rate limiting to prevent overwhelming target servers:
- Default: 10 requests per second per hostname
- Implemented using token bucket algorithm
- Helps avoid triggering rate limit protections on target servers

```bash
# Example: Scanning 100 URLs from same host will take ~10 seconds minimum
echo "example.com" | ./probeHTTP --ports "8000-8100"
```

## Use Cases

- 🔍 **Security reconnaissance** and vulnerability assessment
- 🖥️ **Web server fingerprinting** (TLS versions, protocols, server headers)
- 🔐 **TLS/SSL configuration auditing** (find weak ciphers, old protocols)
- 🔄 **Content change detection** via MMH3 hashing
- ✅ **Batch URL availability checking** (uptime monitoring)
- ⏱️ **Performance monitoring** via response times
- 🔗 **Integration** with other security tools and workflows
- 🌐 **HTTP/3 compatibility testing**
- 📊 **Multi-port service discovery**

## Performance

### Optimizations

- **Connection pooling**: Reuses TCP connections (40% improvement)
- **Parallel TLS attempts**: Reduces latency for HTTPS requests
- **Pre-compiled regexes**: 90% faster title extraction
- **Efficient hashing**: Uses fast MMH3 algorithm
- **Context cancellation**: Immediate shutdown on Ctrl+C
- **Worker pool**: Controlled concurrency prevents resource exhaustion

### Benchmarks

Run benchmarks with:
```bash
make bench
```

See `test/benchmark_test.go` for detailed performance tests.

## Development

### Project Structure

```
probeHTTP/
├── cmd/probehttp/          # Main application entry point
│   └── main.go
├── internal/
│   ├── config/             # Configuration and flag parsing
│   ├── hash/               # MMH3 hashing implementation
│   ├── output/             # Result structures
│   ├── parser/             # URL, port, and HTML parsing
│   │   ├── url.go          # URL expansion and validation
│   │   ├── port.go         # Port range parsing
│   │   └── html.go         # Title extraction
│   └── probe/              # Core probing logic
│       ├── prober.go       # Main probing functions
│       ├── client.go       # HTTP client creation
│       ├── redirect.go     # Redirect handling
│       └── worker.go       # Worker pool
├── pkg/
│   └── useragent/          # User-Agent pool
├── test/                   # Test files
│   ├── integration_test.go # Integration tests
│   ├── benchmark_test.go   # Performance benchmarks
│   ├── fuzz_test.go        # Fuzzing tests
│   ├── tls_parallel_test.go# TLS strategy tests
│   └── ...
├── docs/                   # Documentation
│   ├── IMPLEMENTATION_SUMMARY.md
│   ├── AUDIT_REPORT.md
│   └── ...
├── Makefile                # Build automation
└── README.md               # This file
```

### Building

```bash
# Build
make build

# Build for all platforms
make build-all

# Run tests
make test

# Run tests with coverage
make coverage

# Run benchmarks
make bench

# Run linter
make lint

# Run security checks
make security

# Run all checks
make check
```

### Testing

The project includes comprehensive tests:
- **Unit tests**: Test individual functions
- **Integration tests**: Test end-to-end workflows
- **Fuzz tests**: Find edge cases with random inputs
- **Benchmark tests**: Measure performance
- **TLS tests**: Verify parallel TLS strategy logic

```bash
# Run all tests
make test

# Run specific test
go test -v -run TestProbeURL_Success ./test

# Run fuzz tests (60 seconds each)
make fuzz

# Generate coverage report
make coverage
```

### Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run `make check` to ensure quality
6. Submit a pull request

## Security

### Best Practices

- Use `--allow-private` only in controlled environments
- Use `--insecure` only for testing purposes
- Review TLS versions and cipher suites in output
- Monitor rate limiting to avoid overwhelming targets
- Use `--retries` carefully to avoid excessive load

### Vulnerability Scanning

```bash
# Run security checks
make security

# Includes:
# - govulncheck: Go vulnerability scanning
# - gosec: Security-focused static analysis
```

## License

[Add your license here]

## Authors

- [Add authors here]

## Acknowledgments

- Built with Go 1.24
- Uses QUIC Go library for HTTP/3 support
- MMH3 hashing by twmb/murmur3
- HTML parsing by golang.org/x/net

## Changelog

See [GitHub Releases](https://github.com/secinto/probeHTTP/releases) for version history.

---

**Questions or issues?** Please open an issue on GitHub.
