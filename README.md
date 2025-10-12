# probeHTTP

A comprehensive HTTP probing tool written in Go that performs HTTP requests with metadata extraction, hashing, and content analysis.

## Features

- **Multi-scheme and multi-port probing** - Test HTTP and HTTPS on multiple ports
- HTTP/HTTPS probing with configurable redirect handling
- MMH3 hash calculation for response body and headers
- HTML title extraction
- Word and line counting
- Web server and content-type detection
- Concurrent request processing with worker pool
- JSON output format
- Flexible input/output (stdin/stdout or files)
- Port range support (e.g., `8000-8010`)

## Installation

```bash
go build -o probeHTTP main.go
```

## Usage

### Basic Usage

```bash
# Probe from stdin
echo "https://example.com" | ./probeHTTP

# Probe from file
./probeHTTP -i urls.txt

# Probe multiple URLs
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
| `--concurrency` | `-c` | Number of concurrent requests | 10 |
| `--silent` | | Silent mode (no errors to stderr) | false |
| `--all-schemes` | `-as` | Test both HTTP and HTTPS (overrides input scheme) | false |
| `--ignore-ports` | `-ip` | Ignore input ports and test common HTTP/HTTPS ports | false |
| `--ports` | `-p` | Custom port list (comma-separated, supports ranges) | - |

### Examples

#### Basic Examples

```bash
# Probe with custom timeout and concurrency
echo "https://example.com" | ./probeHTTP -t 10 -c 5

# Don't follow redirects
echo "https://google.com" | ./probeHTTP -fr=false

# Save output to file
./probeHTTP -i urls.txt -o results.json

# Silent mode (suppress errors)
./probeHTTP -i urls.txt -silent
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

## Output Format

The tool outputs JSON for each probed URL:

```json
{
  "timestamp": "2025-10-10T13:06:57+02:00",
  "hash": {
    "body_mmh3": "3570969655",
    "header_mmh3": "3370267568"
  },
  "port": "443",
  "url": "https://example.com",
  "input": "https://example.com",
  "title": "Example Domain",
  "scheme": "https",
  "webserver": "",
  "content_type": "text/html",
  "method": "GET",
  "host": "example.com",
  "path": "/",
  "time": "470.210792ms",
  "words": 25,
  "lines": 2,
  "status_code": 200,
  "content_length": 513
}
```

### Output Fields

- `timestamp`: RFC3339 formatted timestamp
- `hash.body_mmh3`: MMH3 hash of response body
- `hash.header_mmh3`: MMH3 hash of concatenated headers
- `port`: Port number used for the request
- `url`: Final URL after redirects
- `input`: Original input URL
- `title`: HTML page title
- `scheme`: URL scheme (http/https)
- `webserver`: Server header value
- `content_type`: Content-Type header value
- `method`: HTTP method used (always GET)
- `host`: Hostname from URL
- `path`: URL path
- `time`: Response time duration
- `words`: Word count in response body
- `lines`: Line count in response body
- `status_code`: HTTP status code
- `content_length`: Response body size in bytes
- `error`: Error message (only present if request failed)

## Input Format

- One URL per line
- Lines starting with `#` are treated as comments and ignored
- Empty lines are skipped

### Default Behavior

- **Hostname only** (e.g., `example.com`): Tests both HTTP and HTTPS on standard ports (80 and 443)
- **Hostname with port** (e.g., `example.com:8080`): Tests both HTTP and HTTPS on the specified port
- **URL with scheme** (e.g., `https://example.com`): Tests only the specified scheme on the standard port
- Use `--all-schemes` to override explicit schemes and test both HTTP and HTTPS

Example input file:
```
# Production servers
https://example.com       # Tests: https://example.com:443
https://api.example.com   # Tests: https://api.example.com:443

# Development servers (test both schemes)
dev.example.com           # Tests: http://dev.example.com:80, https://dev.example.com:443

# Custom port
staging.example.com:3000  # Tests: http://staging.example.com:3000, https://staging.example.com:3000
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
Output: 8 URLs (4 HTTP ports × 1 + 4 HTTPS ports × 1)

Input: example.com (with --ports "80,443,8080-8082")
Output: 10 URLs (2 schemes × 5 ports)

Input: https://example.com (with --all-schemes --ignore-ports)
Output: 8 URLs (2 schemes × 4 common ports each)
```

## Dependencies

- `github.com/twmb/murmur3` - MMH3 hashing
- `golang.org/x/net/html` - HTML parsing

## Error Handling

- Connection errors, timeouts, and invalid URLs are handled gracefully
- By default, errors are written to stderr
- Use `-silent` flag to suppress error messages
- Failed requests include an `error` field in the JSON output

## Use Cases

- Security reconnaissance and vulnerability assessment
- Web server fingerprinting
- Content change detection via hashing
- Batch URL availability checking
- Performance monitoring via response times
- Integration with other security tools and workflows
