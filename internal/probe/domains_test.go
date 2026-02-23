package probe

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"sort"
	"testing"
	"time"
)

func TestDiscoverDomains_NilInputs(t *testing.T) {
	result := DiscoverDomains(nil, http.Header{}, "example.com")
	if result != nil {
		t.Errorf("expected nil for nil TLS state and no CSP, got %v", result)
	}
}

func TestDiscoverDomains_CertificateOnly(t *testing.T) {
	now := time.Now()
	cert, _ := newSelfSignedCert(t, "example.com",
		[]string{"example.com", "www.example.com", "api.example.com"},
		now.Add(-24*time.Hour), now.Add(365*24*time.Hour))

	connState := &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}

	result := DiscoverDomains(connState, http.Header{}, "example.com")
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Domains) != 3 {
		t.Errorf("domain count = %d, want 3; got %v", len(result.Domains), result.Domains)
	}

	// CN is also a SAN, so source should be "san" (SAN takes priority)
	if src, ok := result.DomainSources["example.com"]; !ok || src != "san" {
		t.Errorf("expected source 'san' for example.com, got %q", src)
	}

	// www and api should be SANs
	if src := result.DomainSources["www.example.com"]; src != "san" {
		t.Errorf("expected source 'san' for www.example.com, got %q", src)
	}

	// NewDomains should not include the input host
	for _, d := range result.NewDomains {
		if d == "example.com" {
			t.Error("input host should not appear in NewDomains")
		}
	}

	if len(result.NewDomains) != 2 {
		t.Errorf("new domain count = %d, want 2; got %v", len(result.NewDomains), result.NewDomains)
	}
}

func TestDiscoverDomains_CNOnlyNoDuplicateSAN(t *testing.T) {
	now := time.Now()
	// CN not in SANs list
	cert, _ := newSelfSignedCert(t, "main.example.com",
		[]string{"www.example.com"},
		now.Add(-24*time.Hour), now.Add(365*24*time.Hour))

	connState := &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}

	result := DiscoverDomains(connState, http.Header{}, "other.com")
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if src := result.DomainSources["main.example.com"]; src != "cn" {
		t.Errorf("expected source 'cn' for main.example.com, got %q", src)
	}

	if src := result.DomainSources["www.example.com"]; src != "san" {
		t.Errorf("expected source 'san' for www.example.com, got %q", src)
	}
}

func TestDiscoverDomains_CSPAndCert(t *testing.T) {
	now := time.Now()
	cert, _ := newSelfSignedCert(t, "example.com",
		[]string{"example.com", "www.example.com"},
		now.Add(-24*time.Hour), now.Add(365*24*time.Hour))

	connState := &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}

	headers := http.Header{}
	headers.Set("Content-Security-Policy", "script-src cdn.example.com api.example.com")

	result := DiscoverDomains(connState, headers, "example.com")
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Should have: example.com (san), www.example.com (san), cdn.example.com (csp), api.example.com (csp)
	if len(result.Domains) != 4 {
		t.Errorf("domain count = %d, want 4; got %v", len(result.Domains), result.Domains)
	}

	if src := result.DomainSources["cdn.example.com"]; src != "csp" {
		t.Errorf("expected source 'csp' for cdn.example.com, got %q", src)
	}
}

func TestDiscoverDomains_CSPOnlyNoTLS(t *testing.T) {
	headers := http.Header{}
	headers.Set("Content-Security-Policy", "default-src cdn.example.com")

	result := DiscoverDomains(nil, headers, "example.com")
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if len(result.Domains) != 1 || result.Domains[0] != "cdn.example.com" {
		t.Errorf("expected [cdn.example.com], got %v", result.Domains)
	}

	if src := result.DomainSources["cdn.example.com"]; src != "csp" {
		t.Errorf("expected source 'csp', got %q", src)
	}
}

func TestDiscoverDomains_SortedOutput(t *testing.T) {
	now := time.Now()
	cert, _ := newSelfSignedCert(t, "z-domain.com",
		[]string{"b-domain.com", "a-domain.com", "z-domain.com"},
		now.Add(-24*time.Hour), now.Add(365*24*time.Hour))

	connState := &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}

	result := DiscoverDomains(connState, http.Header{}, "other.com")
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	if !sort.StringsAreSorted(result.Domains) {
		t.Errorf("domains not sorted: %v", result.Domains)
	}
}
