package probe

import (
	"net/http"
	"sort"
	"testing"
)

func TestExtractCSPDomains_EmptyHeader(t *testing.T) {
	headers := http.Header{}
	domains := ExtractCSPDomains(headers)
	if domains != nil {
		t.Errorf("expected nil, got %v", domains)
	}
}

func TestExtractCSPDomains_BasicDirectives(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Security-Policy",
		"default-src 'self' cdn.example.com; script-src scripts.example.com 'unsafe-inline'; img-src images.example.com data:")

	domains := ExtractCSPDomains(headers)
	sort.Strings(domains)

	expected := []string{"cdn.example.com", "images.example.com", "scripts.example.com"}
	if len(domains) != len(expected) {
		t.Fatalf("domain count = %d, want %d; got %v", len(domains), len(expected), domains)
	}

	for i, d := range domains {
		if d != expected[i] {
			t.Errorf("domains[%d] = %q, want %q", i, d, expected[i])
		}
	}
}

func TestExtractCSPDomains_FiltersKeywords(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Security-Policy",
		"default-src 'self' 'unsafe-inline' 'unsafe-eval' data: blob: https: http: 'none' 'strict-dynamic' actual.example.com")

	domains := ExtractCSPDomains(headers)
	if len(domains) != 1 {
		t.Fatalf("expected 1 domain, got %v", domains)
	}
	if domains[0] != "actual.example.com" {
		t.Errorf("domain = %q, want %q", domains[0], "actual.example.com")
	}
}

func TestExtractCSPDomains_URLSources(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Security-Policy",
		"script-src https://cdn.example.com/path; connect-src wss://ws.example.com:8080")

	domains := ExtractCSPDomains(headers)
	sort.Strings(domains)

	expected := []string{"cdn.example.com", "ws.example.com"}
	if len(domains) != len(expected) {
		t.Fatalf("domain count = %d, want %d; got %v", len(domains), len(expected), domains)
	}
}

func TestExtractCSPDomains_Deduplication(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Security-Policy",
		"default-src cdn.example.com; script-src cdn.example.com; style-src cdn.example.com")

	domains := ExtractCSPDomains(headers)
	if len(domains) != 1 {
		t.Fatalf("expected 1 domain after dedup, got %v", domains)
	}
}

func TestExtractCSPDomains_WildcardDomains(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Security-Policy", "img-src *.example.com")

	domains := ExtractCSPDomains(headers)
	if len(domains) != 1 || domains[0] != "*.example.com" {
		t.Errorf("expected [*.example.com], got %v", domains)
	}
}

func TestExtractCSPDomains_FiltersNoncesAndHashes(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Security-Policy",
		"script-src 'nonce-abc123' 'sha256-xyz789' real.example.com")

	domains := ExtractCSPDomains(headers)
	if len(domains) != 1 || domains[0] != "real.example.com" {
		t.Errorf("expected [real.example.com], got %v", domains)
	}
}

func TestExtractCSPDomains_IrrelevantDirectives(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Security-Policy",
		"report-uri https://report.example.com/csp; upgrade-insecure-requests")

	domains := ExtractCSPDomains(headers)
	if len(domains) != 0 {
		t.Errorf("expected no domains from irrelevant directives, got %v", domains)
	}
}

func TestStripPort(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"example.com:443", "example.com"},
		{"example.com", "example.com"},
		{"[::1]:443", "[::1]:443"}, // IPv6 with brackets not stripped
	}

	for _, tt := range tests {
		got := stripPort(tt.input)
		if got != tt.want {
			t.Errorf("stripPort(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
