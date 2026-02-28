package probe

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestNewIPTracker(t *testing.T) {
	tracker := NewIPTracker()
	if tracker == nil {
		t.Fatal("NewIPTracker returned nil")
	}
}

func TestIPTracker_GetIP_NotFound(t *testing.T) {
	tracker := NewIPTracker()
	ip := tracker.GetIP("nonexistent.example.com")
	if ip != "" {
		t.Errorf("expected empty string for unknown host, got %q", ip)
	}
}

func TestIPTracker_StoreAndRetrieve(t *testing.T) {
	tracker := NewIPTracker()

	// Manually store an IP (simulating what DialContext does)
	tracker.ips.Store("example.com", "93.184.216.34")

	ip := tracker.GetIP("example.com")
	if ip != "93.184.216.34" {
		t.Errorf("GetIP() = %q, want %q", ip, "93.184.216.34")
	}
}

func TestIPTracker_StoreWithPort(t *testing.T) {
	tracker := NewIPTracker()

	// Store both hostname and hostname:port variants
	tracker.ips.Store("example.com", "93.184.216.34")
	tracker.ips.Store("example.com:443", "93.184.216.34")

	ip1 := tracker.GetIP("example.com")
	ip2 := tracker.GetIP("example.com:443")

	if ip1 != "93.184.216.34" {
		t.Errorf("GetIP(host) = %q, want %q", ip1, "93.184.216.34")
	}
	if ip2 != "93.184.216.34" {
		t.Errorf("GetIP(host:port) = %q, want %q", ip2, "93.184.216.34")
	}
}

func TestIPTracker_ConcurrentAccess(t *testing.T) {
	tracker := NewIPTracker()
	done := make(chan bool, 10)

	// Concurrent writes
	for i := 0; i < 10; i++ {
		go func(n int) {
			tracker.ips.Store("host", "1.2.3.4")
			_ = tracker.GetIP("host")
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	ip := tracker.GetIP("host")
	if ip != "1.2.3.4" {
		t.Errorf("after concurrent access, GetIP() = %q, want %q", ip, "1.2.3.4")
	}
}

func TestIPTracker_DialContextCreation(t *testing.T) {
	tracker := NewIPTracker()
	dialer := &net.Dialer{}
	dialFn := tracker.DialContext(dialer)
	if dialFn == nil {
		t.Fatal("DialContext returned nil function")
	}
}

func TestIPTracker_DialContext_RecordsIP(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tracker := NewIPTracker()
	dialer := &net.Dialer{}
	dialFn := tracker.DialContext(dialer)

	transport := &http.Transport{
		DialContext: dialFn,
	}
	client := &http.Client{Transport: transport}
	defer transport.CloseIdleConnections()

	req, _ := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("client.Do: %v", err)
	}
	resp.Body.Close()

	// server.URL is like "http://127.0.0.1:12345". DialContext uses addr "127.0.0.1:12345"
	// and stores by host "127.0.0.1" and "127.0.0.1:12345"
	u, parseErr := url.Parse(server.URL)
	if parseErr != nil {
		t.Fatalf("url.Parse: %v", parseErr)
	}
	if err != nil {
		t.Fatalf("url.Parse: %v", err)
	}
	host := u.Hostname()
	ip := tracker.GetIP(host)
	if ip == "" {
		ip = tracker.GetIP(u.Host)
	}
	if ip == "" {
		t.Error("DialContext should have recorded IP after connection, GetIP returned empty")
	}
	// Should be 127.0.0.1 or ::1 for localhost
	if ip != "127.0.0.1" && ip != "::1" {
		t.Errorf("GetIP(%q) = %q, want 127.0.0.1 or ::1", host, ip)
	}
}
