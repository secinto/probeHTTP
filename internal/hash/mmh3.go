package hash

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/twmb/murmur3"
)

// Hash contains MMH3 hashes
type Hash struct {
	BodyMMH3   string `json:"body_mmh3"`
	HeaderMMH3 string `json:"header_mmh3"`
}

// CalculateMMH3 calculates the MMH3 hash of the data
func CalculateMMH3(data []byte) string {
	hash := murmur3.Sum32(data)
	return fmt.Sprintf("%d", hash)
}

// CalculateHeaderMMH3 calculates the MMH3 hash of concatenated headers
func CalculateHeaderMMH3(headers http.Header) string {
	// Sort headers for consistent hashing
	var keys []string
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Estimate capacity for pre-allocation (optimization from audit)
	estimatedSize := 0
	for k, vals := range headers {
		for _, v := range vals {
			estimatedSize += len(k) + 2 + len(v) + 1 // "key: value\n"
		}
	}

	// Concatenate headers with pre-allocated builder
	var headerStr strings.Builder
	headerStr.Grow(estimatedSize)
	for _, k := range keys {
		for _, v := range headers[k] {
			headerStr.WriteString(k)
			headerStr.WriteString(": ")
			headerStr.WriteString(v)
			headerStr.WriteString("\n")
		}
	}

	return CalculateMMH3([]byte(headerStr.String()))
}
