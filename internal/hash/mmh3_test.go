package hash

import (
	"net/http"
	"testing"
)

func TestCalculateMMH3_Deterministic(t *testing.T) {
	data := []byte("hello world")
	h1 := CalculateMMH3(data)
	h2 := CalculateMMH3(data)
	if h1 != h2 {
		t.Errorf("same data produced different hashes: %q vs %q", h1, h2)
	}
}

func TestCalculateMMH3_DifferentData(t *testing.T) {
	h1 := CalculateMMH3([]byte("hello"))
	h2 := CalculateMMH3([]byte("world"))
	if h1 == h2 {
		t.Error("different data should produce different hashes")
	}
}

func TestCalculateMMH3_EmptyData(t *testing.T) {
	h := CalculateMMH3([]byte{})
	if h == "" {
		t.Error("empty data should still produce a hash")
	}

	// Verify it's a valid number string
	if h == "0" {
		// MMH3 of empty data is 0
	}
}

func TestCalculateMMH3_Format(t *testing.T) {
	h := CalculateMMH3([]byte("test"))
	// Should be a decimal number string (not hex)
	for _, c := range h {
		if c < '0' || c > '9' {
			t.Errorf("hash %q contains non-digit character %q", h, string(c))
			break
		}
	}
}

func TestCalculateHeaderMMH3_Deterministic(t *testing.T) {
	headers := http.Header{
		"Content-Type": {"text/html"},
		"Server":       {"nginx"},
	}
	h1 := CalculateHeaderMMH3(headers)
	h2 := CalculateHeaderMMH3(headers)
	if h1 != h2 {
		t.Errorf("same headers produced different hashes: %q vs %q", h1, h2)
	}
}

func TestCalculateHeaderMMH3_OrderIndependent(t *testing.T) {
	// Headers are sorted by key before hashing, so order shouldn't matter
	h1 := CalculateHeaderMMH3(http.Header{
		"A-Header": {"value1"},
		"B-Header": {"value2"},
	})
	h2 := CalculateHeaderMMH3(http.Header{
		"B-Header": {"value2"},
		"A-Header": {"value1"},
	})
	if h1 != h2 {
		t.Errorf("header order should not affect hash: %q vs %q", h1, h2)
	}
}

func TestCalculateHeaderMMH3_DifferentHeaders(t *testing.T) {
	h1 := CalculateHeaderMMH3(http.Header{"Server": {"nginx"}})
	h2 := CalculateHeaderMMH3(http.Header{"Server": {"apache"}})
	if h1 == h2 {
		t.Error("different headers should produce different hashes")
	}
}

func TestCalculateHeaderMMH3_EmptyHeaders(t *testing.T) {
	h := CalculateHeaderMMH3(http.Header{})
	if h == "" {
		t.Error("empty headers should still produce a hash")
	}
}

func TestCalculateHeaderMMH3_MultipleValues(t *testing.T) {
	headers := http.Header{
		"Set-Cookie": {"a=1", "b=2"},
	}
	h := CalculateHeaderMMH3(headers)
	if h == "" {
		t.Error("should handle multiple values per header")
	}

	// Single value should produce different hash
	h2 := CalculateHeaderMMH3(http.Header{
		"Set-Cookie": {"a=1"},
	})
	if h == h2 {
		t.Error("different number of header values should produce different hash")
	}
}
