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

func TestIsSNIRequired(t *testing.T) {
	tests := []struct {
		name      string
		hostname  string
		allErrors []string
		want      bool
	}{
		{
			name:     "bare IPv4 with handshake failure",
			hostname: "193.110.129.78",
			allErrors: []string{
				"modern-tls/HTTP/2: Request failed: tls: handshake failure",
				"compat-tls/HTTP/1.1: Request failed: remote error: tls: handshake failure",
			},
			want: true,
		},
		{
			name:     "bare IPv6 with handshake failure",
			hostname: "2001:db8::1",
			allErrors: []string{
				"modern-tls/HTTP/2: Request failed: remote error: tls: handshake failure",
			},
			want: true,
		},
		{
			name:     "domain name should return false",
			hostname: "example.com",
			allErrors: []string{
				"modern-tls/HTTP/2: Request failed: tls: handshake failure",
			},
			want: false,
		},
		{
			name:     "bare IP with connection refused",
			hostname: "192.168.1.1",
			allErrors: []string{
				"modern-tls/HTTP/2: Request failed: connection refused",
			},
			want: false,
		},
		{
			name:     "bare IP with timeout",
			hostname: "10.0.0.1",
			allErrors: []string{
				"modern-tls/HTTP/2: Request failed: timeout",
			},
			want: false,
		},
		{
			name:     "bare IP with mixed errors disqualifies",
			hostname: "10.0.0.1",
			allErrors: []string{
				"modern-tls/HTTP/2: Request failed: tls: handshake failure",
				"compat-tls/HTTP/1.1: Request failed: connection refused",
			},
			want: false,
		},
		{
			name:     "bare IP with eof disqualifies",
			hostname: "10.0.0.1",
			allErrors: []string{
				"modern-tls/HTTP/2: Request failed: tls: handshake failure",
				"compat-tls/HTTP/1.1: Request failed: eof",
			},
			want: false,
		},
		{
			name:     "bare IP with no errors",
			hostname: "10.0.0.1",
			allErrors: []string{},
			want:      false,
		},
		{
			name:      "bare IP with nil errors",
			hostname:  "10.0.0.1",
			allErrors: nil,
			want:      false,
		},
		{
			name:     "bare IP with remote error only",
			hostname: "172.16.0.1",
			allErrors: []string{
				"modern-tls/HTTP/2: Request failed: remote error: tls: internal error",
			},
			want: true,
		},
		{
			name:     "bare IP with no route to host",
			hostname: "10.0.0.1",
			allErrors: []string{
				"modern-tls/HTTP/2: Request failed: no route to host",
			},
			want: false,
		},
		{
			name:     "bare IP with network unreachable",
			hostname: "10.0.0.1",
			allErrors: []string{
				"modern-tls/HTTP/2: Request failed: network unreachable",
			},
			want: false,
		},
		{
			name:     "bare IP with no such host",
			hostname: "10.0.0.1",
			allErrors: []string{
				"modern-tls/HTTP/2: Request failed: no such host",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSNIRequired(tt.hostname, tt.allErrors)
			if got != tt.want {
				t.Errorf("IsSNIRequired(%q, %v) = %v, want %v", tt.hostname, tt.allErrors, got, tt.want)
			}
		})
	}
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name   string
		errMsg string
		want   bool
	}{
		{"tls handshake", "tls: handshake failure", true},
		{"tls_ prefix", "tls_alert_handshake_failure", true},
		{"handshake", "remote error: tls: handshake failure", true},
		{"connection refused", "connection refused", true},
		{"connection reset", "connection reset by peer", true},
		{"timeout", "request timeout", true},
		{"eof", "unexpected EOF", true},
		{"certificate", "certificate verify failed", true},
		{"no route to host", "no route to host", true},
		{"network unreachable", "network unreachable", true},
		{"protocol", "protocol error", true},
		{"no such host", "no such host", true},
		{"dial tcp", "dial tcp 1.2.3.4:443: connect: connection refused", true},
		{"remote error", "remote error: tls: internal error", true},
		{"invalid url", "Invalid URL: parse error", false},
		{"rate limit", "rate limit wait cancelled", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isConnectionError(tt.errMsg)
			if got != tt.want {
				t.Errorf("isConnectionError(%q) = %v, want %v", tt.errMsg, got, tt.want)
			}
		})
	}
}

func TestGetTLSVersionString(t *testing.T) {
	tests := []struct {
		version uint16
		want    string
	}{
		{0x0304, "1.3"},
		{0x0303, "1.2"},
		{0x0302, "1.1"},
		{0x0301, "1.0"},
		{0x9999, "0x9999"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := getTLSVersionString(tt.version)
			if got != tt.want {
				t.Errorf("getTLSVersionString(0x%04x) = %q, want %q", tt.version, got, tt.want)
			}
		})
	}
}

func TestStripDefaultPort(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"https default port", "https://host:443/path", "https://host/path"},
		{"http default port", "http://host:80/path", "http://host/path"},
		{"https custom port", "https://host:8443/path", "https://host:8443/path"},
		{"http custom port", "http://host:8080/path", "http://host:8080/path"},
		{"https no port", "https://host/path", "https://host/path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.raw)
			if err != nil {
				t.Fatalf("url.Parse(%q): %v", tt.raw, err)
			}
			got := stripDefaultPort(u)
			if got != tt.want {
				t.Errorf("stripDefaultPort(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestFormatRawRequest(t *testing.T) {
	req, err := http.NewRequest("GET", "https://example.com/path?q=1", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set("User-Agent", "test-agent")
	req.Header.Set("Accept", "text/html")

	got := formatRawRequest(req)

	if !strings.Contains(got, "GET") {
		t.Error("formatRawRequest should contain method")
	}
	if !strings.Contains(got, "/path") {
		t.Error("formatRawRequest should contain path")
	}
	if !strings.Contains(got, "Host:") {
		t.Error("formatRawRequest should contain Host header")
	}
	if !strings.Contains(got, "User-Agent") {
		t.Error("formatRawRequest should contain User-Agent")
	}
	if !strings.Contains(got, "Accept") {
		t.Error("formatRawRequest should contain Accept")
	}
}

func TestFormatRawResponse(t *testing.T) {
	resp := &http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     http.Header{"Content-Type": {"text/html"}},
	}
	got := formatRawResponse(resp)

	if !strings.Contains(got, "HTTP/1.1") {
		t.Error("formatRawResponse should contain protocol")
	}
	if !strings.Contains(got, "200") {
		t.Error("formatRawResponse should contain status code")
	}
	if !strings.Contains(got, "Content-Type") {
		t.Error("formatRawResponse should contain headers")
	}
}

func TestNormalizeHeaders(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Type", "text/html")
	headers.Set("X-Custom-Header", "value")

	got := normalizeHeaders(headers)

	if v, ok := got["content_type"]; !ok || v != "text/html" {
		t.Errorf("normalizeHeaders: content_type = %q (ok=%v), want text/html", v, ok)
	}
	if v, ok := got["x_custom_header"]; !ok || v != "value" {
		t.Errorf("normalizeHeaders: x_custom_header = %q (ok=%v), want value", v, ok)
	}
}

func TestProbeURL_HTTP_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("hello"))
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.AllowPrivateIPs = true
	cfg.Timeout = 5
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	if result.Error != "" {
		t.Errorf("ProbeURL error: %s", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	if result.Protocol != "HTTP/1.1" {
		t.Errorf("Protocol = %q, want HTTP/1.1", result.Protocol)
	}
	if result.Input != server.URL {
		t.Errorf("Input = %q, want %q", result.Input, server.URL)
	}
}

func TestProbeURL_InvalidURL(t *testing.T) {
	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, "http://[invalid", "http://[invalid")

	if result.Error == "" {
		t.Error("ProbeURL should return error for invalid URL")
	}
	if !strings.Contains(result.Error, "Invalid URL") && !strings.Contains(result.Error, "invalid") {
		t.Errorf("error = %q, want to contain 'Invalid URL' or 'invalid'", result.Error)
	}
}

func TestProbeURL_NoSchemePrependsHTTP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	u, _ := url.Parse(server.URL)
	hostPort := u.Host

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.AllowPrivateIPs = true
	cfg.Timeout = 5
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, hostPort, hostPort)

	if result.Error != "" {
		t.Errorf("ProbeURL error: %s", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
}

func TestNewProber_WithResolveIP(t *testing.T) {
	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.ResolveIP = true
	prober := NewProber(cfg)
	defer prober.Close()

	if prober.ipTracker == nil {
		t.Error("NewProber with ResolveIP should set ipTracker")
	}
}

func TestNewProber_WithTechDetect(t *testing.T) {
	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.TechDetect = true
	prober := NewProber(cfg)
	defer prober.Close()

	// techDetector may be nil if wappalyzer init fails, but prober should still be usable
	if prober.config.TechDetect != true {
		t.Error("config.TechDetect should be true")
	}
}

func TestProber_Close_Idempotent(t *testing.T) {
	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	prober := NewProber(cfg)

	err1 := prober.Close()
	err2 := prober.Close()
	if err1 != nil {
		t.Errorf("first Close: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second Close: %v", err2)
	}
}

func TestProbeURL_WithRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Location", "/final")
			w.WriteHeader(http.StatusFound)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("final"))
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.AllowPrivateIPs = true
	cfg.FollowRedirects = true
	cfg.MaxRedirects = 5
	cfg.Timeout = 5
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL+"/", server.URL+"/")

	if result.Error != "" {
		t.Errorf("ProbeURL error: %s", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
}

func TestProbeURL_WithDebug(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("body"))
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.AllowPrivateIPs = true
	cfg.Debug = true
	cfg.Timeout = 5
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	if result.Error != "" {
		t.Errorf("ProbeURL error: %s", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
}

func TestProbeURL_WithIncludeResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("body"))
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.AllowPrivateIPs = true
	cfg.IncludeResponse = true
	cfg.Timeout = 5
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	if result.Error != "" {
		t.Errorf("ProbeURL error: %s", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	if result.RawRequest == "" || result.RawResponse == "" {
		t.Error("IncludeResponse should populate RawRequest and RawResponse")
	}
}

func TestProbeURL_WithStoreResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("stored"))
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.AllowPrivateIPs = true
	cfg.StoreResponse = true
	cfg.StoreResponseDir = t.TempDir()
	cfg.Timeout = 5
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	if result.Error != "" {
		t.Errorf("ProbeURL error: %s", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
}

func TestProbeURL_HTTPS_Success(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.InsecureSkipVerify = true
	cfg.Timeout = 5
	cfg.TLSHandshakeTimeout = 5
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	if result.Error != "" {
		t.Errorf("ProbeURL error: %s", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	if result.TLSVersion == "" {
		t.Error("TLSVersion should be populated for HTTPS")
	}
	if result.TLSConfigStrategy == "" {
		t.Error("TLSConfigStrategy should be populated for HTTPS")
	}
}

func TestProbeURL_HTTPS_WithExtractTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.InsecureSkipVerify = true
	cfg.ExtractTLS = true
	cfg.Timeout = 5
	cfg.TLSHandshakeTimeout = 5
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	if result.Error != "" {
		t.Errorf("ProbeURL error: %s", result.Error)
	}
	if result.TLS == nil || result.TLS.Certificate == nil {
		t.Error("ExtractTLS should populate TLS.Certificate")
	}
}

func TestProbeURL_ContextCancelled(t *testing.T) {
	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	prober := NewProber(cfg)
	defer prober.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := prober.ProbeURL(ctx, "http://example.com", "http://example.com")

	if result.Error == "" {
		t.Error("ProbeURL with cancelled context should return error")
	}
	if !strings.Contains(result.Error, "cancel") {
		t.Errorf("ProbeURL with cancelled context: error = %q, want to contain 'cancel'", result.Error)
	}
}

func TestProbeURL_HTTP_404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.AllowPrivateIPs = true
	cfg.Timeout = 5
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	if result.Error != "" {
		t.Errorf("ProbeURL should not error for 404: %s", result.Error)
	}
	if result.StatusCode != 404 {
		t.Errorf("StatusCode = %d, want 404", result.StatusCode)
	}
}

func TestProbeURL_HTTP_500(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.AllowPrivateIPs = true
	cfg.Timeout = 5
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	if result.Error != "" {
		t.Errorf("ProbeURL should not error for 500: %s", result.Error)
	}
	if result.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", result.StatusCode)
	}
}

func TestProbeURL_WithResolveIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.AllowPrivateIPs = true
	cfg.ResolveIP = true
	cfg.Timeout = 5
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	if result.Error != "" {
		t.Errorf("ProbeURL error: %s", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
	if result.HostIP == "" {
		t.Error("ResolveIP should populate HostIP for localhost")
	}
}

func TestProbeURL_WithTechDetect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Powered-By", "PHP")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><head><title>Test</title></head></html>"))
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.AllowPrivateIPs = true
	cfg.TechDetect = true
	cfg.Timeout = 5
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	if result.Error != "" {
		t.Errorf("ProbeURL error: %s", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("StatusCode = %d, want 200", result.StatusCode)
	}
}

func TestProbeURL_WithDetectHSTS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Strict-Transport-Security", "max-age=31536000")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.AllowPrivateIPs = true
	cfg.DetectHSTS = true
	cfg.Timeout = 5
	prober := NewProber(cfg)
	defer prober.Close()

	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	if result.Error != "" {
		t.Errorf("ProbeURL error: %s", result.Error)
	}
	if !result.HSTS {
		t.Error("DetectHSTS should set HSTS true when header present")
	}
}

func TestProcessURLs_ProcessesURLs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := config.New()
	cfg.Silent = true
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg.AllowPrivateIPs = true
	cfg.Timeout = 5
	cfg.Concurrency = 2
	prober := NewProber(cfg)
	defer prober.Close()

	urls := []string{server.URL, server.URL + "/path"}
	originalInputMap := map[string]string{server.URL: server.URL, server.URL + "/path": server.URL + "/path"}

	ctx := context.Background()
	results := prober.ProcessURLs(ctx, urls, originalInputMap, 2)

	count := 0
	for r := range results {
		count++
		if r.Error != "" {
			t.Errorf("result %d error: %s", count, r.Error)
		}
		if r.StatusCode != 200 {
			t.Errorf("result %d StatusCode = %d, want 200", count, r.StatusCode)
		}
	}
	if count != 2 {
		t.Errorf("expected 2 results, got %d", count)
	}
}
