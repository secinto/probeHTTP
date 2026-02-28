package useragent

import (
	"testing"
)

func TestGet_Custom(t *testing.T) {
	got := Get("custom-ua", false)
	if got != "custom-ua" {
		t.Errorf("Get(custom-ua, false) = %q, want custom-ua", got)
	}
}

func TestGet_Default(t *testing.T) {
	got := Get("", false)
	if got != Default {
		t.Errorf("Get(\"\", false) = %q, want %q", got, Default)
	}
}

func TestGet_Random(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		got := Get("", true)
		if got == "" {
			t.Errorf("Get(\"\", true) returned empty string")
		}
		seen[got] = true
	}
	// Should return at least one value from Pool
	foundInPool := false
	for _, ua := range Pool {
		if seen[ua] {
			foundInPool = true
			break
		}
	}
	if !foundInPool {
		t.Error("Get(\"\", true) should return values from Pool")
	}
}

func TestGetRandom(t *testing.T) {
	got := GetRandom()
	if got == "" {
		t.Error("GetRandom() returned empty string")
	}
	foundInPool := false
	for _, ua := range Pool {
		if got == ua {
			foundInPool = true
			break
		}
	}
	if !foundInPool {
		t.Errorf("GetRandom() = %q, should be one of Pool", got)
	}
}
