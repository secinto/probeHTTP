package parser

import (
	"strings"
	"testing"
)

// --- ParseInputURL ---

func TestParseInputURL(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantScheme string
		wantHost   string
		wantPort   string
		wantPath   string
	}{
		{"bare hostname", "example.com", "", "example.com", "", "/"},
		{"hostname with port", "example.com:8080", "", "example.com", "8080", "/"},
		{"http URL", "http://example.com", "http", "example.com", "", "/"},
		{"https URL", "https://example.com", "https", "example.com", "", "/"},
		{"https with port", "https://example.com:8443", "https", "example.com", "8443", "/"},
		{"with path", "http://example.com/path/to/page", "http", "example.com", "", "/path/to/page"},
		{"with query", "http://example.com/page?q=1&b=2", "http", "example.com", "", "/page?q=1&b=2"},
		{"with fragment", "http://example.com/page#section", "http", "example.com", "", "/page#section"},
		{"bare host with path", "example.com/path", "", "example.com", "", "/path"},
		{"bare host port path", "example.com:8080/path", "", "example.com", "8080", "/path"},
		{"port 443", "example.com:443", "", "example.com", "443", "/"},
		{"port 80", "example.com:80", "", "example.com", "80", "/"},
		{"IP address", "192.168.1.1", "", "192.168.1.1", "", "/"},
		{"IP with port", "192.168.1.1:8080", "", "192.168.1.1", "8080", "/"},
		{"full URL all components", "https://example.com:8443/api?key=val#ref", "https", "example.com", "8443", "/api?key=val#ref"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseInputURL(tt.input)
			if got.Scheme != tt.wantScheme {
				t.Errorf("Scheme = %q, want %q", got.Scheme, tt.wantScheme)
			}
			if got.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", got.Host, tt.wantHost)
			}
			if got.Port != tt.wantPort {
				t.Errorf("Port = %q, want %q", got.Port, tt.wantPort)
			}
			if got.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", got.Path, tt.wantPath)
			}
		})
	}
}

func TestParseInputURL_InvalidPort(t *testing.T) {
	// Non-numeric port should be treated as part of the host
	got := ParseInputURL("example.com:invalid")
	if got.Port != "" {
		t.Errorf("Port = %q, want empty for invalid port", got.Port)
	}
	if got.Host != "example.com:invalid" {
		t.Errorf("Host = %q, want %q", got.Host, "example.com:invalid")
	}
}

// --- ValidateURL ---

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		allowPrivateIPs bool
		wantErr         bool
		errContains     string
	}{
		{"valid public URL", "https://example.com", false, false, ""},
		{"valid with private allowed", "http://192.168.1.1", true, false, ""},
		{"empty hostname", "", false, true, "empty hostname"},
		{"null bytes", "https://example\x00.com", false, true, "null bytes"},
		{"too long", "https://" + strings.Repeat("a", 2050), false, true, "too long"},
		{"localhost blocked", "localhost", false, true, "localhost not allowed"},
		{"127.0.0.1 blocked", "127.0.0.1", false, true, "localhost not allowed"},
		{"private IP blocked", "192.168.1.1", false, true, "private IP"},
		{"10.x.x.x blocked", "10.0.0.1", false, true, "private IP"},
		{"localhost allowed", "localhost", true, false, ""},
		{"private IP allowed", "192.168.1.1", true, false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.input, tt.allowPrivateIPs)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %q, want to contain %q", err.Error(), tt.errContains)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// --- ExpandURLs ---

func TestExpandURLs_BareHostname(t *testing.T) {
	urls := ExpandURLs("example.com", false, false, "")
	// No scheme → tests both http and https
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %v", urls)
	}
	has := map[string]bool{}
	for _, u := range urls {
		has[u] = true
	}
	if !has["http://example.com/"] || !has["https://example.com/"] {
		t.Errorf("expected http and https URLs, got %v", urls)
	}
}

func TestExpandURLs_ExplicitScheme(t *testing.T) {
	urls := ExpandURLs("https://example.com", false, false, "")
	if len(urls) != 1 || urls[0] != "https://example.com/" {
		t.Errorf("got %v, want [https://example.com/]", urls)
	}
}

func TestExpandURLs_Port443ForcesHTTPS(t *testing.T) {
	urls := ExpandURLs("example.com:443", false, false, "")
	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %v", urls)
	}
	if !strings.HasPrefix(urls[0], "https://") {
		t.Errorf("port 443 should force https, got %q", urls[0])
	}
}

func TestExpandURLs_Port80ForcesHTTP(t *testing.T) {
	urls := ExpandURLs("example.com:80", false, false, "")
	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %v", urls)
	}
	if !strings.HasPrefix(urls[0], "http://") {
		t.Errorf("port 80 should force http, got %q", urls[0])
	}
}

func TestExpandURLs_AllSchemes(t *testing.T) {
	urls := ExpandURLs("https://example.com", true, false, "")
	if len(urls) != 2 {
		t.Fatalf("allSchemes should produce 2 URLs, got %v", urls)
	}
}

func TestExpandURLs_CustomPorts(t *testing.T) {
	urls := ExpandURLs("https://example.com", false, false, "8443,9443")
	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs for 2 custom ports, got %v", urls)
	}
	for _, u := range urls {
		if !strings.Contains(u, ":8443") && !strings.Contains(u, ":9443") {
			t.Errorf("expected custom port in URL, got %q", u)
		}
	}
}

func TestExpandURLs_IgnorePorts(t *testing.T) {
	urls := ExpandURLs("https://example.com:9999", false, true, "")
	// ignorePorts uses default HTTPS ports: 443, 8443, 10443, 8444
	if len(urls) != 4 {
		t.Fatalf("expected 4 URLs for default HTTPS ports, got %d: %v", len(urls), urls)
	}
}

// --- NormalizeURL ---

func TestNormalizeURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"http://example.com:80/path", "http://example.com/path"},
		{"https://example.com:443/path", "https://example.com/path"},
		{"http://example.com:8080/path", "http://example.com:8080/path"},
		{"https://example.com:8443/path", "https://example.com:8443/path"},
		{"http://example.com/path", "http://example.com/path"},
		// url.Parse succeeds for non-URL strings and returns URL-encoded form
		{"not a url", "not%20a%20url"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeURL(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeURL(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// --- DeduplicateURLs ---

func TestDeduplicateURLs(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  int
	}{
		{"exact duplicates", []string{"http://a.com/", "http://a.com/"}, 1},
		{"port normalization", []string{"http://a.com/", "http://a.com:80/"}, 1},
		{"https port normalization", []string{"https://a.com/", "https://a.com:443/"}, 1},
		{"different schemes kept", []string{"http://a.com/", "https://a.com/"}, 2},
		{"different ports kept", []string{"http://a.com:8080/", "http://a.com:9090/"}, 2},
		{"preserves first", []string{"http://a.com:80/", "http://a.com/"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeduplicateURLs(tt.input)
			if len(got) != tt.want {
				t.Errorf("DeduplicateURLs returned %d URLs, want %d; got %v", len(got), tt.want, got)
			}
		})
	}
}

// --- getSchemesToTest ---

func TestGetSchemesToTest(t *testing.T) {
	tests := []struct {
		name       string
		parsed     ParsedURL
		allSchemes bool
		want       []string
	}{
		{"all schemes flag", ParsedURL{}, true, []string{"http", "https"}},
		{"port 443", ParsedURL{Port: "443"}, false, []string{"https"}},
		{"port 80", ParsedURL{Port: "80"}, false, []string{"http"}},
		{"no scheme", ParsedURL{}, false, []string{"http", "https"}},
		{"explicit http", ParsedURL{Scheme: "http"}, false, []string{"http"}},
		{"explicit https", ParsedURL{Scheme: "https"}, false, []string{"https"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getSchemesToTest(tt.parsed, tt.allSchemes)
			if len(got) != len(tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
			for i, s := range got {
				if s != tt.want[i] {
					t.Errorf("schemes[%d] = %q, want %q", i, s, tt.want[i])
				}
			}
		})
	}
}
