# probeHTTP - Critical Issues Summary

**Analysis Date:** November 26, 2024  
**Codebase Version:** 2.0

---

## 🚨 Critical Issues Requiring Immediate Attention

### 1. HTTP/3 Transport Resource Leak
- **File:** `internal/probe/client.go:119-131`
- **Issue:** HTTP/3 transport never closed, causing goroutine and connection leaks
- **Impact:** Memory leaks, file descriptor exhaustion
- **Fix:** Implement Close() method on Client and call defer prober.Close()

### 2. Goroutine Leak in Parallel TLS
- **File:** `internal/probe/prober.go:395-450`
- **Issue:** Goroutines may block on channel send after cancellation
- **Impact:** Goroutine accumulation, memory growth
- **Fix:** Use buffered channel and drain after success

### 3. Missing Body Close on Error Paths
- **File:** `internal/probe/prober.go:630-640`
- **Issue:** Response bodies not closed when redirect errors occur
- **Impact:** Connection leaks, file descriptor exhaustion
- **Fix:** Add defer finalResp.Body.Close() with nil checks

---

## ⚠️ High Priority Issues

### 4. Debug File Logger Never Closed
- **File:** `internal/config/config.go:119-126`
- **Issue:** Debug log file handle never closed
- **Impact:** File descriptor leak
- **Fix:** Add Close() method to Config

### 5. O(n²) URL Deduplication
- **File:** `cmd/probehttp/main.go:107-126`
- **Issue:** Inefficient nested loop for URL mapping
- **Impact:** Slow with large URL lists
- **Fix:** Pre-compute normalized map for O(n) lookup

### 6. Silent Error Swallowing
- **File:** Multiple locations
- **Issue:** `io.ReadAll` errors ignored with `_`
- **Impact:** Silent data loss, incomplete processing
- **Fix:** Check and log all errors

### 7. No Timeout for Rate Limiter
- **File:** `internal/probe/prober.go:350-359`
- **Issue:** Rate limiter Wait() can block indefinitely
- **Impact:** Hangs on rate-limited hosts
- **Fix:** Add context timeout for Wait()

### 8. HTTP Client Cleanup Missing
- **File:** `internal/probe/client.go`
- **Issue:** Multiple clients created but never cleaned up
- **Impact:** Resource accumulation
- **Fix:** Track and cleanup all created clients

---

## Quick Fix Checklist

```bash
# 1. Add Close methods
[ ] Client.Close() - close HTTP/3 transport
[ ] Config.Close() - close debug log file
[ ] Prober.Close() - cleanup all clients

# 2. Fix goroutine leaks
[ ] Make TLS batch results channel buffered
[ ] Drain channel after finding success
[ ] Add defer cleanup for all goroutines

# 3. Fix resource leaks
[ ] Close response bodies on all error paths
[ ] Add nil checks before closing
[ ] Use defer where appropriate

# 4. Fix error handling
[ ] Check all io.ReadAll errors
[ ] Add context timeouts
[ ] Use typed errors

# 5. Optimize performance
[ ] Fix O(n²) URL deduplication
[ ] Pre-compute normalized URL map
[ ] Add metrics for monitoring
```

---

## Testing Before Deployment

```bash
# 1. Race detector
go test -race ./...

# 2. Memory leak test
go test -run TestConcurrent -count=100
# Monitor: watch -n 1 'ps aux | grep probeHTTP'

# 3. Goroutine leak test
# Add to test:
before := runtime.NumGoroutine()
# ... run tests
after := runtime.NumGoroutine()
assert.Equal(t, before, after)

# 4. Load test
echo "example.com" | ./probeHTTP --ports "1-10000" --concurrency 100

# 5. Resource monitoring
# Run with: ulimit -n 256  # Limit file descriptors
# Should fail gracefully, not leak
```

---

## Deployment Safety

**Current Status:** ⚠️ **NOT PRODUCTION READY**

**Blockers:**
1. Resource leaks in HTTP/3 transport
2. Goroutine leaks in parallel TLS
3. Connection leaks on error paths

**After Critical Fixes:** ✅ **SAFE FOR PRODUCTION**

**Timeline:**
- Critical fixes: 2-3 days
- Testing: 1-2 days
- Production deployment: After critical fixes + testing

---

## Contact

For urgent issues or questions about these findings:
- See full analysis: `docs/ISSUES_AND_IMPROVEMENTS.md`
- Run static analysis: `make lint && make security`
- Profile memory: `go test -memprofile mem.prof`

---

**Last Updated:** November 26, 2024
