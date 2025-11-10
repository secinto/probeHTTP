package parser

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Default common HTTP ports for probing
var DefaultHTTPPorts = []string{"80", "8000", "8080", "8888"}

// Default common HTTPS ports for probing
var DefaultHTTPSPorts = []string{"443", "8443", "10443", "8444"}

// ParsePortList parses a comma-separated port list with support for ranges
// Examples:
//   "80,443,8080" → [80, 443, 8080]
//   "8000-8005" → [8000, 8001, 8002, 8003, 8004, 8005]
//   "80,443,8000-8010" → [80, 443, 8000, 8001, ..., 8010]
func ParsePortList(portStr string) ([]string, error) {
	if portStr == "" {
		return nil, fmt.Errorf("empty port list")
	}

	portMap := make(map[string]bool) // Use map for deduplication
	parts := strings.Split(portStr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if it's a range (e.g., "8000-8010")
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid port range format: %s", part)
			}

			start, err := strconv.Atoi(strings.TrimSpace(rangeParts[0]))
			if err != nil {
				return nil, fmt.Errorf("invalid start port in range %s: %v", part, err)
			}

			end, err := strconv.Atoi(strings.TrimSpace(rangeParts[1]))
			if err != nil {
				return nil, fmt.Errorf("invalid end port in range %s: %v", part, err)
			}

			// Validate port range
			if start < 1 || start > 65535 {
				return nil, fmt.Errorf("start port out of range (1-65535): %d", start)
			}
			if end < 1 || end > 65535 {
				return nil, fmt.Errorf("end port out of range (1-65535): %d", end)
			}
			if start > end {
				return nil, fmt.Errorf("invalid range %s: start port > end port", part)
			}

			// Add all ports in range
			for port := start; port <= end; port++ {
				portMap[strconv.Itoa(port)] = true
			}
		} else {
			// Single port
			port, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid port number: %s", part)
			}

			// Validate port
			if port < 1 || port > 65535 {
				return nil, fmt.Errorf("port out of range (1-65535): %d", port)
			}

			portMap[part] = true
		}
	}

	// Check if we got any valid ports
	if len(portMap) == 0 {
		return nil, fmt.Errorf("no valid ports found in port list")
	}

	// Convert map to sorted slice
	ports := make([]string, 0, len(portMap))
	for port := range portMap {
		ports = append(ports, port)
	}

	// Sort ports numerically
	sort.Slice(ports, func(i, j int) bool {
		pi, _ := strconv.Atoi(ports[i])
		pj, _ := strconv.Atoi(ports[j])
		return pi < pj
	})

	return ports, nil
}
