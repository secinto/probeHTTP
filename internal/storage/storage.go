package storage

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// GenerateFilename creates a SHA1 hash-based filename for the request
// Format: SHA1(METHOD:URL)
func GenerateFilename(method, urlStr string) string {
	data := fmt.Sprintf("%s:%s", method, urlStr)
	hash := sha1.Sum([]byte(data))
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

// BuildStoragePath creates the full path for storing a response
// Structure: {baseDir}/response/{sanitized_host}/{hash}.txt
func BuildStoragePath(baseDir, host, filename string) string {
	sanitizedHost := SanitizeHost(host)
	return filepath.Join(baseDir, "response", sanitizedHost, filename+".txt")
}

// EnsureDir creates a directory and all parent directories if they don't exist
func EnsureDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}

// StoreResponse writes the response data to disk
// Returns the path where the file was stored
func StoreResponse(baseDir string, parsedURL *url.URL, method string, data []byte) (string, error) {
	filename := GenerateFilename(method, parsedURL.String())
	storagePath := BuildStoragePath(baseDir, parsedURL.Host, filename)

	if err := EnsureDir(storagePath); err != nil {
		return "", fmt.Errorf("failed to create storage directory: %w", err)
	}

	if err := os.WriteFile(storagePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write response: %w", err)
	}

	return storagePath, nil
}

// FormatStoredResponse formats the stored response file content
// Includes: raw request, raw response headers, redirect chain (if any), final URL, response body
func FormatStoredResponse(rawRequest, rawResponseHeaders string, redirectChain []string, body []byte, finalURL string) []byte {
	var builder strings.Builder

	// Write raw request
	builder.WriteString("=== REQUEST ===\n")
	builder.WriteString(rawRequest)
	if !strings.HasSuffix(rawRequest, "\n") {
		builder.WriteString("\n")
	}
	builder.WriteString("\n")

	// Write response headers
	builder.WriteString("=== RESPONSE HEADERS ===\n")
	builder.WriteString(rawResponseHeaders)
	if !strings.HasSuffix(rawResponseHeaders, "\n") {
		builder.WriteString("\n")
	}
	builder.WriteString("\n")

	// Write redirect chain if present
	if len(redirectChain) > 1 {
		builder.WriteString("=== REDIRECT CHAIN ===\n")
		for i, redirect := range redirectChain {
			builder.WriteString(fmt.Sprintf("[%d] %s\n", i+1, redirect))
		}
		builder.WriteString("\n")
	}

	// Write final URL
	builder.WriteString("=== FINAL URL ===\n")
	builder.WriteString(finalURL)
	builder.WriteString("\n\n")

	// Write response body
	builder.WriteString("=== RESPONSE BODY ===\n")
	builder.Write(body)

	return []byte(builder.String())
}
