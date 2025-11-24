package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"probeHTTP/internal/config"
)

// resetConfig resets the global config to default values for testing
func resetConfig() *config.Config {
	cfg := config.New()
	// Initialize logger for tests (use a no-op handler for silent mode)
	cfg.Logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in tests
	}))
	return cfg
}

// createTestServer creates a test HTTP server with custom handlers
func createTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// simpleHTMLHandler returns a simple HTML page
func simpleHTMLHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Server", "TestServer/1.0")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
    <title>Test Page</title>
</head>
<body>
    <h1>Hello World</h1>
    <p>This is a test page.</p>
</body>
</html>`)
}

// redirectHandler returns a redirect response
func redirectHandler(target string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusFound)
	}
}

// statusCodeHandler returns a response with a specific status code
func statusCodeHandler(code int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		fmt.Fprintf(w, "Status: %d", code)
	}
}

// multiRedirectHandler creates a chain of redirects
func multiRedirectHandler(count int, finalHandler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check if this is the final redirect
		redirectNum := 0
		if r.URL.Query().Has("redirect") {
			fmt.Sscanf(r.URL.Query().Get("redirect"), "%d", &redirectNum)
		}

		if redirectNum >= count {
			finalHandler(w, r)
			return
		}

		// Redirect to next step
		nextURL := fmt.Sprintf("%s?redirect=%d", r.URL.Path, redirectNum+1)
		http.Redirect(w, r, nextURL, http.StatusFound)
	}
}

// assertStringEqual checks if two strings are equal
func assertStringEqual(t *testing.T, got, want, msg string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %q, want %q", msg, got, want)
	}
}

// assertIntEqual checks if two integers are equal
func assertIntEqual(t *testing.T, got, want int, msg string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %d, want %d", msg, got, want)
	}
}

// assertNotEmpty checks if a string is not empty
func assertNotEmpty(t *testing.T, value, msg string) {
	t.Helper()
	if value == "" {
		t.Errorf("%s: expected non-empty string", msg)
	}
}

// assertSliceEqual checks if two string slices are equal
func assertSliceEqual(t *testing.T, got, want []string, msg string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s: got %d items, want %d items", msg, len(got), len(want))
		return
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("%s: at index %d, got %q, want %q", msg, i, got[i], want[i])
		}
	}
}

// assertErrorContains checks if an error contains a specific substring
func assertErrorContains(t *testing.T, err error, substr, msg string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: expected error containing %q, got nil", msg, substr)
		return
	}
	if !contains(err.Error(), substr) {
		t.Errorf("%s: expected error containing %q, got %q", msg, substr, err.Error())
	}
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexOf(s, substr) >= 0)
}

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
