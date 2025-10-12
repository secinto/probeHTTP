package main

import (
	"reflect"
	"sort"
	"testing"
)

// TestParsePortList tests the parsePortList function
func TestParsePortList(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		want      []string
		wantError bool
	}{
		{
			name:      "single port",
			input:     "80",
			want:      []string{"80"},
			wantError: false,
		},
		{
			name:      "multiple ports",
			input:     "80,443,8080",
			want:      []string{"80", "443", "8080"},
			wantError: false,
		},
		{
			name:      "simple range",
			input:     "8000-8003",
			want:      []string{"8000", "8001", "8002", "8003"},
			wantError: false,
		},
		{
			name:      "mixed ports and ranges",
			input:     "80,443,8000-8002,9000",
			want:      []string{"80", "443", "8000", "8001", "8002", "9000"},
			wantError: false,
		},
		{
			name:      "ports with spaces",
			input:     "80, 443, 8080",
			want:      []string{"80", "443", "8080"},
			wantError: false,
		},
		{
			name:      "duplicate ports",
			input:     "80,443,80,8080,443",
			want:      []string{"80", "443", "8080"},
			wantError: false,
		},
		{
			name:      "overlapping range",
			input:     "80,8000-8002,8001-8003",
			want:      []string{"80", "8000", "8001", "8002", "8003"},
			wantError: false,
		},
		{
			name:      "invalid port - non-numeric",
			input:     "80,abc,443",
			want:      nil,
			wantError: true,
		},
		{
			name:      "invalid port - zero",
			input:     "0,80,443",
			want:      nil,
			wantError: true,
		},
		{
			name:      "invalid port - too high",
			input:     "80,65536,443",
			want:      nil,
			wantError: true,
		},
		{
			name:      "invalid port - negative",
			input:     "80,-1,443",
			want:      nil,
			wantError: true,
		},
		{
			name:      "invalid range - wrong order",
			input:     "8080-8000",
			want:      nil,
			wantError: true,
		},
		{
			name:      "invalid range - malformed",
			input:     "8000-8001-8002",
			want:      nil,
			wantError: true,
		},
		{
			name:      "invalid range - start out of bounds",
			input:     "0-100",
			want:      nil,
			wantError: true,
		},
		{
			name:      "invalid range - end out of bounds",
			input:     "8000-70000",
			want:      nil,
			wantError: true,
		},
		{
			name:      "empty string",
			input:     "",
			want:      nil,
			wantError: true,
		},
		{
			name:      "only whitespace",
			input:     "  ,  , ",
			want:      nil,
			wantError: true,
		},
		{
			name:      "max valid port",
			input:     "65535",
			want:      []string{"65535"},
			wantError: false,
		},
		{
			name:      "min valid port",
			input:     "1",
			want:      []string{"1"},
			wantError: false,
		},
		{
			name:      "large range",
			input:     "8000-8010",
			want:      []string{"8000", "8001", "8002", "8003", "8004", "8005", "8006", "8007", "8008", "8009", "8010"},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parsePortList(tt.input)

			if tt.wantError {
				if err == nil {
					t.Errorf("parsePortList() expected error, got none")
				}
				return
			}

			if err != nil {
				t.Errorf("parsePortList() unexpected error: %v", err)
				return
			}

			// Sort both slices for comparison
			sort.Strings(got)
			sort.Strings(tt.want)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parsePortList() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestParseInputURL tests the parseInputURL function
func TestParseInputURL(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  ParsedURL
	}{
		{
			name:  "full URL with HTTPS",
			input: "https://example.com:443/path/to/resource",
			want: ParsedURL{
				Original: "https://example.com:443/path/to/resource",
				Scheme:   "https",
				Host:     "example.com",
				Port:     "443",
				Path:     "/path/to/resource",
			},
		},
		{
			name:  "full URL with HTTP",
			input: "http://example.com:80/api",
			want: ParsedURL{
				Original: "http://example.com:80/api",
				Scheme:   "http",
				Host:     "example.com",
				Port:     "80",
				Path:     "/api",
			},
		},
		{
			name:  "hostname only",
			input: "example.com",
			want: ParsedURL{
				Original: "example.com",
				Scheme:   "",
				Host:     "example.com",
				Port:     "",
				Path:     "/",
			},
		},
		{
			name:  "hostname with port",
			input: "example.com:8080",
			want: ParsedURL{
				Original: "example.com:8080",
				Scheme:   "",
				Host:     "example.com",
				Port:     "8080",
				Path:     "/",
			},
		},
		{
			name:  "URL with scheme no port",
			input: "https://example.com",
			want: ParsedURL{
				Original: "https://example.com",
				Scheme:   "https",
				Host:     "example.com",
				Port:     "",
				Path:     "/",
			},
		},
		{
			name:  "URL with scheme and path",
			input: "http://example.com/test",
			want: ParsedURL{
				Original: "http://example.com/test",
				Scheme:   "http",
				Host:     "example.com",
				Port:     "",
				Path:     "/test",
			},
		},
		{
			name:  "URL with non-standard port",
			input: "https://example.com:9443/secure",
			want: ParsedURL{
				Original: "https://example.com:9443/secure",
				Scheme:   "https",
				Host:     "example.com",
				Port:     "9443",
				Path:     "/secure",
			},
		},
		{
			name:  "hostname with subdomain",
			input: "api.example.com",
			want: ParsedURL{
				Original: "api.example.com",
				Scheme:   "",
				Host:     "api.example.com",
				Port:     "",
				Path:     "/",
			},
		},
		{
			name:  "hostname with subdomain and port",
			input: "api.example.com:3000",
			want: ParsedURL{
				Original: "api.example.com:3000",
				Scheme:   "",
				Host:     "api.example.com",
				Port:     "3000",
				Path:     "/",
			},
		},
		{
			name:  "URL with query parameters",
			input: "https://example.com/search?q=test",
			want: ParsedURL{
				Original: "https://example.com/search?q=test",
				Scheme:   "https",
				Host:     "example.com",
				Port:     "",
				Path:     "/search?q=test",
			},
		},
		{
			name:  "URL with fragment",
			input: "https://example.com/page#section",
			want: ParsedURL{
				Original: "https://example.com/page#section",
				Scheme:   "https",
				Host:     "example.com",
				Port:     "",
				Path:     "/page#section",
			},
		},
		{
			name:  "localhost",
			input: "localhost:8080",
			want: ParsedURL{
				Original: "localhost:8080",
				Scheme:   "",
				Host:     "localhost",
				Port:     "8080",
				Path:     "/",
			},
		},
		{
			name:  "IP address",
			input: "192.168.1.1:80",
			want: ParsedURL{
				Original: "192.168.1.1:80",
				Scheme:   "",
				Host:     "192.168.1.1",
				Port:     "80",
				Path:     "/",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseInputURL(tt.input)

			if got.Original != tt.want.Original {
				t.Errorf("parseInputURL().Original = %v, want %v", got.Original, tt.want.Original)
			}
			if got.Scheme != tt.want.Scheme {
				t.Errorf("parseInputURL().Scheme = %v, want %v", got.Scheme, tt.want.Scheme)
			}
			if got.Host != tt.want.Host {
				t.Errorf("parseInputURL().Host = %v, want %v", got.Host, tt.want.Host)
			}
			if got.Port != tt.want.Port {
				t.Errorf("parseInputURL().Port = %v, want %v", got.Port, tt.want.Port)
			}
			if got.Path != tt.want.Path {
				t.Errorf("parseInputURL().Path = %v, want %v", got.Path, tt.want.Path)
			}
		})
	}
}

// TestGetSchemesToTest tests the getSchemesToTest function
func TestGetSchemesToTest(t *testing.T) {
	tests := []struct {
		name       string
		parsed     ParsedURL
		allSchemes bool
		want       []string
	}{
		{
			name: "no scheme, default behavior",
			parsed: ParsedURL{
				Scheme: "",
				Host:   "example.com",
			},
			allSchemes: false,
			want:       []string{"http", "https"},
		},
		{
			name: "http scheme, no flag",
			parsed: ParsedURL{
				Scheme: "http",
				Host:   "example.com",
			},
			allSchemes: false,
			want:       []string{"http"},
		},
		{
			name: "https scheme, no flag",
			parsed: ParsedURL{
				Scheme: "https",
				Host:   "example.com",
			},
			allSchemes: false,
			want:       []string{"https"},
		},
		{
			name: "http scheme, all-schemes flag",
			parsed: ParsedURL{
				Scheme: "http",
				Host:   "example.com",
			},
			allSchemes: true,
			want:       []string{"http", "https"},
		},
		{
			name: "https scheme, all-schemes flag",
			parsed: ParsedURL{
				Scheme: "https",
				Host:   "example.com",
			},
			allSchemes: true,
			want:       []string{"http", "https"},
		},
		{
			name: "no scheme, all-schemes flag",
			parsed: ParsedURL{
				Scheme: "",
				Host:   "example.com",
			},
			allSchemes: true,
			want:       []string{"http", "https"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set config for this test
			resetConfig()
			config.AllSchemes = tt.allSchemes

			got := getSchemesToTest(tt.parsed)

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getSchemesToTest() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestGetPortsToTest tests the getPortsToTest function
func TestGetPortsToTest(t *testing.T) {
	tests := []struct {
		name        string
		parsed      ParsedURL
		scheme      string
		ignorePorts bool
		customPorts string
		want        []string
	}{
		{
			name: "no port, http scheme, default",
			parsed: ParsedURL{
				Port: "",
			},
			scheme:      "http",
			ignorePorts: false,
			customPorts: "",
			want:        []string{"80"},
		},
		{
			name: "no port, https scheme, default",
			parsed: ParsedURL{
				Port: "",
			},
			scheme:      "https",
			ignorePorts: false,
			customPorts: "",
			want:        []string{"443"},
		},
		{
			name: "port 8080, no flags",
			parsed: ParsedURL{
				Port: "8080",
			},
			scheme:      "http",
			ignorePorts: false,
			customPorts: "",
			want:        []string{"8080"},
		},
		{
			name: "port 443, ignore-ports, https",
			parsed: ParsedURL{
				Port: "443",
			},
			scheme:      "https",
			ignorePorts: true,
			customPorts: "",
			want:        DefaultHTTPSPorts,
		},
		{
			name: "port 80, ignore-ports, http",
			parsed: ParsedURL{
				Port: "80",
			},
			scheme:      "http",
			ignorePorts: true,
			customPorts: "",
			want:        DefaultHTTPPorts,
		},
		{
			name: "custom ports override everything",
			parsed: ParsedURL{
				Port: "9000",
			},
			scheme:      "http",
			ignorePorts: true,
			customPorts: "80,443,8080",
			want:        []string{"80", "443", "8080"},
		},
		{
			name: "custom ports with range",
			parsed: ParsedURL{
				Port: "",
			},
			scheme:      "https",
			ignorePorts: false,
			customPorts: "8000-8002",
			want:        []string{"8000", "8001", "8002"},
		},
		{
			name: "no port, no flags, http",
			parsed: ParsedURL{
				Port: "",
			},
			scheme:      "http",
			ignorePorts: false,
			customPorts: "",
			want:        []string{"80"},
		},
		{
			name: "no port, no flags, https",
			parsed: ParsedURL{
				Port: "",
			},
			scheme:      "https",
			ignorePorts: false,
			customPorts: "",
			want:        []string{"443"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set config for this test
			resetConfig()
			config.IgnorePorts = tt.ignorePorts
			config.CustomPorts = tt.customPorts

			got := getPortsToTest(tt.parsed, tt.scheme)

			// Sort for comparison
			sort.Strings(got)
			want := make([]string, len(tt.want))
			copy(want, tt.want)
			sort.Strings(want)

			if !reflect.DeepEqual(got, want) {
				t.Errorf("getPortsToTest() = %v, want %v", got, want)
			}
		})
	}
}

// TestExpandURLs tests the expandURLs function
func TestExpandURLs(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		allSchemes  bool
		ignorePorts bool
		customPorts string
		wantCount   int
		wantContain []string
	}{
		{
			name:        "hostname only - default behavior",
			input:       "example.com",
			allSchemes:  false,
			ignorePorts: false,
			customPorts: "",
			wantCount:   2,
			wantContain: []string{
				"http://example.com/",
				"https://example.com/",
			},
		},
		{
			name:        "hostname with port - default behavior",
			input:       "example.com:8080",
			allSchemes:  false,
			ignorePorts: false,
			customPorts: "",
			wantCount:   2,
			wantContain: []string{
				"http://example.com:8080/",
				"https://example.com:8080/",
			},
		},
		{
			name:        "https URL - use only https",
			input:       "https://example.com",
			allSchemes:  false,
			ignorePorts: false,
			customPorts: "",
			wantCount:   1,
			wantContain: []string{
				"https://example.com/",
			},
		},
		{
			name:        "http URL - use only http",
			input:       "http://example.com",
			allSchemes:  false,
			ignorePorts: false,
			customPorts: "",
			wantCount:   1,
			wantContain: []string{
				"http://example.com/",
			},
		},
		{
			name:        "https URL with all-schemes",
			input:       "https://example.com:443",
			allSchemes:  true,
			ignorePorts: false,
			customPorts: "",
			wantCount:   2,
			wantContain: []string{
				"http://example.com:443/",
				"https://example.com:443/",
			},
		},
		{
			name:        "hostname with ignore-ports",
			input:       "example.com:9999",
			allSchemes:  false,
			ignorePorts: true,
			customPorts: "",
			wantCount:   8, // 4 HTTP ports + 4 HTTPS ports
			wantContain: []string{
				"http://example.com:80/",
				"http://example.com:8080/",
				"https://example.com:443/",
				"https://example.com:8443/",
			},
		},
		{
			name:        "hostname with custom ports",
			input:       "example.com",
			allSchemes:  false,
			ignorePorts: false,
			customPorts: "80,443,8080",
			wantCount:   6, // 2 schemes × 3 ports
			wantContain: []string{
				"http://example.com:80/",
				"http://example.com:443/",
				"http://example.com:8080/",
				"https://example.com:80/",
				"https://example.com:443/",
				"https://example.com:8080/",
			},
		},
		{
			name:        "all-schemes + ignore-ports",
			input:       "https://example.com:443",
			allSchemes:  true,
			ignorePorts: true,
			customPorts: "",
			wantCount:   8, // 2 schemes × 4 common ports each
			wantContain: []string{
				"http://example.com:80/",
				"https://example.com:443/",
			},
		},
		{
			name:        "URL with path",
			input:       "https://example.com/api/v1",
			allSchemes:  false,
			ignorePorts: false,
			customPorts: "",
			wantCount:   1,
			wantContain: []string{
				"https://example.com/api/v1",
			},
		},
		{
			name:        "URL with path and all-schemes",
			input:       "https://example.com/test",
			allSchemes:  true,
			ignorePorts: false,
			customPorts: "",
			wantCount:   2,
			wantContain: []string{
				"http://example.com/test",
				"https://example.com/test",
			},
		},
		{
			name:        "custom ports with range",
			input:       "example.com",
			allSchemes:  false,
			ignorePorts: false,
			customPorts: "8000-8002",
			wantCount:   6, // 2 schemes × 3 ports
			wantContain: []string{
				"http://example.com:8000/",
				"http://example.com:8001/",
				"http://example.com:8002/",
				"https://example.com:8000/",
				"https://example.com:8001/",
				"https://example.com:8002/",
			},
		},
		{
			name:        "subdomain",
			input:       "api.example.com",
			allSchemes:  false,
			ignorePorts: false,
			customPorts: "",
			wantCount:   2,
			wantContain: []string{
				"http://api.example.com/",
				"https://api.example.com/",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set config for this test
			resetConfig()
			config.AllSchemes = tt.allSchemes
			config.IgnorePorts = tt.ignorePorts
			config.CustomPorts = tt.customPorts

			got := expandURLs(tt.input)

			if len(got) != tt.wantCount {
				t.Errorf("expandURLs() returned %d URLs, want %d\nGot: %v", len(got), tt.wantCount, got)
			}

			// Check that all expected URLs are present
			for _, wantURL := range tt.wantContain {
				found := false
				for _, gotURL := range got {
					if gotURL == wantURL {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expandURLs() missing expected URL: %s\nGot: %v", wantURL, got)
				}
			}

			// Check for duplicates
			urlMap := make(map[string]bool)
			for _, url := range got {
				if urlMap[url] {
					t.Errorf("expandURLs() contains duplicate URL: %s", url)
				}
				urlMap[url] = true
			}
		})
	}
}

// TestExpandURLs_Deduplication tests that expandURLs properly deduplicates
func TestExpandURLs_Deduplication(t *testing.T) {
	resetConfig()
	config.AllSchemes = false
	config.IgnorePorts = false
	config.CustomPorts = ""

	// Test that same URL isn't duplicated
	input := "http://example.com:80"
	got := expandURLs(input)

	if len(got) != 1 {
		t.Errorf("expandURLs() should not create duplicates, got %d URLs: %v", len(got), got)
	}
}

// TestExpandURLs_ErrorHandling tests error cases in URL expansion
func TestExpandURLs_ErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		customPorts string
		wantCount   int // Should still return some URLs even with errors
	}{
		{
			name:        "invalid custom ports - should use defaults",
			input:       "example.com",
			customPorts: "invalid",
			wantCount:   2, // Falls back to default behavior
		},
		{
			name:        "empty custom ports - should use defaults",
			input:       "example.com",
			customPorts: "",
			wantCount:   2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConfig()
			config.CustomPorts = tt.customPorts
			config.Silent = true // Suppress error messages in tests

			got := expandURLs(tt.input)

			if len(got) != tt.wantCount {
				t.Errorf("expandURLs() with error should return %d URLs, got %d: %v", tt.wantCount, len(got), got)
			}
		})
	}
}
