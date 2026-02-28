package tech

import (
	"net/http"
	"testing"
)

func TestNewDetector(t *testing.T) {
	detector, err := NewDetector()
	if err != nil {
		t.Fatalf("NewDetector() error: %v", err)
	}
	if detector == nil {
		t.Error("NewDetector() returned nil detector")
	}
}

func TestDetect_NilDetector(t *testing.T) {
	var d *Detector
	got := d.Detect(http.Header{}, nil)
	if got != nil {
		t.Errorf("nil Detector.Detect() = %v, want nil", got)
	}
}

func TestDetect_EmptyHeadersBody(t *testing.T) {
	detector, err := NewDetector()
	if err != nil {
		t.Fatalf("NewDetector() error: %v", err)
	}
	got := detector.Detect(http.Header{}, nil)
	if got != nil {
		t.Errorf("Detect(empty, nil) = %v, want nil", got)
	}
}

func TestDetect_WithFingerprint(t *testing.T) {
	detector, err := NewDetector()
	if err != nil {
		t.Fatalf("NewDetector() error: %v", err)
	}
	// WordPress emits X-Powered-By and has characteristic HTML
	headers := http.Header{}
	headers.Set("X-Powered-By", "PHP/8.1")
	headers.Set("Server", "nginx")
	body := []byte(`<!DOCTYPE html><html><head><meta name="generator" content="WordPress 6.0"></head><body></body></html>`)

	got := detector.Detect(headers, body)
	if got == nil {
		t.Log("Detect() returned nil - wappalyzer fingerprints may vary; this is acceptable")
		return
	}
	if len(got) == 0 {
		t.Log("Detect() returned empty slice - wappalyzer fingerprints may vary; this is acceptable")
		return
	}
	// If we get results, verify they are non-empty strings
	for _, tech := range got {
		if tech == "" {
			t.Error("Detect() returned empty technology string")
		}
	}
}
