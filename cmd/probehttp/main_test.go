package main

import (
	"strings"
	"testing"
)

func TestReadURLs(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect []string
	}{
		{"empty", "", nil},
		{"single URL", "https://example.com", []string{"https://example.com"}},
		{"multiple URLs", "https://a.com\nhttps://b.com", []string{"https://a.com", "https://b.com"}},
		{"skips comments", "https://a.com\n# comment\nhttps://b.com", []string{"https://a.com", "https://b.com"}},
		{"skips empty lines", "https://a.com\n\nhttps://b.com", []string{"https://a.com", "https://b.com"}},
		{"trims whitespace", "  https://example.com  ", []string{"https://example.com"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readURLs(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("readURLs: %v", err)
			}
			if len(got) != len(tt.expect) {
				t.Errorf("len = %d, want %d", len(got), len(tt.expect))
			}
			for i := range got {
				if got[i] != tt.expect[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.expect[i])
				}
			}
		})
	}
}

func TestReadURLs_EmptyInput(t *testing.T) {
	got, err := readURLs(strings.NewReader(""))
	if err != nil {
		t.Fatalf("readURLs empty: %v", err)
	}
	if got != nil {
		t.Errorf("readURLs empty input: got %v, want nil", got)
	}
}
