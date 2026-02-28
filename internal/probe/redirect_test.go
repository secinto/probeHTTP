package probe

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"probeHTTP/internal/config"
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

func TestFollowRedirects_302To200(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Location", "/final")
			w.WriteHeader(http.StatusFound)
			return
		}
		if r.URL.Path == "/final" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ok"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	prober := NewProber(cfg)
	defer prober.Close()

	client := prober.client.GetHTTPClient()
	req, _ := http.NewRequest("GET", server.URL+"/", nil)
	req = req.WithContext(context.Background())
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("initial request: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected 302, got %d", resp.StatusCode)
	}

	u, _ := url.Parse(server.URL)
	hostname := u.Hostname()
	if u.Port() != "" {
		hostname = u.Hostname() + ":" + u.Port()
	}
	hostname = u.Host

	var buf strings.Builder
	ctx := context.Background()
	finalResp, statusChain, hostChain, _, err := prober.followRedirects(ctx, resp, 10, 1, hostname, &buf, client)
	if err != nil {
		t.Fatalf("followRedirects: %v", err)
	}
	defer finalResp.Body.Close()

	if finalResp.StatusCode != http.StatusOK {
		t.Errorf("final status = %d, want 200", finalResp.StatusCode)
	}
	if len(statusChain) != 2 || statusChain[0] != 302 || statusChain[1] != 200 {
		t.Errorf("statusChain = %v, want [302, 200]", statusChain)
	}
	if len(hostChain) != 2 {
		t.Errorf("hostChain length = %d, want 2", len(hostChain))
	}
}

func TestFollowRedirects_NoLocationHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	prober := NewProber(cfg)
	defer prober.Close()

	client := prober.client.GetHTTPClient()
	req, _ := http.NewRequest("GET", server.URL+"/", nil)
	req = req.WithContext(context.Background())
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("initial request: %v", err)
	}

	u, _ := url.Parse(server.URL)
	hostname := u.Host
	var buf strings.Builder
	ctx := context.Background()
	finalResp, statusChain, _, _, err := prober.followRedirects(ctx, resp, 10, 1, hostname, &buf, client)
	if err != nil {
		t.Fatalf("followRedirects: %v", err)
	}
	defer finalResp.Body.Close()

	if finalResp.StatusCode != http.StatusFound {
		t.Errorf("final status = %d, want 302 (no Location, should stop)", finalResp.StatusCode)
	}
	if len(statusChain) != 1 || statusChain[0] != 302 {
		t.Errorf("statusChain = %v, want [302]", statusChain)
	}
}

func TestFollowRedirects_MaxRedirectsExceeded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/loop")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	prober := NewProber(cfg)
	defer prober.Close()

	client := prober.client.GetHTTPClient()
	req, _ := http.NewRequest("GET", server.URL+"/", nil)
	req = req.WithContext(context.Background())
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("initial request: %v", err)
	}

	u, _ := url.Parse(server.URL)
	hostname := u.Host
	var buf strings.Builder
	ctx := context.Background()
	_, _, _, _, err = prober.followRedirects(ctx, resp, 10, 1, hostname, &buf, client)
	if err == nil {
		t.Fatal("followRedirects should error on redirect loop")
	}
	if !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Errorf("error = %v, want 'stopped after 10 redirects'", err)
	}
}

func TestFollowRedirects_SameHostOnly_BlocksCrossHost(t *testing.T) {
	serverB := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer serverB.Close()

	serverA := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", serverB.URL+"/")
		w.WriteHeader(http.StatusFound)
	}))
	defer serverA.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.SameHostOnly = true
	prober := NewProber(cfg)
	defer prober.Close()

	client := prober.client.GetHTTPClient()
	req, _ := http.NewRequest("GET", serverA.URL+"/", nil)
	req = req.WithContext(context.Background())
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("initial request: %v", err)
	}

	u, _ := url.Parse(serverA.URL)
	hostname := u.Host
	var buf strings.Builder
	ctx := context.Background()
	_, _, _, _, err = prober.followRedirects(ctx, resp, 10, 1, hostname, &buf, client)
	if err == nil {
		t.Fatal("followRedirects should error on cross-host redirect with SameHostOnly")
	}
	if !strings.Contains(err.Error(), "cross-host redirect blocked") {
		t.Errorf("error = %v, want 'cross-host redirect blocked'", err)
	}
}

func TestFollowRedirects_NonRedirectReturnsImmediately(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	prober := NewProber(cfg)
	defer prober.Close()

	client := prober.client.GetHTTPClient()
	req, _ := http.NewRequest("GET", server.URL+"/", nil)
	req = req.WithContext(context.Background())
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("initial request: %v", err)
	}
	defer resp.Body.Close()

	u, _ := url.Parse(server.URL)
	hostname := u.Host
	var buf strings.Builder
	ctx := context.Background()
	finalResp, statusChain, hostChain, _, err := prober.followRedirects(ctx, resp, 10, 1, hostname, &buf, client)
	if err != nil {
		t.Fatalf("followRedirects: %v", err)
	}
	defer finalResp.Body.Close()

	if finalResp.StatusCode != http.StatusOK {
		t.Errorf("final status = %d, want 200", finalResp.StatusCode)
	}
	if len(statusChain) != 1 || statusChain[0] != 200 {
		t.Errorf("statusChain = %v, want [200]", statusChain)
	}
	if len(hostChain) != 1 {
		t.Errorf("hostChain length = %d, want 1", len(hostChain))
	}
}
