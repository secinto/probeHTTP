package storage

import (
	"crypto/sha1"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestGenerateFilename(t *testing.T) {
	// GenerateFilename should produce SHA1(URL) — no method prefix
	urlStr := "https://example.com"
	expected := sha1Hex(urlStr)
	got := GenerateFilename(urlStr)
	if got != expected {
		t.Errorf("GenerateFilename(%q) = %q, want %q", urlStr, got, expected)
	}

	// Different URLs must produce different hashes
	other := GenerateFilename("https://example.com/path")
	if got == other {
		t.Error("different URLs should produce different filenames")
	}

	// Same URL should always produce the same hash
	again := GenerateFilename(urlStr)
	if got != again {
		t.Error("same URL should produce same filename")
	}
}

func TestBuildStoragePath(t *testing.T) {
	tests := []struct {
		name     string
		baseDir  string
		host     string
		filename string
		want     string
	}{
		{
			name:     "simple host no port",
			baseDir:  "/tmp/responses",
			host:     "example.com",
			filename: "abc123",
			want:     filepath.Join("/tmp/responses", "example.com", "abc123.txt"),
		},
		{
			name:     "host with port",
			baseDir:  "/tmp/responses",
			host:     "example.com:8080",
			filename: "abc123",
			want:     filepath.Join("/tmp/responses", "example.com_8080", "abc123.txt"),
		},
		{
			name:     "no response subdir",
			baseDir:  "/data/srd",
			host:     "www.hall.ag:80",
			filename: "deadbeef",
			// Must NOT contain "response/" segment
			want: filepath.Join("/data/srd", "www.hall.ag_80", "deadbeef.txt"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildStoragePath(tt.baseDir, tt.host, tt.filename)
			if got != tt.want {
				t.Errorf("BuildStoragePath() = %q, want %q", got, tt.want)
			}
			// Verify no "response" segment anywhere in the path
			if strings.Contains(got, "/response/") {
				t.Error("BuildStoragePath should not contain /response/ segment")
			}
		})
	}
}

func TestFormatStoredResponse_SingleHop(t *testing.T) {
	chain := []ChainEntry{
		{
			RawRequest:  "GET / HTTP/1.1\nHost: example.com\nUser-Agent: test\n",
			RawResponse: "HTTP/1.1 200 OK\nContent-Type: text/html\n",
			Body:        []byte("<html>hello</html>"),
		},
	}
	finalURL := "https://example.com"

	result := string(FormatStoredResponse(chain, finalURL))

	// Should contain request headers followed by blank line
	if !strings.Contains(result, "GET / HTTP/1.1\n") {
		t.Error("should contain request line")
	}

	// Should contain response headers
	if !strings.Contains(result, "HTTP/1.1 200 OK\n") {
		t.Error("should contain response status line")
	}

	// Should contain body
	if !strings.Contains(result, "<html>hello</html>") {
		t.Error("should contain response body")
	}

	// Final URL should be the last line
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	lastLine := lines[len(lines)-1]
	if lastLine != finalURL {
		t.Errorf("last line should be final URL %q, got %q", finalURL, lastLine)
	}

	// Should NOT contain any === SECTION === markers
	if strings.Contains(result, "===") {
		t.Error("should not contain === section markers")
	}
}

func TestFormatStoredResponse_MultiHop(t *testing.T) {
	chain := []ChainEntry{
		{
			RawRequest:  "GET / HTTP/1.1\nHost: example.com\n",
			RawResponse: "HTTP/1.1 302 Found\nLocation: https://example.com/\n",
			Body:        []byte("redirect"),
		},
		{
			RawRequest:  "GET / HTTP/1.1\nHost: example.com\n",
			RawResponse: "HTTP/1.1 200 OK\nContent-Type: text/html\n",
			Body:        []byte("<html>final</html>"),
		},
	}
	finalURL := "https://example.com/"

	result := string(FormatStoredResponse(chain, finalURL))

	// Should contain both request lines (two hops)
	if strings.Count(result, "GET / HTTP/1.1") != 2 {
		t.Errorf("should contain 2 request lines, got %d", strings.Count(result, "GET / HTTP/1.1"))
	}

	// Should contain both response status lines
	if !strings.Contains(result, "HTTP/1.1 302 Found") {
		t.Error("should contain 302 redirect response")
	}
	if !strings.Contains(result, "HTTP/1.1 200 OK") {
		t.Error("should contain 200 OK response")
	}

	// Should contain both bodies
	if !strings.Contains(result, "redirect") {
		t.Error("should contain redirect body")
	}
	if !strings.Contains(result, "<html>final</html>") {
		t.Error("should contain final body")
	}

	// Final URL on last line
	lines := strings.Split(strings.TrimRight(result, "\n"), "\n")
	if lines[len(lines)-1] != finalURL {
		t.Errorf("last line should be %q, got %q", finalURL, lines[len(lines)-1])
	}

	// 302 response must appear before 200 response
	idx302 := strings.Index(result, "302 Found")
	idx200 := strings.Index(result, "200 OK")
	if idx302 >= idx200 {
		t.Error("302 hop should appear before 200 hop in output")
	}
}

func TestFormatStoredResponse_EmptyBody(t *testing.T) {
	chain := []ChainEntry{
		{
			RawRequest:  "GET / HTTP/1.1\nHost: example.com\n",
			RawResponse: "HTTP/1.1 204 No Content\n",
			Body:        nil,
		},
	}
	finalURL := "https://example.com"

	result := string(FormatStoredResponse(chain, finalURL))

	// Should still have the final URL
	if !strings.HasSuffix(strings.TrimRight(result, "\n"), finalURL) {
		t.Error("should end with final URL even with empty body")
	}
}

func TestStoreResponse(t *testing.T) {
	tmpDir := t.TempDir()

	data := []byte("test response data")
	parsedURL := mustParseURL("https://example.com/page")

	storagePath, err := StoreResponse(tmpDir, parsedURL, data)
	if err != nil {
		t.Fatalf("StoreResponse() error: %v", err)
	}

	// Verify file exists and content matches
	content, err := os.ReadFile(storagePath)
	if err != nil {
		t.Fatalf("failed to read stored file: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("stored content mismatch: got %q, want %q", content, data)
	}

	// Verify path does not contain /response/ segment
	if strings.Contains(storagePath, "/response/") {
		t.Error("storage path should not contain /response/ segment")
	}

	// Verify filename is SHA1(URL)
	expectedHash := sha1Hex("https://example.com/page")
	if !strings.Contains(storagePath, expectedHash) {
		t.Errorf("storage path should contain SHA1 hash %q", expectedHash)
	}
}

func TestAppendToIndex(t *testing.T) {
	tmpDir := t.TempDir()

	storagePath := filepath.Join(tmpDir, "example.com_80", "abc123.txt")
	err := AppendToIndex(tmpDir, storagePath, "http://example.com:80", 200, "OK")
	if err != nil {
		t.Fatalf("AppendToIndex() error: %v", err)
	}

	// Read index file
	indexPath := filepath.Join(tmpDir, "index.txt")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index.txt: %v", err)
	}

	line := string(content)

	// Verify format: {relative_path} {url} ({statusCode} {statusText})
	expected := "example.com_80/abc123.txt http://example.com:80 (200 OK)\n"
	if line != expected {
		t.Errorf("index line = %q, want %q", line, expected)
	}
}

func TestAppendToIndex_Multiple(t *testing.T) {
	tmpDir := t.TempDir()

	AppendToIndex(tmpDir, filepath.Join(tmpDir, "a.com_80", "hash1.txt"), "http://a.com:80", 200, "OK")
	AppendToIndex(tmpDir, filepath.Join(tmpDir, "b.com_443", "hash2.txt"), "https://b.com:443", 301, "Moved Permanently")

	content, _ := os.ReadFile(filepath.Join(tmpDir, "index.txt"))
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")

	if len(lines) != 2 {
		t.Fatalf("expected 2 index lines, got %d", len(lines))
	}

	if !strings.Contains(lines[0], "http://a.com:80") {
		t.Error("first line should reference a.com")
	}
	if !strings.Contains(lines[1], "https://b.com:443") {
		t.Error("second line should reference b.com")
	}
}

func TestAppendToIndex_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()

	var wg sync.WaitGroup
	n := 50
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			storagePath := filepath.Join(tmpDir, "host", "file.txt")
			AppendToIndex(tmpDir, storagePath, "http://example.com", 200, "OK")
		}(i)
	}
	wg.Wait()

	content, _ := os.ReadFile(filepath.Join(tmpDir, "index.txt"))
	lines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")
	if len(lines) != n {
		t.Errorf("expected %d index lines from concurrent writes, got %d", n, len(lines))
	}
}

func TestSanitizeHost(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"example.com", "example.com"},
		{"example.com:8080", "example.com_8080"},
		{"sub.domain.com:443", "sub.domain.com_443"},
		{"my-host.io", "my-host.io"},
	}

	for _, tt := range tests {
		got := SanitizeHost(tt.input)
		if got != tt.want {
			t.Errorf("SanitizeHost(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// helpers

func sha1Hex(s string) string {
	h := sha1.Sum([]byte(s))
	return hex.EncodeToString(h[:])
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil {
		panic(err)
	}
	return u
}
