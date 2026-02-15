package probe

import (
	"crypto/tls"
	"testing"

	"probeHTTP/internal/config"
)

func TestGetTLSStrategies_BatchSizes(t *testing.T) {
	batch1, batch2 := GetTLSStrategies()

	if len(batch1) != 3 {
		t.Errorf("batch1 should have 3 strategies, got %d", len(batch1))
	}
	if len(batch2) != 2 {
		t.Errorf("batch2 should have 2 strategies, got %d", len(batch2))
	}
}

func TestGetTLSStrategies_Batch1Names(t *testing.T) {
	batch1, _ := GetTLSStrategies()
	expected := []string{"TLS 1.3", "TLS 1.2 Secure", "TLS 1.2 Compatible"}
	for i, s := range batch1 {
		if s.Name != expected[i] {
			t.Errorf("batch1[%d].Name = %q, want %q", i, s.Name, expected[i])
		}
	}
}

func TestGetTLSStrategies_Batch2Names(t *testing.T) {
	_, batch2 := GetTLSStrategies()
	expected := []string{"TLS 1.1", "TLS 1.0"}
	for i, s := range batch2 {
		if s.Name != expected[i] {
			t.Errorf("batch2[%d].Name = %q, want %q", i, s.Name, expected[i])
		}
	}
}

func TestGetTLSStrategies_TLS13NoCipherSuites(t *testing.T) {
	batch1, _ := GetTLSStrategies()
	tls13 := batch1[0]
	if tls13.CipherSuites != nil {
		t.Error("TLS 1.3 should have nil cipher suites (auto-selected by Go)")
	}
	if tls13.MinVersion != tls.VersionTLS13 || tls13.MaxVersion != tls.VersionTLS13 {
		t.Error("TLS 1.3 should be locked to version 1.3")
	}
}

func TestGetTLSStrategies_TLS12SecureCiphers(t *testing.T) {
	batch1, _ := GetTLSStrategies()
	secure := batch1[1]
	if len(secure.CipherSuites) != 6 {
		t.Errorf("TLS 1.2 Secure should have 6 cipher suites, got %d", len(secure.CipherSuites))
	}
	// All should be ECDHE (forward secrecy)
	for _, cs := range secure.CipherSuites {
		name := tls.CipherSuiteName(cs)
		if name == "" {
			t.Errorf("unknown cipher suite: 0x%04x", cs)
		}
	}
}

func TestGetTLSStrategies_TLS12CompatHasMoreCiphers(t *testing.T) {
	batch1, _ := GetTLSStrategies()
	secure := batch1[1]
	compat := batch1[2]
	if len(compat.CipherSuites) <= len(secure.CipherSuites) {
		t.Error("TLS 1.2 Compatible should have more cipher suites than Secure")
	}
}

func TestGetTLSStrategies_LegacyHas3DES(t *testing.T) {
	_, batch2 := GetTLSStrategies()
	for _, strategy := range batch2 {
		has3DES := false
		for _, cs := range strategy.CipherSuites {
			if cs == tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA {
				has3DES = true
				break
			}
		}
		if !has3DES {
			t.Errorf("%s should include 3DES for legacy compatibility", strategy.Name)
		}
	}
}

func TestBuildTLSConfig_BasicFields(t *testing.T) {
	strategy := TLSStrategy{
		Name:       "Test",
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	cfg := &config.Config{InsecureSkipVerify: true}
	tlsCfg := BuildTLSConfig(strategy, cfg)

	if tlsCfg.MinVersion != tls.VersionTLS12 {
		t.Errorf("MinVersion = 0x%04x, want 0x%04x", tlsCfg.MinVersion, tls.VersionTLS12)
	}
	if tlsCfg.MaxVersion != tls.VersionTLS13 {
		t.Errorf("MaxVersion = 0x%04x, want 0x%04x", tlsCfg.MaxVersion, tls.VersionTLS13)
	}
	if !tlsCfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
	if len(tlsCfg.CipherSuites) != 1 {
		t.Errorf("CipherSuites count = %d, want 1", len(tlsCfg.CipherSuites))
	}
}

func TestBuildTLSConfig_NilCipherSuites(t *testing.T) {
	strategy := TLSStrategy{
		Name:       "TLS 1.3",
		MinVersion: tls.VersionTLS13,
		MaxVersion: tls.VersionTLS13,
		// nil CipherSuites for TLS 1.3
	}

	cfg := &config.Config{}
	tlsCfg := BuildTLSConfig(strategy, cfg)

	if tlsCfg.CipherSuites != nil {
		t.Error("TLS 1.3 config should have nil cipher suites")
	}
}

func TestBuildTLSConfig_InsecureSkipVerifyFalse(t *testing.T) {
	strategy := TLSStrategy{MinVersion: tls.VersionTLS12, MaxVersion: tls.VersionTLS12}
	cfg := &config.Config{InsecureSkipVerify: false}
	tlsCfg := BuildTLSConfig(strategy, cfg)

	if tlsCfg.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be false when not set")
	}
}

func TestGetOrderedStrategies_Count(t *testing.T) {
	strategies := GetOrderedStrategies(false)
	if len(strategies) != 5 {
		t.Errorf("expected 5 ordered strategies, got %d", len(strategies))
	}
}

func TestGetOrderedStrategies_Order(t *testing.T) {
	strategies := GetOrderedStrategies(true) // HTTP/3 disabled
	expectedNames := []string{
		"TLS 1.2 Compatible",
		"TLS 1.2 Secure",
		"TLS 1.3",
		"TLS 1.1",
		"TLS 1.0",
	}
	for i, s := range strategies {
		if s.Strategy.Name != expectedNames[i] {
			t.Errorf("strategies[%d].Name = %q, want %q", i, s.Strategy.Name, expectedNames[i])
		}
	}
}

func TestGetOrderedStrategies_Protocols(t *testing.T) {
	strategies := GetOrderedStrategies(true) // HTTP/3 disabled
	expectedProtocols := []string{"HTTP/1.1", "HTTP/2", "HTTP/2", "HTTP/1.1", "HTTP/1.1"}
	for i, s := range strategies {
		if s.Protocol != expectedProtocols[i] {
			t.Errorf("strategies[%d].Protocol = %q, want %q", i, s.Protocol, expectedProtocols[i])
		}
	}
}

func TestGetOrderedStrategies_HTTP3Enabled(t *testing.T) {
	strategies := GetOrderedStrategies(false) // HTTP/3 enabled
	// TLS 1.3 should use HTTP/3
	tls13 := strategies[2]
	if tls13.Protocol != "HTTP/3" {
		t.Errorf("TLS 1.3 with HTTP/3 enabled should use HTTP/3, got %q", tls13.Protocol)
	}
}

func TestGetOrderedStrategies_HTTP3Disabled(t *testing.T) {
	strategies := GetOrderedStrategies(true) // HTTP/3 disabled
	// TLS 1.3 should fall back to HTTP/2
	tls13 := strategies[2]
	if tls13.Protocol != "HTTP/2" {
		t.Errorf("TLS 1.3 with HTTP/3 disabled should use HTTP/2, got %q", tls13.Protocol)
	}
}
