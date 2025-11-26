# Testing Documentation

## Test Suite Overview

The probeHTTP project includes a comprehensive test suite with **35 test functions** covering unit tests, integration tests, and end-to-end system tests.

## Test Coverage

**Overall Coverage: 77.9%**

### Coverage by Function:
- `init()`: 100.0%
- `readURLs()`: 100.0%
- `processURLs()`: 100.0%
- `worker()`: 100.0%
- `createHTTPClient()`: 100.0%
- `calculateMMH3()`: 100.0%
- `calculateHeaderMMH3()`: 100.0%
- `countWordsAndLines()`: 100.0%
- `extractTitle()`: 93.8%
- `probeURL()`: 85.7%
- `main()`: 0.0% (expected - entry point)

## Test Files

### 1. helpers_test.go
Contains test utility functions and helpers:
- `resetConfig()`: Resets global config to defaults
- `createTestServer()`: Creates HTTP test servers
- `simpleHTMLHandler()`: Returns basic HTML
- `redirectHandler()`: Simulates redirects
- `statusCodeHandler()`: Returns specific HTTP status codes
- `multiRedirectHandler()`: Creates redirect chains
- Assert helpers: `assertStringEqual()`, `assertIntEqual()`, `assertSliceEqual()`, etc.

### 2. main_test.go - Unit Tests
Tests all pure functions in isolation:

#### TestReadURLs (8 subtests)
- Single URL
- Multiple URLs
- URLs with comments
- URLs with empty lines
- URLs with whitespace
- Empty input
- Only comments and empty lines
- Mixed content

#### TestCalculateMMH3 (3 subtests)
- Empty data
- Simple text with known hash value
- Hash consistency

#### TestCalculateHeaderMMH3 (5 subtests)
- Empty headers
- Single header
- Multiple headers
- Headers with multiple values
- Order independence

#### TestExtractTitle (9 subtests)
- Simple title
- Title with whitespace
- No title
- Empty title
- Nested structure
- Malformed HTML
- Special characters
- Empty string
- Non-HTML content

#### TestCountWordsAndLines (10 subtests)
- Empty string
- Single word
- Multiple words
- Multiple lines
- Lines with varying words
- Extra whitespace
- Tabs
- Newlines only
- HTML content
- Mixed content with punctuation

#### TestCreateHTTPClient (4 subtests)
- Default settings
- No redirect following
- Custom max redirects
- Custom timeout

#### TestProcessURLs (4 subtests)
- Empty URL list
- Single URL
- Multiple URLs with single worker
- Multiple URLs with multiple workers

#### TestProcessURLs_Concurrency (4 subtests)
- Concurrency level 1
- Concurrency level 2
- Concurrency level 3
- Concurrency level 5

### 3. integration_test.go - Integration Tests
Tests with mock HTTP servers:

#### TestProbeURL_Success
- Verifies successful HTTP requests
- Checks all output fields
- Validates hashes, metadata extraction

#### TestProbeURL_StatusCodes (6 subtests)
- 200 OK
- 201 Created
- 204 No Content
- 400 Bad Request
- 404 Not Found
- 500 Internal Server Error

#### TestProbeURL_Redirects
- Tests redirect following
- Verifies final URL tracking

#### TestProbeURL_NoFollowRedirects
- Tests redirect behavior when disabled
- Verifies staying at redirect URL

#### TestProbeURL_MaxRedirects
- Tests redirect limit enforcement
- Verifies error handling

#### TestProbeURL_Timeout
- Tests request timeout
- Verifies timeout error messages

#### TestProbeURL_InvalidURL (3 subtests)
- Invalid scheme
- No host
- Malformed URL

#### TestProbeURL_URLWithoutScheme
- Tests default scheme (http://) addition

#### TestProbeURL_ContentAnalysis
- Tests title extraction
- Tests word/line counting
- Tests hash calculation
- Tests server header extraction

#### TestProbeURL_HashConsistency
- Verifies identical hashes for same content

#### TestProbeURL_PortExtraction
- Tests port extraction from URLs
- Tests default ports (80/443)

#### TestProbeURL_PathExtraction (3 subtests)
- Root path
- Simple path
- Nested path

#### TestProbeURL_HostExtraction
- Tests host extraction from URLs

#### TestWorker
- Tests worker function
- Tests concurrent processing

### 4. system_test.go - End-to-End Tests
Full program integration tests:

#### TestEndToEnd_StdinStdout
- Tests reading from stdin
- Tests writing to stdout
- Validates JSON output

#### TestEndToEnd_FileIO
- Tests file input
- Tests file output
- Validates file contents

#### TestEndToEnd_MultipleURLs
- Tests concurrent processing of multiple URLs
- Validates different status codes

#### TestEndToEnd_CommentsAndEmptyLines
- Tests input filtering
- Validates comment and blank line handling

#### TestEndToEnd_InvalidURLs
- Tests error handling for invalid URLs

#### TestEndToEnd_JSONOutput
- Validates JSON marshaling/unmarshaling
- Tests output format compliance

#### TestEndToEnd_ConcurrentProcessing (4 subtests)
- Concurrency 1, 5, 10, 20
- Tests scalability

#### TestEndToEnd_WithTestDataFile
- Tests with fixture data
- Validates real-world input

#### TestBinary_Help
- Tests compiled binary help output
- Validates all CLI flags are documented

#### TestBinary_BasicExecution
- Tests binary execution
- Validates JSON output from binary

#### TestBinary_FileInputOutput
- Tests binary with file I/O
- Validates end-to-end workflow

## Test Data Fixtures

### testdata/urls.txt
Sample URLs with comments for testing:
- Valid HTTP/HTTPS URLs
- URLs without scheme
- Comments and empty lines

### testdata/malformed.txt
Invalid URLs for error testing:
- Invalid schemes
- Missing components
- Malformed syntax

### testdata/test.html
Sample HTML for title extraction testing

## Running Tests

### Run all tests:
```bash
go test -v
```

### Run with coverage:
```bash
go test -v -cover
```

### Generate coverage report:
```bash
go test -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Run specific test:
```bash
go test -v -run TestProbeURL_Success
```

### Run tests with timeout:
```bash
go test -v -timeout 60s
```

## Test Execution Time

Total test execution time: ~6-7 seconds

Breakdown:
- Unit tests: <1 second
- Integration tests (with timeouts): ~4-5 seconds
- System tests: ~1-2 seconds

## Test Categories

1. **Unit Tests (35+ assertions)**: Test individual functions in isolation
2. **Integration Tests (15+ scenarios)**: Test with mock HTTP servers
3. **System Tests (11 scenarios)**: End-to-end testing including binary execution

## Continuous Integration

Tests can be integrated into CI/CD pipelines:

```yaml
# Example GitHub Actions
- name: Run tests
  run: go test -v -cover ./...
```

## Test Best Practices

1. **Isolation**: Each test is independent and can run in any order
2. **Config Reset**: Uses `resetConfig()` to ensure clean state
3. **Mock Servers**: Uses httptest for reliable integration tests
4. **Timeout Protection**: All tests have timeout protection
5. **Coverage Tracking**: Maintains >75% code coverage
6. **Error Testing**: Comprehensive error path testing
7. **Concurrency Testing**: Tests with various concurrency levels

## Known Test Limitations

- `main()` function (0% coverage): Entry point, tested via binary execution
- Some error branches in `probeURL()` and `extractTitle()`: Edge cases in production scenarios
- Real network requests: Only tested via binary tests, not in unit/integration tests
