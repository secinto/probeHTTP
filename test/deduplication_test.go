package main

import (
	"reflect"
	"testing"

	"probeHTTP/internal/parser"
)

// TestNormalizeURL tests the NormalizeURL function
func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HTTP with explicit port 80",
			input:    "http://example.com:80/",
			expected: "http://example.com/",
		},
		{
			name:     "HTTPS with explicit port 443",
			input:    "https://example.com:443/",
			expected: "https://example.com/",
		},
		{
			name:     "HTTP without port",
			input:    "http://example.com/",
			expected: "http://example.com/",
		},
		{
			name:     "HTTPS without port",
			input:    "https://example.com/",
			expected: "https://example.com/",
		},
		{
			name:     "HTTP with non-standard port",
			input:    "http://example.com:8080/",
			expected: "http://example.com:8080/",
		},
		{
			name:     "HTTPS with non-standard port",
			input:    "https://example.com:8443/",
			expected: "https://example.com:8443/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.NormalizeURL(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeURL() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestDeduplicateURLs tests the DeduplicateURLs function
func TestDeduplicateURLs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "duplicate HTTP URLs",
			input: []string{
				"http://example.com:80/",
				"http://example.com/",
			},
			expected: []string{"http://example.com:80/"},
		},
		{
			name: "duplicate HTTPS URLs",
			input: []string{
				"https://example.com:443/",
				"https://example.com/",
			},
			expected: []string{"https://example.com:443/"},
		},
		{
			name: "mixed duplicates",
			input: []string{
				"http://example.com:80/",
				"http://example.com/",
				"https://example.com:443/",
				"https://example.com/",
				"http://example.com:8080/",
			},
			expected: []string{
				"http://example.com:80/",
				"https://example.com:443/",
				"http://example.com:8080/",
			},
		},
		{
			name: "no duplicates",
			input: []string{
				"http://example.com:80/",
				"https://example.com:443/",
				"http://example.com:8080/",
			},
			expected: []string{
				"http://example.com:80/",
				"https://example.com:443/",
				"http://example.com:8080/",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.DeduplicateURLs(tt.input)
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("DeduplicateURLs() = %v, want %v", got, tt.expected)
			}
		})
	}
}
