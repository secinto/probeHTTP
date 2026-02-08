package storage

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// ChainEntry carries per-hop request/response data for the redirect chain.
// Each hop in the redirect chain produces one ChainEntry.
type ChainEntry struct {
	RawRequest  string // formatted by formatRawRequest (request line + headers)
	RawResponse string // status line + headers via formatRawResponse
	Body        []byte // response body for this hop
}

// GenerateFilename creates a SHA1 hash-based filename from the URL.
// Format: SHA1(URL) — matches HTTPx convention (no method prefix).
func GenerateFilename(urlStr string) string {
	hash := sha1.Sum([]byte(urlStr))
	return hex.EncodeToString(hash[:])
}

// SanitizeHost sanitizes a hostname for use in directory paths
// Handles ports (e.g., example.com:8080 -> example.com_8080)
func SanitizeHost(host string) string {
	// Replace colons with underscores for port handling
	sanitized := strings.ReplaceAll(host, ":", "_")
	// Remove any other potentially problematic characters
	sanitized = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, sanitized)
	return sanitized
}

// BuildStoragePath creates the full path for storing a response.
// Structure: {baseDir}/{sanitized_host}/{hash}.txt — matches HTTPx layout (no response/ subdir).
func BuildStoragePath(baseDir, host, filename string) string {
	sanitizedHost := SanitizeHost(host)
	return filepath.Join(baseDir, sanitizedHost, filename+".txt")
}

// EnsureDir creates a directory and all parent directories if they don't exist
func EnsureDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}

// StoreResponse writes the response data to disk.
// Returns the path where the file was stored.
func StoreResponse(baseDir string, parsedURL *url.URL, data []byte) (string, error) {
	filename := GenerateFilename(parsedURL.String())
	storagePath := BuildStoragePath(baseDir, parsedURL.Host, filename)

	if err := EnsureDir(storagePath); err != nil {
		return "", fmt.Errorf("failed to create storage directory: %w", err)
	}

	if err := os.WriteFile(storagePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write response: %w", err)
	}

	return storagePath, nil
}

// FormatStoredResponse formats the stored response file content in raw HTTP format.
// Output matches HTTPx: raw request/response pairs per hop, final URL on last line.
//
// Format per hop:
//
//	GET / HTTP/1.1\nHost: ...\n\n\nHTTP/1.1 302 Found\n...\n\n[body]\n\n
//
// Final line: bare URL
func FormatStoredResponse(chain []ChainEntry, finalURL string) []byte {
	var builder strings.Builder

	for i, entry := range chain {
		// Write raw request (already ends with \n after last header)
		builder.WriteString(entry.RawRequest)
		// Blank line after request headers (end of request)
		builder.WriteString("\n")

		// Write raw response headers (already ends with \n after last header)
		builder.WriteString(entry.RawResponse)
		// Blank line after response headers (end of headers, before body)
		builder.WriteString("\n")

		// Write body
		if len(entry.Body) > 0 {
			builder.Write(entry.Body)
		}

		// Separator between hops (blank line before next hop's request)
		if i < len(chain)-1 {
			builder.WriteString("\n\n")
		}
	}

	// Final URL on last line
	builder.WriteString("\n\n")
	builder.WriteString(finalURL)

	return []byte(builder.String())
}

// indexMu protects concurrent writes to index.txt
var indexMu sync.Mutex

// AppendToIndex appends a line to the index.txt file in the storage directory.
// Format matches HTTPx: {relative_path} {url} ({statusCode} {statusText})
// Example: www.hall.ag_80/85c766da...c4.txt http://www.hall.ag:80 (200 OK)
func AppendToIndex(baseDir, storagePath, urlStr string, statusCode int, statusText string) error {
	indexMu.Lock()
	defer indexMu.Unlock()

	indexPath := filepath.Join(baseDir, "index.txt")
	relPath, _ := filepath.Rel(baseDir, storagePath)
	line := fmt.Sprintf("%s %s (%d %s)\n", relPath, urlStr, statusCode, statusText)

	f, err := os.OpenFile(indexPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}
