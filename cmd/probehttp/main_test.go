package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestReadURLs_SkipsCommentsAndBlankLines(t *testing.T) {
	input := "# comment\n\nhttps://example.com\n  http://example.org/path  \n"

	urls, err := readURLs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(urls) != 2 {
		t.Fatalf("expected 2 URLs, got %d (%v)", len(urls), urls)
	}
	if urls[0] != "https://example.com" {
		t.Fatalf("unexpected first URL: %q", urls[0])
	}
	if urls[1] != "http://example.org/path" {
		t.Fatalf("unexpected second URL: %q", urls[1])
	}
}

func TestReadURLs_LongLineWithinBuffer(t *testing.T) {
	longURL := "https://example.com/" + strings.Repeat("a", 70*1024)
	input := longURL + "\n"

	urls, err := readURLs(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error for long line: %v", err)
	}
	if len(urls) != 1 {
		t.Fatalf("expected 1 URL, got %d", len(urls))
	}
	if urls[0] != longURL {
		t.Fatalf("long URL mismatch")
	}
}

func TestReadURLs_ErrTooLong(t *testing.T) {
	tooLongURL := "https://example.com/" + strings.Repeat("b", 1024*1024+1)
	input := tooLongURL + "\n"

	_, err := readURLs(strings.NewReader(input))
	if err == nil {
		t.Fatal("expected scanner error for oversized line, got nil")
	}
	if err != bufio.ErrTooLong {
		t.Fatalf("expected bufio.ErrTooLong, got %v", err)
	}
}
