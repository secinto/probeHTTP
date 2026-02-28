package version

import (
	"strings"
	"testing"
)

func TestGetVersion(t *testing.T) {
	got := GetVersion()

	if !strings.Contains(got, "probeHTTP") {
		t.Errorf("GetVersion() output should contain 'probeHTTP', got %q", got)
	}
	if !strings.Contains(got, Version) {
		t.Errorf("GetVersion() output should contain Version %q, got %q", Version, got)
	}
	if !strings.Contains(got, GitCommit) {
		t.Errorf("GetVersion() output should contain GitCommit %q, got %q", GitCommit, got)
	}
	if !strings.Contains(got, BuildDate) {
		t.Errorf("GetVersion() output should contain BuildDate %q, got %q", BuildDate, got)
	}
	if !strings.Contains(got, "go") {
		t.Errorf("GetVersion() output should contain go version, got %q", got)
	}
}

func TestGetShortVersion(t *testing.T) {
	got := GetShortVersion()
	if got != Version {
		t.Errorf("GetShortVersion() = %q, want %q", got, Version)
	}
}
