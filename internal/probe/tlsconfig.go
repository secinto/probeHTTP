package probe

import (
	"crypto/tls"

	"probeHTTP/internal/config"
)

// TLSStrategy represents a TLS configuration strategy
type TLSStrategy struct {
	Name         string
	MinVersion   uint16
	MaxVersion   uint16
	CipherSuites []uint16
}

// GetTLSStrategies returns the two batches of TLS strategies
// Batch 1: Modern TLS configurations (TLS 1.3, TLS 1.2 Secure, TLS 1.2 Compatible)
// Batch 2: Legacy TLS configurations (TLS 1.1, TLS 1.0) - only used if Batch 1 fails
func GetTLSStrategies() (batch1 []TLSStrategy, batch2 []TLSStrategy) {
	// Batch 1: Modern TLS configurations
	batch1 = []TLSStrategy{
		{
			Name:       "TLS 1.3",
			MinVersion: tls.VersionTLS13,
			MaxVersion: tls.VersionTLS13,
			// TLS 1.3 cipher suites are auto-selected by Go
			CipherSuites: nil,
		},
		{
			Name:       "TLS 1.2 Secure",
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				// ECDHE with AES-GCM
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				// ECDHE with ChaCha20-Poly1305
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			},
		},
		{
			Name:       "TLS 1.2 Compatible",
			MinVersion: tls.VersionTLS12,
			MaxVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				// All from TLS 1.2 Secure
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				// ECDHE with AES-CBC
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				// RSA with AES-GCM
				tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
				// RSA with AES-CBC
				tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			},
		},
	}

	// Batch 2: Legacy TLS configurations (only used if Batch 1 fails)
	batch2 = []TLSStrategy{
		{
			Name:       "TLS 1.1",
			MinVersion: tls.VersionTLS11,
			MaxVersion: tls.VersionTLS11,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA, // Weak, but sometimes needed for legacy compatibility
			},
		},
		{
			Name:       "TLS 1.0",
			MinVersion: tls.VersionTLS10,
			MaxVersion: tls.VersionTLS10,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_AES_128_CBC_SHA,
				tls.TLS_RSA_WITH_AES_256_CBC_SHA,
				tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA, // Weak, but sometimes needed for legacy compatibility
			},
		},
	}

	return batch1, batch2
}

// BuildTLSConfig creates a tls.Config from a TLS strategy and config
func BuildTLSConfig(strategy TLSStrategy, cfg *config.Config) *tls.Config {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
		MinVersion:         strategy.MinVersion,
		MaxVersion:         strategy.MaxVersion,
	}

	// Only set cipher suites if specified (TLS 1.3 auto-selects)
	if strategy.CipherSuites != nil && len(strategy.CipherSuites) > 0 {
		tlsConfig.CipherSuites = strategy.CipherSuites
	}

	return tlsConfig
}
