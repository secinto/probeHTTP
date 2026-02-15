package probe

import (
	"crypto/tls"
	"net/http"
	"sort"
	"strings"

	"probeHTTP/internal/output"
)

// DiscoverDomains collects domains from the TLS certificate (SANs, CN) and
// CSP headers, deduplicates them, and identifies domains not matching the
// input hostname.
func DiscoverDomains(connState *tls.ConnectionState, headers http.Header, inputHost string) *output.DiscoveredDomains {
	sources := make(map[string]string) // domain -> source
	seen := make(map[string]bool)

	// Extract from certificate SANs
	if connState != nil && len(connState.PeerCertificates) > 0 {
		cert := connState.PeerCertificates[0]

		// SANs (primary source)
		for _, san := range cert.DNSNames {
			san = strings.ToLower(san)
			if !seen[san] {
				seen[san] = true
				sources[san] = "san"
			}
		}

		// Common Name (fallback source)
		cn := strings.ToLower(cert.Subject.CommonName)
		if cn != "" && !seen[cn] {
			seen[cn] = true
			sources[cn] = "cn"
		}
	}

	// Extract from CSP headers
	cspDomains := ExtractCSPDomains(headers)
	for _, domain := range cspDomains {
		domain = strings.ToLower(domain)
		if !seen[domain] {
			seen[domain] = true
			sources[domain] = "csp"
		}
	}

	if len(sources) == 0 {
		return nil
	}

	// Build sorted domain list
	domains := make([]string, 0, len(sources))
	for d := range sources {
		domains = append(domains, d)
	}
	sort.Strings(domains)

	// Identify new domains (not matching the input hostname)
	inputLower := strings.ToLower(inputHost)
	var newDomains []string
	for _, d := range domains {
		if d != inputLower {
			newDomains = append(newDomains, d)
		}
	}

	return &output.DiscoveredDomains{
		Domains:       domains,
		DomainSources: sources,
		NewDomains:    newDomains,
	}
}
