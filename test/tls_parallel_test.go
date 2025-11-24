package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"probeHTTP/internal/probe"
)

// TestTLSStrategies tests that TLS strategies are correctly defined
func TestTLSStrategies(t *testing.T) {
	batch1, batch2 := probe.GetTLSStrategies()

	// Verify Batch 1 has 3 strategies
	if len(batch1) != 3 {
		t.Errorf("expected 3 strategies in Batch 1, got %d", len(batch1))
	}

	// Verify Batch 2 has 2 strategies
	if len(batch2) != 2 {
		t.Errorf("expected 2 strategies in Batch 2, got %d", len(batch2))
	}

	// Verify Batch 1 strategies
	expectedBatch1 := []string{"TLS 1.3", "TLS 1.2 Secure", "TLS 1.2 Compatible"}
	for i, expected := range expectedBatch1 {
		if batch1[i].Name != expected {
			t.Errorf("Batch 1[%d]: expected %s, got %s", i, expected, batch1[i].Name)
		}
	}

	// Verify Batch 2 strategies
	expectedBatch2 := []string{"TLS 1.1", "TLS 1.0"}
	for i, expected := range expectedBatch2 {
		if batch2[i].Name != expected {
			t.Errorf("Batch 2[%d]: expected %s, got %s", i, expected, batch2[i].Name)
		}
	}
}

// TestBuildTLSConfig tests TLS config building
func TestBuildTLSConfig(t *testing.T) {
	cfg := resetConfig()
	batch1, _ := probe.GetTLSStrategies()

	for _, strategy := range batch1 {
		tlsConfig := probe.BuildTLSConfig(strategy, cfg)
		if tlsConfig == nil {
			t.Errorf("BuildTLSConfig returned nil for strategy %s", strategy.Name)
		}
		if tlsConfig.MinVersion != strategy.MinVersion {
			t.Errorf("MinVersion mismatch for %s: expected %d, got %d", strategy.Name, strategy.MinVersion, tlsConfig.MinVersion)
		}
		if tlsConfig.MaxVersion != strategy.MaxVersion {
			t.Errorf("MaxVersion mismatch for %s: expected %d, got %d", strategy.Name, strategy.MaxVersion, tlsConfig.MaxVersion)
		}
	}
}

// TestHTTPSProbe tests HTTPS probing with parallel TLS attempts
func TestHTTPSProbe(t *testing.T) {
	// Create a test server with TLS
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><head><title>Test</title></head><body>Test</body></html>"))
	}))
	defer server.Close()

	cfg := resetConfig()
	cfg.Silent = true
	cfg.InsecureSkipVerify = true // Allow self-signed cert

	prober := probe.NewProber(cfg)
	ctx := context.Background()

	// Probe HTTPS URL
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	// Verify no error
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}

	// Verify TLS metadata is populated
	if result.TLSVersion == "" {
		t.Error("TLSVersion should be populated for HTTPS URLs")
	}
	if result.Protocol == "" {
		t.Error("Protocol should be populated")
	}
	if result.TLSConfigStrategy == "" {
		t.Error("TLSConfigStrategy should be populated")
	}

	// Verify protocol is one of the expected values
	expectedProtocols := []string{"HTTP/1.1", "HTTP/2", "HTTP/3"}
	protocolFound := false
	for _, proto := range expectedProtocols {
		if result.Protocol == proto {
			protocolFound = true
			break
		}
	}
	if !protocolFound && result.Protocol != "" {
		t.Errorf("unexpected protocol: %s", result.Protocol)
	}
}

// TestHTTPProbe tests HTTP probing (no TLS)
func TestHTTPProbe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("<html><head><title>Test</title></head><body>Test</body></html>"))
	}))
	defer server.Close()

	cfg := resetConfig()
	cfg.Silent = true

	prober := probe.NewProber(cfg)
	ctx := context.Background()

	// Probe HTTP URL
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	// Verify no error
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}

	// Verify protocol is HTTP/1.1 for non-HTTPS
	if result.Protocol != "HTTP/1.1" {
		t.Errorf("expected HTTP/1.1 for HTTP URL, got %s", result.Protocol)
	}

	// TLS fields should be empty for HTTP
	if result.TLSVersion != "" {
		t.Errorf("TLSVersion should be empty for HTTP URLs, got %s", result.TLSVersion)
	}
	if result.TLSConfigStrategy != "" {
		t.Errorf("TLSConfigStrategy should be empty for HTTP URLs, got %s", result.TLSConfigStrategy)
	}
}

// TestTLSClientCreation tests client creation methods
func TestTLSClientCreation(t *testing.T) {
	cfg := resetConfig()
	batch1, _ := probe.GetTLSStrategies()

	for _, strategy := range batch1 {
		tlsConfig := probe.BuildTLSConfig(strategy, cfg)

		// Test HTTP/1.1 client
		client11 := probe.NewHTTP11Client(cfg, tlsConfig)
		if client11 == nil {
			t.Errorf("NewHTTP11Client returned nil for strategy %s", strategy.Name)
		}

		// Test HTTP/2 client
		client2 := probe.NewHTTP2Client(cfg, tlsConfig)
		if client2 == nil {
			t.Errorf("NewHTTP2Client returned nil for strategy %s", strategy.Name)
		}

		// Test HTTP/3 client
		client3 := probe.NewHTTP3Client(cfg, tlsConfig)
		if client3 == nil {
			t.Errorf("NewHTTP3Client returned nil for strategy %s", strategy.Name)
		}
	}
}

// TestTLSVersionString tests TLS version string conversion
func TestTLSVersionString(t *testing.T) {
	// Note: getTLSVersionString is not exported, so we test indirectly through probing
	// This test verifies the functionality works end-to-end
	cfg := resetConfig()
	cfg.Silent = true
	cfg.InsecureSkipVerify = true

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	prober := probe.NewProber(cfg)
	ctx := context.Background()
	result := prober.ProbeURL(ctx, server.URL, server.URL)

	if result.Error == "" && result.TLSVersion != "" {
		// Verify TLS version is a valid format
		validVersions := []string{"1.3", "1.2", "1.1", "1.0"}
		valid := false
		for _, v := range validVersions {
			if result.TLSVersion == v {
				valid = true
				break
			}
		}
		if !valid && len(result.TLSVersion) > 0 {
			t.Errorf("unexpected TLS version format: %s", result.TLSVersion)
		}
	}
}
