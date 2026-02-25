package probe

import (
	"fmt"
	"testing"

	"probeHTTP/internal/config"
)

func TestNewClient(t *testing.T) {
	cfg := config.New()
	cfg.Silent = true
	client := NewClient(cfg)
	if client == nil {
		t.Fatal("NewClient returned nil")
	}
	if client.GetHTTPClient() == nil {
		t.Error("GetHTTPClient returned nil")
	}
}

func TestGetLimiter_ReturnsSameLimiterForSameHost(t *testing.T) {
	cfg := config.New()
	cfg.Silent = true
	client := NewClient(cfg)

	lim1 := client.GetLimiter("example.com")
	lim2 := client.GetLimiter("example.com")
	if lim1 != lim2 {
		t.Error("GetLimiter should return same limiter for same host")
	}
}

func TestGetLimiter_ReturnsDifferentLimitersForDifferentHosts(t *testing.T) {
	cfg := config.New()
	cfg.Silent = true
	client := NewClient(cfg)

	lim1 := client.GetLimiter("host1.com")
	lim2 := client.GetLimiter("host2.com")
	if lim1 == lim2 {
		t.Error("GetLimiter should return different limiters for different hosts")
	}
}

func TestGetLimiter_UsesConfigRateAndBurst(t *testing.T) {
	cfg := config.New()
	cfg.Silent = true
	cfg.RateLimitPerHost = 5
	cfg.RateLimitBurst = 3
	client := NewClient(cfg)

	lim := client.GetLimiter("example.com")
	if lim == nil {
		t.Fatal("GetLimiter returned nil")
	}
	// With burst 3, we should be able to Allow() 3 times immediately
	for i := 0; i < 3; i++ {
		if !lim.Allow() {
			t.Errorf("Allow() should succeed for burst %d", i+1)
		}
	}
	// 4th call should fail (no tokens, need to wait)
	if lim.Allow() {
		t.Error("4th Allow() should fail when burst exhausted")
	}
}

func TestGetLimiter_ClampsInvalidConfig(t *testing.T) {
	cfg := config.New()
	cfg.Silent = true
	cfg.RateLimitPerHost = 0
	cfg.RateLimitBurst = 0
	client := NewClient(cfg)

	lim := client.GetLimiter("example.com")
	if lim == nil {
		t.Fatal("GetLimiter returned nil")
	}
	// Should still work (clamped to 1 req/s, burst 1)
	if !lim.Allow() {
		t.Error("Allow() should succeed at least once with clamped config")
	}
}

func TestGetLimiter_EvictionWhenOverCapacity(t *testing.T) {
	cfg := config.New()
	cfg.Silent = true
	client := NewClient(cfg)

	// Create more limiters than maxLimiters (10000) to trigger eviction
	// Use unique hosts to force creation of new limiters
	for i := 0; i < maxLimiters+100; i++ {
		host := fmt.Sprintf("host-%d.example.com", i)
		lim := client.GetLimiter(host)
		if lim == nil {
			t.Fatalf("GetLimiter returned nil for host %s", host)
		}
	}
	// Should not panic; eviction should have run
	// Verify we can still get limiters
	lim := client.GetLimiter("new-host-after-eviction.example.com")
	if lim == nil {
		t.Error("GetLimiter should work after eviction")
	}
}
