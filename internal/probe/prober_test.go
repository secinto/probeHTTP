package probe

import "testing"

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
