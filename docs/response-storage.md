# Plan: Align probeHTTP Response Storage with HTTPx

## Context

probeHTTP is intended as a replacement for projectdiscovery/httpx, but its response storage format diverges from HTTPx in several ways. The `hall.ag` project used HTTPx and produced response files at `~/checkfix/projects/hall.ag/responses/`. We need probeHTTP's `-sr`/`-srd` output to be structurally identical to HTTPx so downstream tools (and checkfix) can consume either tool's output interchangeably.

## Differences Found

| Aspect | HTTPx (native) | probeHTTP (current) | Action |
|--------|----------------|---------------------|--------|
| **Directory layout** | `{srd}/{domain_port}/{hash}.txt` | `{srd}/response/{host}/{hash}.txt` | Remove extra `response/` subdir |
| **Index file** | `{srd}/index.txt` -- one line per response | None | Add index.txt generation |
| **Hash input** | `SHA1(URL)` | `SHA1(METHOD:URL)` | Change to hash URL only |
| **File content** | Raw HTTP: req+resp per hop, final URL on last line | Custom `=== SECTION ===` format | Rewrite to raw HTTP format |
| **Redirect chain** | Full request-response pairs per hop | Summary: `[1] host (status: 301)` | Store full req/resp per hop |
| **Final URL** | Bare URL on the last line | Inside `=== FINAL URL ===` section | Append bare URL at end |

## Files to Modify

| File | Role |
|------|------|
| `internal/storage/storage.go` | Core: format, hash, path, index |
| `internal/probe/redirect.go` | Capture raw req/resp per redirect hop |
| `internal/probe/prober.go` | Wire chain entries to storage, call index |
| `cmd/probehttp/main.go` | Remove `response/` subdir, init index mutex |

### Existing helpers to reuse
- `formatRawRequest(req)` at `prober.go:1075` -- formats raw HTTP request line + headers
- `formatRawResponse(resp)` at `prober.go:1109` -- formats raw HTTP status line + headers
- `SanitizeHost(host)` at `storage.go:23` -- already matches HTTPx convention

---

## Step 1: Add `ChainEntry` type and update `FormatStoredResponse`

**File:** `internal/storage/storage.go`

Add a struct to carry per-hop data:
```go
type ChainEntry struct {
    RawRequest  string // formatted by formatRawRequest
    RawResponse string // status line + headers via formatRawResponse
    Body        []byte // response body for this hop
}
```

Replace `FormatStoredResponse` signature and body:
```go
func FormatStoredResponse(chain []ChainEntry, finalURL string) []byte
```

Output format (matching HTTPx file content exactly):
```
GET / HTTP/1.1                     <-- hop 1 request
Host: example.com
User-Agent: ...
Accept-Charset: utf-8
Accept-Encoding: gzip
                                   <-- blank line (end of request headers)
                                   <-- blank line (separator)
HTTP/1.1 302 Found                 <-- hop 1 response
Location: https://example.com/
Content-Length: 5
                                   <-- blank line (end of response headers)
[body bytes if any]
                                   <-- blank line (separator before next hop)
GET / HTTP/1.1                     <-- hop 2 request (after redirect)
Host: example.com:443
...
                                   <-- blank line
HTTP/1.1 200 OK                    <-- hop 2 response
Content-Type: text/html
...

[final body]

https://example.com                <-- final URL, last line
```

## Step 2: Update hashing and path construction

**File:** `internal/storage/storage.go`

**`GenerateFilename`** -- drop `method` parameter:
```go
// Before: GenerateFilename(method, urlStr) -> SHA1("GET:https://...")
// After:  GenerateFilename(urlStr)         -> SHA1("https://...")
func GenerateFilename(urlStr string) string {
    hash := sha1.Sum([]byte(urlStr))
    return hex.EncodeToString(hash[:])
}
```

**`BuildStoragePath`** -- remove `response/` segment:
```go
// Before: filepath.Join(baseDir, "response", sanitizedHost, filename+".txt")
// After:  filepath.Join(baseDir, sanitizedHost, filename+".txt")
```

**`StoreResponse`** -- drop `method` parameter, update internal call:
```go
func StoreResponse(baseDir string, parsedURL *url.URL, data []byte) (string, error) {
    filename := GenerateFilename(parsedURL.String())
    storagePath := BuildStoragePath(baseDir, parsedURL.Host, filename)
    // ... rest unchanged
}
```

## Step 3: Add `index.txt` generation

**File:** `internal/storage/storage.go`

Add a concurrency-safe index writer. Since probeHTTP runs up to N concurrent goroutines writing responses, the index file needs protection:

```go
var indexMu sync.Mutex

func AppendToIndex(baseDir, storagePath, urlStr string, statusCode int, statusText string) error {
    indexMu.Lock()
    defer indexMu.Unlock()

    indexPath := filepath.Join(baseDir, "index.txt")
    // Make storagePath relative to baseDir for the index entry
    relPath, _ := filepath.Rel(baseDir, storagePath)
    line := fmt.Sprintf("%s %s (%d %s)\n", relPath, urlStr, statusCode, statusText)

    f, err := os.OpenFile(indexPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil { return err }
    defer f.Close()
    _, err = f.WriteString(line)
    return err
}
```

Index line format (matching HTTPx): `{relative_path} {url} ({statusCode} {statusText})`

Example: `www.hall.ag_80/85c766da4db303ada59a00430c4e12a2ea1bb9c4.txt http://www.hall.ag:80 (200 OK)`

## Step 4: Capture full redirect chain in redirect-following functions

**File:** `internal/probe/redirect.go`

Modify `followRedirects()` return signature to also return `[]storage.ChainEntry`:
```go
func (p *Prober) followRedirects(ctx context.Context, initialResp *http.Response,
    maxRedirects int, startStep int, initialHostname string, buf *strings.Builder,
) (*http.Response, []int, []string, []storage.ChainEntry, error)
```

Inside the redirect loop, at each hop:
1. **Before sending the redirect request**: call `formatRawRequest(req)` to capture the raw request
2. **After receiving the redirect response**: call `formatRawResponse(nextResp)` to capture response headers, then read the body
3. **Append** a `ChainEntry{RawRequest, RawResponse, Body}` to the chain slice

The body at each hop needs to be read and stored. Currently the body is only read in debug mode (redirect.go:104-112). Change this to always read the body when `StoreResponse` is enabled. The body is already limited by `MaxBodySize`.

**File:** `internal/probe/prober.go`

Apply identical changes to `followRedirectsWithClient()` (prober.go:877).

## Step 5: Wire chain entries into storage calls in prober

**File:** `internal/probe/prober.go`

In `probeURLHTTP()` (~line 383) and `probeURLWithConfig()` (~line 834), update the storage block:

**Before (current):**
```go
if p.config.StoreResponse {
    rawResponseHeaders := formatRawResponse(finalResp)
    redirectChain := make([]string, len(hostChain))
    for i, host := range hostChain {
        redirectChain[i] = fmt.Sprintf("%s (status: %d)", host, statusChain[i])
    }
    storedData := storage.FormatStoredResponse(rawRequest, rawResponseHeaders, redirectChain, initialBody, finalURL)
    storagePath, err := storage.StoreResponse(p.config.StoreResponseDir, finalParsedURL, "GET", storedData)
    ...
}
```

**After:**
```go
if p.config.StoreResponse {
    // Build initial hop entry (the first request/response before any redirects)
    initialEntry := storage.ChainEntry{
        RawRequest:  rawRequest,
        RawResponse: formatRawResponse(resp),  // original response, not finalResp
        Body:        initialResponseBody,       // body of first response (may be redirect body)
    }

    // Prepend initial hop to redirect chain entries
    fullChain := append([]storage.ChainEntry{initialEntry}, chainEntries...)
    // If final response differs from last chain entry, add it too
    // (handle case where no redirects occurred)

    storedData := storage.FormatStoredResponse(fullChain, finalURL)
    storagePath, err := storage.StoreResponse(p.config.StoreResponseDir, finalParsedURL, storedData)
    if err != nil {
        p.config.Logger.Warn("failed to store response", "url", probeURL, "error", err)
    } else {
        result.StoredResponsePath = storagePath
        // Append to index
        storage.AppendToIndex(p.config.StoreResponseDir, storagePath, probeURL,
            result.StatusCode, http.StatusText(result.StatusCode))
    }
}
```

Note: The initial request's body needs to be preserved separately for the chain entry. Currently `initialBody` is overwritten when redirects are followed (prober.go:317). We need to save it before the redirect follow or capture it as part of the initial `ChainEntry`.

## Step 6: Update `main.go`

**File:** `cmd/probehttp/main.go`

Remove the `response/` subdirectory from initialization:

```go
// Before (line 53):
responseDir := filepath.Join(cfg.StoreResponseDir, "response")

// After:
responseDir := cfg.StoreResponseDir
```

## Step 7: Update tests

Add unit tests in a new `internal/storage/storage_test.go`:

1. `TestGenerateFilename` -- verify SHA1(URL) produces expected hash
2. `TestBuildStoragePath` -- verify `{srd}/{host_port}/{hash}.txt` (no `response/`)
3. `TestFormatStoredResponse` -- verify raw HTTP format output matches HTTPx
4. `TestAppendToIndex` -- verify index line format and concurrent safety
5. `TestStoreResponse` -- verify file written to correct path

---

## Verification

1. **Build**: `go build ./...` -- ensure compilation passes
2. **Unit tests**: `go test ./internal/storage/... -v`
3. **Manual test**: Run probeHTTP with `-sr -srd /tmp/probe-test` against a URL that redirects (e.g., http://example.com which 302s to https), then:
   - Verify directory structure: `find /tmp/probe-test -type f`
   - Verify no `response/` subdirectory exists
   - Verify `index.txt` exists with correct format
   - Verify `.txt` file content matches raw HTTP format
4. **Compatibility check**: Diff file format against `~/checkfix/projects/hall.ag/responses/domains/response/*/` to confirm structural parity
