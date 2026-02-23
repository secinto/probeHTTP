package cdn

import (
	"net/http"
	"testing"
)

func TestDetectCDN_Cloudflare(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		wantCDN  bool
		wantName string
	}{
		{
			"Cf-Ray header",
			http.Header{"Cf-Ray": {"abc123"}},
			true, "Cloudflare",
		},
		{
			"Server cloudflare",
			http.Header{"Server": {"cloudflare"}},
			true, "Cloudflare",
		},
		{
			"Server cloudflare mixed case",
			http.Header{"Server": {"Cloudflare"}},
			true, "Cloudflare",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cdn, name := DetectCDN(tt.headers)
			if cdn != tt.wantCDN || name != tt.wantName {
				t.Errorf("DetectCDN() = (%v, %q), want (%v, %q)", cdn, name, tt.wantCDN, tt.wantName)
			}
		})
	}
}

func TestDetectCDN_CloudFront(t *testing.T) {
	tests := []struct {
		name    string
		headers http.Header
	}{
		{"X-Amz-Cf-Id", http.Header{"X-Amz-Cf-Id": {"abc"}}},
		{"X-Amz-Cf-Pop", http.Header{"X-Amz-Cf-Pop": {"IAD50-C1"}}},
		{"Via cloudfront", http.Header{"Via": {"1.1 abc123.cloudfront.net (CloudFront)"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cdn, name := DetectCDN(tt.headers)
			if !cdn || name != "CloudFront" {
				t.Errorf("DetectCDN() = (%v, %q), want (true, CloudFront)", cdn, name)
			}
		})
	}
}

func TestDetectCDN_OtherProviders(t *testing.T) {
	tests := []struct {
		name     string
		headers  http.Header
		wantName string
	}{
		{"Fastly", http.Header{"X-Fastly-Request-Id": {"abc"}}, "Fastly"},
		{"Fastly debug", http.Header{"Fastly-Debug-Digest": {"abc"}}, "Fastly"},
		{"Akamai", http.Header{"X-Akamai-Transformed": {"abc"}}, "Akamai"},
		{"Sucuri", http.Header{"X-Sucuri-Id": {"abc"}}, "Sucuri"},
		{"Incapsula", http.Header{"X-Iinfo": {"abc"}}, "Incapsula"},
		{"KeyCDN", http.Header{"Server": {"keycdn-engine"}}, "KeyCDN"},
		{"StackPath", http.Header{"X-Hw": {"abc"}}, "StackPath"},
		{"Varnish via", http.Header{"Via": {"1.1 varnish (Varnish/6.0)"}}, "Varnish"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cdn, name := DetectCDN(tt.headers)
			if !cdn || name != tt.wantName {
				t.Errorf("DetectCDN() = (%v, %q), want (true, %q)", cdn, name, tt.wantName)
			}
		})
	}
}

func TestDetectCDN_GenericXCDN(t *testing.T) {
	// Use Set() to ensure canonical header key form
	headers := http.Header{}
	headers.Set("X-CDN", "CustomCDN")
	cdn, name := DetectCDN(headers)
	if !cdn || name != "CustomCDN" {
		t.Errorf("DetectCDN() = (%v, %q), want (true, CustomCDN)", cdn, name)
	}
}

func TestDetectCDN_NoCDN(t *testing.T) {
	headers := http.Header{
		"Server":       {"Apache/2.4"},
		"Content-Type": {"text/html"},
	}
	cdn, name := DetectCDN(headers)
	if cdn || name != "" {
		t.Errorf("DetectCDN() = (%v, %q), want (false, \"\")", cdn, name)
	}
}

func TestDetectCDN_EmptyHeaders(t *testing.T) {
	cdn, name := DetectCDN(http.Header{})
	if cdn || name != "" {
		t.Errorf("DetectCDN() = (%v, %q), want (false, \"\")", cdn, name)
	}
}

func TestDetectCDN_HeaderExistenceCheck(t *testing.T) {
	// Cf-Ray with empty value — header exists but empty
	// http.Header.Get returns "" for missing headers, so this tests existence vs value check
	headers := http.Header{"Cf-Ray": {""}}
	// Get("Cf-Ray") returns "" even though header exists, so it won't match
	cdn, _ := DetectCDN(headers)
	if cdn {
		t.Error("empty header value should not trigger CDN detection")
	}
}
