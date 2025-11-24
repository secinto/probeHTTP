package main

import (
	"testing"

	"probeHTTP/internal/hash"
	"probeHTTP/internal/parser"
)

// Fuzz test for URL parsing
func FuzzParseInputURL(f *testing.F) {
	// Seed corpus
	f.Add("example.com")
	f.Add("http://example.com:8080")
	f.Add("https://example.com/path")
	f.Add("subdomain.example.com:9000/api/v1?query=value#fragment")
	f.Add("://invalid")
	f.Add("http://")
	f.Add("")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		_ = parser.ParseInputURL(input)
	})
}

// Fuzz test for port list parsing
func FuzzParsePortList(f *testing.F) {
	// Seed corpus
	f.Add("80,443")
	f.Add("8000-8010")
	f.Add("80,443,8000-8010")
	f.Add("invalid")
	f.Add("99999")
	f.Add("-1")
	f.Add("8000-7000")
	f.Add("")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		_, _ = parser.ParsePortList(input)
	})
}

// Fuzz test for HTML title extraction
func FuzzExtractTitle(f *testing.F) {
	// Seed corpus
	f.Add("<html><head><title>Test</title></head></html>")
	f.Add("<title>Simple</title>")
	f.Add("<html><head><meta property=\"og:title\" content=\"OG Title\"></head></html>")
	f.Add("<!DOCTYPE html><html></html>")
	f.Add("")
	f.Add("<<<<invalid>>>>")
	f.Add("<title>Test\\u0026Title</title>")

	f.Fuzz(func(t *testing.T, input string) {
		// Should not panic
		_ = parser.ExtractTitle(input)
	})
}

// Fuzz test for MMH3 hash calculation
func FuzzCalculateMMH3(f *testing.F) {
	// Seed corpus
	f.Add([]byte("test data"))
	f.Add([]byte(""))
	f.Add([]byte("very long string with lots of content and data"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Should not panic
		_ = hash.CalculateMMH3(data)
	})
}

// Fuzz test for URL validation
func FuzzValidateURL(f *testing.F) {
	// Seed corpus
	f.Add("http://example.com", false)
	f.Add("https://localhost", false)
	f.Add("http://192.168.1.1", false)
	f.Add("http://10.0.0.1", true)
	f.Add("", false)

	f.Fuzz(func(t *testing.T, url string, allowPrivate bool) {
		// Should not panic
		_ = parser.ValidateURL(url, allowPrivate)
	})
}

// Fuzz test for URL expansion
func FuzzExpandURLs(f *testing.F) {
	// Seed corpus
	f.Add("example.com", false, false, "")
	f.Add("http://example.com:8080", true, false, "")
	f.Add("example.com", false, true, "")
	f.Add("example.com", false, false, "80,443")

	f.Fuzz(func(t *testing.T, url string, allSchemes bool, ignorePorts bool, customPorts string) {
		// Should not panic
		_ = parser.ExpandURLs(url, allSchemes, ignorePorts, customPorts)
	})
}
