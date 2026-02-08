package cdn

import (
	"net/http"
	"strings"
)

// cdnRule defines a CDN detection rule
type cdnRule struct {
	Name        string
	HeaderKey   string
	HeaderValue string // If empty, just check header existence
	CheckFunc   func(http.Header) bool
}

var cdnRules = []cdnRule{
	{Name: "Cloudflare", HeaderKey: "Cf-Ray"},
	{Name: "Cloudflare", HeaderKey: "Server", HeaderValue: "cloudflare"},
	{Name: "CloudFront", HeaderKey: "X-Amz-Cf-Id"},
	{Name: "CloudFront", HeaderKey: "X-Amz-Cf-Pop"},
	{Name: "Fastly", HeaderKey: "X-Fastly-Request-Id"},
	{Name: "Fastly", HeaderKey: "Fastly-Debug-Digest"},
	{Name: "Akamai", HeaderKey: "X-Akamai-Transformed"},
	{Name: "Sucuri", HeaderKey: "X-Sucuri-Id"},
	{Name: "Incapsula", HeaderKey: "X-Iinfo"},
	{Name: "KeyCDN", HeaderKey: "Server", HeaderValue: "keycdn"},
	{Name: "StackPath", HeaderKey: "X-Hw"},
	{
		Name: "CloudFront",
		CheckFunc: func(h http.Header) bool {
			via := h.Get("Via")
			return strings.Contains(strings.ToLower(via), "cloudfront")
		},
	},
	{
		Name: "Varnish",
		CheckFunc: func(h http.Header) bool {
			via := h.Get("Via")
			return strings.Contains(strings.ToLower(via), "varnish")
		},
	},
}

// DetectCDN checks response headers for CDN indicators
func DetectCDN(headers http.Header) (bool, string) {
	for _, rule := range cdnRules {
		if rule.CheckFunc != nil {
			if rule.CheckFunc(headers) {
				return true, rule.Name
			}
			continue
		}

		val := headers.Get(rule.HeaderKey)
		if val == "" {
			continue
		}

		if rule.HeaderValue == "" {
			// Just checking existence
			return true, rule.Name
		}

		if strings.Contains(strings.ToLower(val), strings.ToLower(rule.HeaderValue)) {
			return true, rule.Name
		}
	}

	// Check generic X-CDN header
	if xcdn := headers.Get("X-CDN"); xcdn != "" {
		return true, xcdn
	}

	return false, ""
}
