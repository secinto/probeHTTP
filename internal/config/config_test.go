package config

import (
	"flag"
	"io"
	"os"
	"strings"
	"testing"
)

func withFlagSet(t *testing.T, args []string, testFn func()) {
	t.Helper()

	originalCommandLine := flag.CommandLine
	originalArgs := os.Args

	commandLine := flag.NewFlagSet(args[0], flag.ContinueOnError)
	commandLine.SetOutput(io.Discard)
	flag.CommandLine = commandLine
	os.Args = args

	defer func() {
		flag.CommandLine = originalCommandLine
		os.Args = originalArgs
	}()

	testFn()
}

func TestParseFlags_InvalidConcurrency(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "zero concurrency",
			args: []string{"probehttp", "-c", "0"},
		},
		{
			name: "negative concurrency",
			args: []string{"probehttp", "-c", "-5"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			withFlagSet(t, testCase.args, func() {
				_, err := ParseFlags()
				if err == nil {
					t.Fatal("expected error for invalid concurrency, got nil")
				}
				if !strings.Contains(err.Error(), "concurrency must be greater than 0") {
					t.Fatalf("unexpected error: %v", err)
				}
			})
		})
	}
}

func TestNew_DefaultValues(t *testing.T) {
	cfg := New()
	if cfg == nil {
		t.Fatal("New returned nil")
	}
	if !cfg.FollowRedirects {
		t.Error("FollowRedirects should default to true")
	}
	if cfg.MaxRedirects != 10 {
		t.Errorf("MaxRedirects want 10, got %d", cfg.MaxRedirects)
	}
	if cfg.Timeout != 10 {
		t.Errorf("Timeout want 10, got %d", cfg.Timeout)
	}
	if cfg.Concurrency != 20 {
		t.Errorf("Concurrency want 20, got %d", cfg.Concurrency)
	}
	if cfg.MaxBodySize != 10*1024*1024 {
		t.Errorf("MaxBodySize want 10MB, got %d", cfg.MaxBodySize)
	}
	if cfg.MaxRetries != 0 {
		t.Errorf("MaxRetries want 0, got %d", cfg.MaxRetries)
	}
}

func TestHasPipedData_DoesNotPanic(t *testing.T) {
	_ = HasPipedData()
}

func TestClose_NoDebugFile(t *testing.T) {
	cfg := New()
	if err := cfg.Close(); err != nil {
		t.Errorf("Close with no debug file should not error, got %v", err)
	}
}

func TestClose_WithDebugFile(t *testing.T) {
	tempFile, err := os.CreateTemp("", "probehttp-config-test-*.log")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile.Name())

	cfg := New()
	cfg.debugFileHandle = tempFile
	if err := cfg.Close(); err != nil {
		t.Errorf("Close with debug file should not error, got %v", err)
	}
}
