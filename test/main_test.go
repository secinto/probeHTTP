package main

import (
	"bufio"
	"io"
	"net/http"
	"strings"
	"testing"

	"probeHTTP/internal/hash"
	"probeHTTP/internal/parser"
)

// readURLs reads URLs from the input reader, skipping comments and empty lines
// This is a test helper that duplicates the function from cmd/probehttp/main.go
func readURLs(reader io.Reader) []string {
	var urls []string
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			urls = append(urls, line)
		}
	}
	return urls
}

// TestReadURLs tests the readURLs function
func TestReadURLs(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single URL",
			input: "https://example.com",
			want:  []string{"https://example.com"},
		},
		{
			name:  "multiple URLs",
			input: "https://example.com\nhttps://github.com\nhttp://google.com",
			want:  []string{"https://example.com", "https://github.com", "http://google.com"},
		},
		{
			name:  "URLs with comments",
			input: "# Comment line\nhttps://example.com\n# Another comment\nhttps://github.com",
			want:  []string{"https://example.com", "https://github.com"},
		},
		{
			name:  "URLs with empty lines",
			input: "https://example.com\n\n\nhttps://github.com",
			want:  []string{"https://example.com", "https://github.com"},
		},
		{
			name:  "URLs with whitespace",
			input: "  https://example.com  \n\t\thttps://github.com\t\t",
			want:  []string{"https://example.com", "https://github.com"},
		},
		{
			name:  "empty input",
			input: "",
			want:  []string{},
		},
		{
			name:  "only comments and empty lines",
			input: "# Comment\n\n# Another comment\n\n",
			want:  []string{},
		},
		{
			name:  "mixed content",
			input: "# Header\nhttps://example.com\n\n# Section 2\nhttp://test.com\n\n",
			want:  []string{"https://example.com", "http://test.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := strings.NewReader(tt.input)
			got := readURLs(reader)
			assertSliceEqual(t, got, tt.want, "readURLs")
		})
	}
}

// TestCalculateMMH3 tests the calculateMMH3 function
func TestCalculateMMH3(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
		want  string
	}{
		{
			name:  "empty data",
			input: []byte(""),
			want:  "0",
		},
		{
			name:  "simple text",
			input: []byte("hello world"),
			want:  "1586663183", // Known MMH3 hash for "hello world"
		},
		{
			name:  "same input produces same hash",
			input: []byte("test data"),
			want:  hash.CalculateMMH3([]byte("test data")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hash.CalculateMMH3(tt.input)
			assertStringEqual(t, got, tt.want, "calculateMMH3")
		})
	}
}

// TestCalculateMMH3_Consistency tests hash consistency
func TestCalculateMMH3_Consistency(t *testing.T) {
	data := []byte("consistency test")
	hash1 := hash.CalculateMMH3(data)
	hash2 := hash.CalculateMMH3(data)
	assertStringEqual(t, hash1, hash2, "hash consistency")
}

// TestCalculateHeaderMMH3 tests the calculateHeaderMMH3 function
func TestCalculateHeaderMMH3(t *testing.T) {
	tests := []struct {
		name    string
		headers http.Header
		want    string
	}{
		{
			name:    "empty headers",
			headers: http.Header{},
			want:    "0",
		},
		{
			name: "single header",
			headers: http.Header{
				"Content-Type": []string{"text/html"},
			},
			want: hash.CalculateHeaderMMH3(http.Header{
				"Content-Type": []string{"text/html"},
			}),
		},
		{
			name: "multiple headers",
			headers: http.Header{
				"Content-Type": []string{"text/html"},
				"Server":       []string{"nginx"},
			},
			want: hash.CalculateHeaderMMH3(http.Header{
				"Content-Type": []string{"text/html"},
				"Server":       []string{"nginx"},
			}),
		},
		{
			name: "header with multiple values",
			headers: http.Header{
				"Set-Cookie": []string{"cookie1=value1", "cookie2=value2"},
			},
			want: hash.CalculateHeaderMMH3(http.Header{
				"Set-Cookie": []string{"cookie1=value1", "cookie2=value2"},
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hash.CalculateHeaderMMH3(tt.headers)
			assertStringEqual(t, got, tt.want, "calculateHeaderMMH3")
		})
	}
}

// TestCalculateHeaderMMH3_OrderIndependent tests that header order doesn't affect hash
func TestCalculateHeaderMMH3_OrderIndependent(t *testing.T) {
	headers1 := http.Header{
		"Content-Type": []string{"text/html"},
		"Server":       []string{"nginx"},
		"X-Custom":     []string{"value"},
	}

	headers2 := http.Header{
		"Server":       []string{"nginx"},
		"X-Custom":     []string{"value"},
		"Content-Type": []string{"text/html"},
	}

	hash1 := hash.CalculateHeaderMMH3(headers1)
	hash2 := hash.CalculateHeaderMMH3(headers2)

	assertStringEqual(t, hash1, hash2, "header order independence")
}

// TestExtractTitle tests the extractTitle function
func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple title",
			input: "<html><head><title>Test Page</title></head><body></body></html>",
			want:  "Test Page",
		},
		{
			name:  "title with whitespace",
			input: "<html><head><title>  Test Page  </title></head><body></body></html>",
			want:  "Test Page",
		},
		{
			name:  "no title",
			input: "<html><head></head><body></body></html>",
			want:  "",
		},
		{
			name:  "empty title",
			input: "<html><head><title></title></head><body></body></html>",
			want:  "",
		},
		{
			name:  "nested in deep structure",
			input: "<html><head><meta charset='utf-8'><title>Deep Title</title></head><body></body></html>",
			want:  "Deep Title",
		},
		{
			name:  "malformed HTML",
			input: "<html><title>Malformed</title><body></body>",
			want:  "Malformed",
		},
		{
			name:  "title with special characters",
			input: "<html><head><title>Test & Demo - Page</title></head><body></body></html>",
			want:  "Test & Demo - Page",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "non-HTML content",
			input: "This is just plain text",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parser.ExtractTitle(tt.input)
			assertStringEqual(t, got, tt.want, "extractTitle")
		})
	}
}

// TestCountWordsAndLines tests the countWordsAndLines function
func TestCountWordsAndLines(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantWords int
		wantLines int
	}{
		{
			name:      "empty string",
			input:     "",
			wantWords: 0,
			wantLines: 0,
		},
		{
			name:      "single word",
			input:     "hello",
			wantWords: 1,
			wantLines: 1,
		},
		{
			name:      "multiple words",
			input:     "hello world test",
			wantWords: 3,
			wantLines: 1,
		},
		{
			name:      "multiple lines",
			input:     "line one\nline two\nline three",
			wantWords: 6,
			wantLines: 3,
		},
		{
			name:      "lines with varying words",
			input:     "one\ntwo three\nfour five six",
			wantWords: 6,
			wantLines: 3,
		},
		{
			name:      "text with extra whitespace",
			input:     "word1  word2   word3",
			wantWords: 3,
			wantLines: 1,
		},
		{
			name:      "text with tabs",
			input:     "word1\tword2\tword3",
			wantWords: 3,
			wantLines: 1,
		},
		{
			name:      "text with newlines only",
			input:     "\n\n\n",
			wantWords: 0,
			wantLines: 4,
		},
		{
			name:      "HTML content",
			input:     "<html><body><p>Hello World</p></body></html>",
			wantWords: 2, // HTML tags are not counted as words
			wantLines: 1,
		},
		{
			name:      "mixed content with punctuation",
			input:     "Hello, world! This is a test.\nAnother line here.",
			wantWords: 9, // Punctuation may attach to words
			wantLines: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotWords, gotLines := parser.CountWordsAndLines(tt.input)
			assertIntEqual(t, gotWords, tt.wantWords, "word count")
			assertIntEqual(t, gotLines, tt.wantLines, "line count")
		})
	}
}

// NOTE: TestCreateHTTPClient and TestProcessURLs have been removed because
// createHTTPClient() and processURLs() functions have been refactored into
// the internal/probe package. These functions are now tested via integration tests.
