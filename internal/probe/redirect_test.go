package probe

import (
	"net/url"
	"testing"
)

func TestNormalizeRedirectURL_SameScheme(t *testing.T) {
	current, _ := url.Parse("http://example.com/page")
	next, _ := url.Parse("http://example.com/other")
	result := normalizeRedirectURL(current, next)
	if result.String() != "http://example.com/other" {
		t.Errorf("same scheme should return unchanged, got %q", result.String())
	}
}

func TestNormalizeRedirectURL_HTTPtoHTTPS_DefaultPort(t *testing.T) {
	// http://host:80 → https://host:80 should normalize to https://host (no port)
	current, _ := url.Parse("http://example.com")
	next, _ := url.Parse("https://example.com:80")
	result := normalizeRedirectURL(current, next)
	if result.Port() != "" {
		t.Errorf("port should be removed, got %q (full: %s)", result.Port(), result.String())
	}
}

func TestNormalizeRedirectURL_HTTPtoHTTPS_NoExplicitPort(t *testing.T) {
	// No explicit port in next URL → no normalization needed
	current, _ := url.Parse("http://example.com")
	next, _ := url.Parse("https://example.com/page")
	result := normalizeRedirectURL(current, next)
	if result.String() != "https://example.com/page" {
		t.Errorf("no explicit port, should pass through, got %q", result.String())
	}
}

func TestNormalizeRedirectURL_HTTPStoHTTP_DefaultPort(t *testing.T) {
	// https://host → http://host:443 should normalize
	current, _ := url.Parse("https://example.com")
	next, _ := url.Parse("http://example.com:443")
	result := normalizeRedirectURL(current, next)
	if result.Port() != "" {
		t.Errorf("port should be removed, got %q (full: %s)", result.Port(), result.String())
	}
}

func TestNormalizeRedirectURL_CustomPort_Preserved(t *testing.T) {
	// http://host:8080 → https://host:8080 should keep the custom port
	current, _ := url.Parse("http://example.com:8080")
	next, _ := url.Parse("https://example.com:8080")
	result := normalizeRedirectURL(current, next)
	if result.Port() != "8080" {
		t.Errorf("custom port should be preserved, got %q", result.Port())
	}
}

func TestNormalizeRedirectURL_ExplicitDefaultPort_Current(t *testing.T) {
	// http://host:80 → https://host:80 should normalize (explicit port 80 on HTTP)
	current, _ := url.Parse("http://example.com:80")
	next, _ := url.Parse("https://example.com:80")
	result := normalizeRedirectURL(current, next)
	if result.Port() != "" {
		t.Errorf("default port carried over should be removed, got %q", result.Port())
	}
}

func TestNormalizeRedirectURL_PathPreserved(t *testing.T) {
	current, _ := url.Parse("http://example.com/page")
	next, _ := url.Parse("https://example.com:80/new-path?q=1")
	result := normalizeRedirectURL(current, next)
	if result.Path != "/new-path" {
		t.Errorf("path should be preserved, got %q", result.Path)
	}
	if result.RawQuery != "q=1" {
		t.Errorf("query should be preserved, got %q", result.RawQuery)
	}
}
