package probe

import "testing"

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
	// DialContext returns a function, verify it's not nil
	dialFn := tracker.DialContext(nil)
	if dialFn == nil {
		t.Fatal("DialContext returned nil function")
	}
}
