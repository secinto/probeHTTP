package parser

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ParsedURL holds the components of a parsed input URL
type ParsedURL struct {
	Original string // Original input string
	Scheme   string // http, https, or empty if not specified
	Host     string // hostname only (no port)
	Port     string // port number or empty
	Path     string // path component (default "/")
}

// ParseInputURL parses an input URL string and extracts its components
func ParseInputURL(inputURL string) ParsedURL {
	parsed := ParsedURL{
		Original: inputURL,
		Path:     "/",
	}

	// Check if input has a scheme
	hasScheme := strings.HasPrefix(inputURL, "http://") || strings.HasPrefix(inputURL, "https://")

	var u *url.URL
	var err error

	if hasScheme {
		// Parse as full URL
		u, err = url.Parse(inputURL)
		if err != nil {
			// If parsing fails, treat whole string as host
			parsed.Host = inputURL
			return parsed
		}

		parsed.Scheme = u.Scheme
		parsed.Host = u.Hostname()
		parsed.Port = u.Port()

		// Preserve full path including query and fragment
		if u.Path != "" {
			parsed.Path = u.Path
		}
		if u.RawQuery != "" {
			parsed.Path += "?" + u.RawQuery
		}
		if u.Fragment != "" {
			parsed.Path += "#" + u.Fragment
		}
	} else {
		// No scheme - could be "host", "host:port", "host/path", or "host:port/path"
		// First, check if there's a path separator
		pathStart := strings.Index(inputURL, "/")
		queryStart := strings.Index(inputURL, "?")
		fragmentStart := strings.Index(inputURL, "#")

		// Find where host:port ends
		hostPortEnd := len(inputURL)
		if pathStart >= 0 {
			hostPortEnd = pathStart
		}
		if queryStart >= 0 && queryStart < hostPortEnd {
			hostPortEnd = queryStart
		}
		if fragmentStart >= 0 && fragmentStart < hostPortEnd {
			hostPortEnd = fragmentStart
		}

		hostPort := inputURL[:hostPortEnd]
		rest := ""
		if hostPortEnd < len(inputURL) {
			rest = inputURL[hostPortEnd:]
		}

		// Parse host:port
		if strings.Contains(hostPort, ":") {
			parts := strings.SplitN(hostPort, ":", 2)
			parsed.Host = parts[0]
			// Check if second part looks like a port number
			portStr := parts[1]
			if port, err := strconv.Atoi(portStr); err == nil && port >= 1 && port <= 65535 {
				parsed.Port = portStr
			} else {
				// Doesn't look like a port, treat whole thing as host
				parsed.Host = hostPort
			}
		} else {
			parsed.Host = hostPort
		}

		// Preserve path, query, and fragment
		if rest != "" {
			parsed.Path = rest
		}
	}

	return parsed
}

// ValidateURL validates a URL for safety and correctness
// Returns an error if the URL is invalid or potentially dangerous
func ValidateURL(input string, allowPrivateIPs bool) error {
	// Check length
	if len(input) > 2048 {
		return fmt.Errorf("URL too long (max 2048 chars)")
	}

	// Check for null bytes
	if strings.Contains(input, "\x00") {
		return fmt.Errorf("URL contains null bytes")
	}

	// Parse URL
	parsed := ParseInputURL(input)

	// Validate hostname is not empty
	if parsed.Host == "" {
		return fmt.Errorf("empty hostname")
	}

	// Check for localhost/private IPs if not allowed
	if !allowPrivateIPs {
		if parsed.Host == "localhost" || parsed.Host == "127.0.0.1" {
			return fmt.Errorf("localhost not allowed")
		}

		// Parse IP
		ip := net.ParseIP(parsed.Host)
		if ip != nil {
			// Check if private IP
			if ip.IsLoopback() || ip.IsPrivate() {
				return fmt.Errorf("private IP addresses not allowed")
			}
		}
	}

	return nil
}

// ExpandURLs takes an input URL and returns all URLs to probe based on configuration
func ExpandURLs(inputURL string, allSchemes bool, ignorePorts bool, customPorts string) []string {
	parsed := ParseInputURL(inputURL)
	schemes := getSchemesToTest(parsed, allSchemes)

	urlMap := make(map[string]bool) // For deduplication
	var urls []string

	for _, scheme := range schemes {
		ports := getPortsToTest(parsed, scheme, ignorePorts, customPorts)

		for _, port := range ports {
			// Determine if port should be included in URL
			includePort := shouldIncludePortInURL(parsed, port, scheme, ignorePorts, customPorts)

			// Build the URL
			urlStr := buildProbeURL(scheme, parsed.Host, port, parsed.Path, includePort)

			// Deduplicate
			if !urlMap[urlStr] {
				urlMap[urlStr] = true
				urls = append(urls, urlStr)
			}
		}
	}

	return urls
}

// getSchemesToTest returns the list of schemes to test based on configuration and input
func getSchemesToTest(parsed ParsedURL, allSchemes bool) []string {
	// If AllSchemes flag is set, always test both
	if allSchemes {
		return []string{"http", "https"}
	}

	// If no scheme in input, test both by default
	if parsed.Scheme == "" {
		return []string{"http", "https"}
	}

	// Use the scheme from input
	return []string{parsed.Scheme}
}

// getPortsToTest returns the list of ports to test based on configuration and input
func getPortsToTest(parsed ParsedURL, scheme string, ignorePorts bool, customPorts string) []string {
	// Custom ports override everything
	if customPorts != "" {
		ports, err := ParsePortList(customPorts)
		if err != nil {
			// Fall back to default behavior on error
		} else {
			return ports
		}
	}

	// If IgnorePorts is set, use default common ports for the scheme
	if ignorePorts {
		if scheme == "https" {
			return DefaultHTTPSPorts
		}
		return DefaultHTTPPorts
	}

	// If port is specified in input, use it
	if parsed.Port != "" {
		return []string{parsed.Port}
	}

	// Default: use standard port for the scheme
	if scheme == "https" {
		return []string{"443"}
	}
	return []string{"80"}
}

// shouldIncludePortInURL determines if the port should be explicitly included in the URL
func shouldIncludePortInURL(parsed ParsedURL, port string, scheme string, ignorePorts bool, customPorts string) bool {
	// Always include port if port-related flags are active
	if ignorePorts || customPorts != "" {
		return true
	}

	// Always include port if it was explicitly specified in the original input
	if parsed.Port != "" {
		return true
	}

	// Include port if it's non-standard for the scheme
	standardPort := "80"
	if scheme == "https" {
		standardPort = "443"
	}

	return port != standardPort
}

// buildProbeURL builds a URL string with optional port inclusion
func buildProbeURL(scheme string, host string, port string, path string, includePort bool) string {
	if includePort {
		return fmt.Sprintf("%s://%s:%s%s", scheme, host, port, path)
	}
	return fmt.Sprintf("%s://%s%s", scheme, host, path)
}

// NormalizeURL normalizes a URL by removing default ports to create a canonical form
// This allows deduplication of URLs like http://host and http://host:80
func NormalizeURL(urlStr string) string {
	parsed, err := url.Parse(urlStr)
	if err != nil {
		return urlStr // Return original if parsing fails
	}

	// Remove default ports
	if parsed.Port() == "80" && parsed.Scheme == "http" {
		parsed.Host = parsed.Hostname()
	}
	if parsed.Port() == "443" && parsed.Scheme == "https" {
		parsed.Host = parsed.Hostname()
	}

	return parsed.String()
}

// DeduplicateURLs removes duplicate URLs that resolve to the same endpoint
// by normalizing URLs (removing default ports) before comparison
func DeduplicateURLs(urls []string) []string {
	seen := make(map[string]bool)
	var deduplicated []string

	for _, urlStr := range urls {
		normalized := NormalizeURL(urlStr)
		if !seen[normalized] {
			seen[normalized] = true
			deduplicated = append(deduplicated, urlStr)
		}
	}

	return deduplicated
}
