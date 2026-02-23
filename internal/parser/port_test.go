package parser

import (
	"strings"
	"testing"
)

func TestParsePortList_SinglePort(t *testing.T) {
	ports, err := ParsePortList("80")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 1 || ports[0] != "80" {
		t.Errorf("got %v, want [80]", ports)
	}
}

func TestParsePortList_MultiplePorts(t *testing.T) {
	ports, err := ParsePortList("443,80,8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should be sorted numerically
	expected := []string{"80", "443", "8080"}
	if len(ports) != len(expected) {
		t.Fatalf("got %v, want %v", ports, expected)
	}
	for i, p := range ports {
		if p != expected[i] {
			t.Errorf("ports[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestParsePortList_Range(t *testing.T) {
	ports, err := ParsePortList("8000-8003")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"8000", "8001", "8002", "8003"}
	if len(ports) != len(expected) {
		t.Fatalf("got %v, want %v", ports, expected)
	}
	for i, p := range ports {
		if p != expected[i] {
			t.Errorf("ports[%d] = %q, want %q", i, p, expected[i])
		}
	}
}

func TestParsePortList_MixedRangeAndSingle(t *testing.T) {
	ports, err := ParsePortList("80,443,8000-8002")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []string{"80", "443", "8000", "8001", "8002"}
	if len(ports) != len(expected) {
		t.Fatalf("got %v, want %v", ports, expected)
	}
}

func TestParsePortList_Deduplication(t *testing.T) {
	ports, err := ParsePortList("80,80,80")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 1 {
		t.Errorf("expected 1 port after dedup, got %v", ports)
	}
}

func TestParsePortList_OverlappingRanges(t *testing.T) {
	ports, err := ParsePortList("8000-8005,8003-8007")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 8 { // 8000-8007
		t.Errorf("expected 8 unique ports, got %d: %v", len(ports), ports)
	}
}

func TestParsePortList_SameStartEnd(t *testing.T) {
	ports, err := ParsePortList("80-80")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 1 || ports[0] != "80" {
		t.Errorf("got %v, want [80]", ports)
	}
}

func TestParsePortList_Spaces(t *testing.T) {
	ports, err := ParsePortList("80, 443, 8080")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ports) != 3 {
		t.Errorf("got %d ports, want 3", len(ports))
	}
}

func TestParsePortList_BoundaryPorts(t *testing.T) {
	// Port 1 (minimum)
	ports, err := ParsePortList("1")
	if err != nil {
		t.Fatalf("port 1 should be valid: %v", err)
	}
	if ports[0] != "1" {
		t.Errorf("got %v, want [1]", ports)
	}

	// Port 65535 (maximum)
	ports, err = ParsePortList("65535")
	if err != nil {
		t.Fatalf("port 65535 should be valid: %v", err)
	}
	if ports[0] != "65535" {
		t.Errorf("got %v, want [65535]", ports)
	}
}

func TestParsePortList_Errors(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr string
	}{
		{"empty string", "", "empty port list"},
		{"port 0", "0", "port out of range"},
		{"port 65536", "65536", "port out of range"},
		// "-1" splits to ["", "1"], empty start fails Atoi
		{"negative port", "-1", "invalid start port in range"},
		{"non-numeric", "abc", "invalid port number"},
		{"reversed range", "8005-8000", "start port > end port"},
		{"range start 0", "0-10", "start port out of range"},
		{"range end 70000", "80-70000", "end port out of range"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParsePortList(tt.input)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want to contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}
