# Remediation Plan: Code Analysis Findings

**Date:** 2025-02-25  
**Source:** Source code analysis (functionality and performance)  
**Status:** Implemented 2025-02-25

---

## Overview

This plan addresses the valid findings from the code analysis. Items are ordered by priority (High → Medium → Low) and grouped by category.

---

## 1. Error Handling

### 1.1 Fix ignored `io.ReadAll` error in redirect body reading

**Location:** `internal/probe/redirect.go:118`  
**Severity:** Medium  
**Effort:** Low

**Current code:**
```go
nextBody, _ = io.ReadAll(io.LimitReader(bodyReader, p.config.MaxBodySize))
```

**Problem:** Read errors are silently ignored. Partial or failed reads can lead to incomplete chain entries and misleading debug output.

**Steps:**
1. Capture the error: `nextBody, err = io.ReadAll(...)`
2. If `err != nil`:
   - Log the error via `p.config.Logger.Warn` (or `Debug` if available)
   - Optionally return early with `fmt.Errorf("redirect body read failed: %w", err)` to abort the redirect chain
   - Or: use empty `nextBody` and continue (document that chain entry may be incomplete)
3. Add a test case for redirect body read failure (e.g., mock connection that returns error on read)

**Files to modify:**
- `internal/probe/redirect.go`
- `test/` (add or extend redirect test)

---

## 2. Documentation Accuracy

### 2.1 Align README with TLS implementation (sequential vs parallel)

**Location:** `README.md`, `docs/`  
**Severity:** Medium  
**Effort:** Low

**Problem:** README describes "parallel TLS attempts" and "Batch 1 (Modern - 3 parallel attempts)" but `probeURLWithParallelTLS` uses sequential fallback.

**Option A — Update docs (recommended, lower risk):**
1. In README "Parallel TLS and Protocol Attempts":
   - Change "in parallel" to "with automatic fallback" or "sequentially"
   - Update "Batch 1 (Modern - 3 parallel attempts)" to "Batch 1 (Modern — tries 3 strategies in order)"
   - Update "Batch 2 (Legacy - 2 parallel attempts)" similarly
2. In "How It Works", clarify that strategies are tried one-by-one until one succeeds
3. Update "Advantages" to say "First successful response stops further attempts" instead of implying parallelism

**Option B — Implement true parallel TLS:**
1. Launch Batch 1 strategies concurrently (e.g., `errgroup` or goroutines)
2. Use a buffered results channel sized to strategy count
3. Return first success; cancel remaining attempts via context
4. Drain channel on success to avoid goroutine leaks (see `docs/ISSUES_AND_IMPROVEMENTS.md` #2)
5. Add tests for parallel behavior and cancellation

**Files to modify (Option A):**
- `README.md`
- Any `docs/*.md` that mention parallel TLS

---

## 3. Performance Optimizations

### 3.1 Reduce header hash allocation

**Location:** `internal/hash/mmh3.go:25-54`  
**Severity:** Low  
**Effort:** Low

**Problem:** `CalculateHeaderMMH3` builds a string then converts to `[]byte`, causing an extra allocation.

**Steps:**
1. Replace `strings.Builder` with `bytes.Buffer`
2. Use `buf.WriteString()` for each header line
3. Call `CalculateMMH3(buf.Bytes())` — no intermediate string
4. Run `BenchmarkCalculateHeaderMMH3` before/after to confirm fewer allocations

**Files to modify:**
- `internal/hash/mmh3.go` (add `"bytes"` import)

---

### 3.2 Labeled break for context cancellation (sender loop)

**Location:** `internal/probe/worker.go:25-34`  
**Severity:** Low  
**Effort:** Low

**Problem:** On context cancellation, `break` exits only the `select`, not the `for` loop. With many URLs, the sender still iterates O(n) times.

**Steps:**
1. Add label before the `for` loop: `sendLoop:`
2. Change `break` to `break sendLoop` in the `ctx.Done()` case
3. Add a test that cancels context mid-scan and verifies quick shutdown (e.g., timing or goroutine count)

**Files to modify:**
- `internal/probe/worker.go`

---

### 3.3 Content-type check before HTML parsing

**Location:** `internal/parser/html.go`, `internal/probe/prober.go`  
**Severity:** Low  
**Effort:** Low

**Problem:** `ExtractTitle` parses every response as HTML, including JSON, images, etc., wasting CPU.

**Steps:**
1. Add optional `contentType` parameter to `ExtractTitle(body, contentType string)` (or a new `ExtractTitleFromHTML` that is only called when appropriate)
2. If `contentType` is non-empty and does not contain `"text/html"`, return `""` immediately
3. In `prober.go` (where `ExtractTitle` is called), pass `finalResp.Header.Get("Content-Type")`
4. Preserve backward compatibility: if `contentType == ""`, keep current behavior (parse anyway)
5. Add test for non-HTML content-type (e.g., `application/json`) returning empty title

**Files to modify:**
- `internal/parser/html.go`
- `internal/probe/prober.go`
- `internal/parser/html_test.go`
- `test/main_test.go` (if it tests ExtractTitle)

---

## 4. Configuration

### 4.1 Configurable rate limit

**Location:** `internal/probe/client.go:76-88`, `internal/config/config.go`  
**Severity:** Medium  
**Effort:** Low

**Problem:** Rate limit (10 req/s, burst 1) is hardcoded.

**Steps:**
1. Add to `Config`:
   - `RateLimitPerHost int` (default 10)
   - `RateLimitBurst int` (default 1)
2. Add CLI flags: `--rate-limit` and `--rate-burst`
3. In `client.go`, use `rate.NewLimiter(rate.Limit(cfg.RateLimitPerHost), cfg.RateLimitBurst)` instead of `rate.NewLimiter(10, 1)`

**Example usage:**
```bash
./probeHTTP -i urls.txt --rate-limit 5 --rate-burst 3
./probeHTTP -i urls.txt --rate-limit 20 --rate-burst 5  # higher throughput
```
4. Update `GetLimiter` to pass config values (or store config reference in Client)
5. Document in README
6. Add test for custom rate limit

**Files to modify:**
- `internal/config/config.go`
- `internal/config/flaggroup.go` (or equivalent flag registration)
- `internal/probe/client.go`
- `README.md`

---

## 5. Resource Management

### 5.1 Cache eviction for unbounded growth

**Location:** `internal/probe/prober.go` (cnameCache), `internal/probe/client.go` (limiters)  
**Severity:** Medium  
**Effort:** Medium

**Problem:** `cnameCache` and `limiters` grow without bound during large scans. (`clientCache` is bounded by ~5 strategy:protocol entries — no change needed.)

**Steps:**

**A. Rate limiter map (`client.go`):**
1. Option 1: Use `sync.Map` with periodic cleanup (e.g., every N requests or T seconds)
2. Option 2: Use an LRU cache with max size (e.g., 1000 hosts)
3. Option 2 is preferable for predictable memory; consider `github.com/hashicorp/golang-lru/v2` or similar

**B. CNAME cache (`prober.go`):**
1. `sync.Map` has no size limit; add eviction
2. Option 1: LRU with max entries (e.g., 5000)
3. Option 2: TTL-based eviction (e.g., entries older than 1 hour)
4. Option 3: Max size with simple eviction (e.g., clear cache when it exceeds 10k entries)

**C. Client cache (`prober.go`):**
1. Cache is keyed by `strategy:protocol`; size is bounded by number of strategies (~5)
2. **No eviction needed** — already bounded; skip this cache

**Files to modify:**
- `internal/probe/client.go` (limiters)
- `internal/probe/prober.go` (cnameCache)
- `go.mod` (if adding LRU dependency)
- Tests for cache eviction behavior

---

## 6. Implementation Order

| # | Item | Category | Priority | Dependencies |
|---|------|----------|----------|--------------|
| 1 | Fix ignored io.ReadAll error | Error handling | High | None |
| 2 | Configurable rate limit | Config | Medium | None |
| 3 | Align README with TLS (Option A) | Docs | Medium | None |
| 4 | Header hash allocation | Performance | Low | None |
| 5 | Labeled break for cancellation | Performance | Low | None |
| 6 | Content-type before HTML parse | Performance | Low | None |
| 7 | Cache eviction | Resource mgmt | Medium | None (or LRU dep) |

**Suggested phases:**
- **Phase 1 (quick wins):** 1, 4, 5, 6 — small, localized changes
- **Phase 2 (config/docs):** 2, 3
- **Phase 3 (resource mgmt):** 7 — may require new dependency and more design

---

## 7. Verification

After each fix:
1. Run `make test`
2. Run `make lint`
3. Run relevant benchmarks (`make bench`)
4. For cache eviction: add/run tests that simulate large scans

---

## 8. bd (beads) Integration

Per project conventions, create issues for each item:

```bash
bd create "Fix ignored io.ReadAll error in redirect.go" --description="..." -t bug -p 1
bd create "Add configurable rate limit (--rate-limit, --rate-burst)" --description="..." -t feature -p 2
bd create "Align README with sequential TLS implementation" --description="..." -t task -p 2
# ... etc.
```

Link related work with `--deps discovered-from:bd-XXX` where appropriate.
