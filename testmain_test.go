package main

import (
	"fmt"
	"os"
	"testing"
)

// TestMain points the user config and home folders at a temporary directory,
// so NewApp (which migrates and loads settings) never reads or writes the real
// user settings while the tests run.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "bujo-helper-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain:", err)
		os.Exit(1)
	}
	for _, key := range []string{"XDG_CONFIG_HOME", "HOME", "AppData"} {
		os.Setenv(key, dir)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
