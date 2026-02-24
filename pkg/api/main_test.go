package api

import (
	"os"
	"testing"
)

// TestMain isolates HOME so user-specific settings do not leak into tests.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "api-test-home")
	if err != nil {
		panic(err)
	}
	os.Setenv("HOME", home)
	code := m.Run()
	_ = os.RemoveAll(home)
	os.Exit(code)
}
