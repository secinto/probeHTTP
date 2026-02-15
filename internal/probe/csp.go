package probe

import (
	"net/http"
	"net/url"
	"strings"
)

// cspDirectives lists CSP directives that can contain domain sources.
var cspDirectives = []string{
	"default-src",
	"script-src",
	"style-src",
	"img-src",
	"connect-src",
	"font-src",
	"frame-src",
	"media-src",
	"object-src",
	"form-action",
	"frame-ancestors",
	"child-src",
	"worker-src",
	"manifest-src",
}

// cspKeywords are CSP source values that are not domains.
var cspKeywords = map[string]bool{
	"'self'":              true,
	"'unsafe-inline'":     true,
	"'unsafe-eval'":       true,
	"'unsafe-hashes'":     true,
	"'strict-dynamic'":    true,
	"'report-sample'":     true,
	"'none'":              true,
	"'nonce'":             true,
	"'wasm-unsafe-eval'":  true,
	"data:":               true,
	"blob:":               true,
	"mediastream:":        true,
	"filesystem:":         true,
	"https:":              true,
	"http:":               true,
	"wss:":                true,
	"ws:":                 true,
	"*":                   true,
}

// ExtractCSPDomains parses the Content-Security-Policy header and extracts
// domain names from all directives that can contain source lists.
func ExtractCSPDomains(headers http.Header) []string {
	csp := headers.Get("Content-Security-Policy")
	if csp == "" {
		return nil
	}

	seen := make(map[string]bool)
	var domains []string

	// CSP directives are separated by semicolons
	directives := strings.Split(csp, ";")
	for _, directive := range directives {
		directive = strings.TrimSpace(directive)
		if directive == "" {
			continue
		}

		parts := strings.Fields(directive)
		if len(parts) < 2 {
			continue
		}

		directiveName := strings.ToLower(parts[0])
		if !isRelevantDirective(directiveName) {
			continue
		}

		// Extract domains from source values (skip directive name)
		for _, source := range parts[1:] {
			source = strings.TrimSpace(source)
			domain := extractDomainFromCSPSource(source)
			if domain != "" && !seen[domain] {
				seen[domain] = true
				domains = append(domains, domain)
			}
		}
	}

	return domains
}

// isRelevantDirective checks if a CSP directive name can contain domain sources.
func isRelevantDirective(name string) bool {
	for _, d := range cspDirectives {
		if name == d {
			return true
		}
	}
	return false
}

// extractDomainFromCSPSource extracts a domain name from a CSP source value.
// Returns empty string for keywords, data URIs, and invalid values.
func extractDomainFromCSPSource(source string) string {
	lower := strings.ToLower(source)

	// Skip CSP keywords
	if cspKeywords[lower] {
		return ""
	}

	// Skip nonce values
	if strings.HasPrefix(lower, "'nonce-") {
		return ""
	}

	// Skip hash values
	if strings.HasPrefix(lower, "'sha256-") || strings.HasPrefix(lower, "'sha384-") || strings.HasPrefix(lower, "'sha512-") {
		return ""
	}

	// If it looks like a URL, extract the host
	if strings.Contains(source, "://") {
		if u, err := url.Parse(source); err == nil && u.Host != "" {
			return stripPort(u.Hostname())
		}
		return ""
	}

	// Strip leading wildcard subdomain prefix for domain detection
	// but keep the result (e.g., "*.example.com" -> "*.example.com")
	domain := source

	// Strip any path component
	if idx := strings.Index(domain, "/"); idx != -1 {
		domain = domain[:idx]
	}

	// Strip port
	domain = stripPort(domain)

	// Validate: must contain a dot (unless it's a wildcard)
	if !strings.Contains(domain, ".") && !strings.HasPrefix(domain, "*.") {
		return ""
	}

	return domain
}

// stripPort removes a port suffix from a host string.
func stripPort(host string) string {
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		// Make sure it's not part of an IPv6 address
		if !strings.Contains(host, "]") {
			return host[:idx]
		}
	}
	return host
}
