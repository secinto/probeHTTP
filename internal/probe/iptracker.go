package probe

import (
	"context"
	"net"
	"sync"
)

// IPTracker records resolved IP addresses during DNS resolution
type IPTracker struct {
	ips sync.Map // hostname -> IP string
}

// NewIPTracker creates a new IPTracker
func NewIPTracker() *IPTracker {
	return &IPTracker{}
}

// GetIP returns the resolved IP for a hostname, or empty string if not found
func (t *IPTracker) GetIP(hostname string) string {
	if val, ok := t.ips.Load(hostname); ok {
		return val.(string)
	}
	return ""
}

// DialContext returns a custom DialContext function that records resolved IPs
func (t *IPTracker) DialContext(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			host = addr
		}

		conn, err := dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		// Record the resolved IP from the connection's remote address
		if remoteAddr := conn.RemoteAddr(); remoteAddr != nil {
			ip, _, splitErr := net.SplitHostPort(remoteAddr.String())
			if splitErr == nil {
				t.ips.Store(host, ip)
				// Also store with port suffix for unique lookups
				if port != "" {
					t.ips.Store(host+":"+port, ip)
				}
			}
		}

		return conn, nil
	}
}
