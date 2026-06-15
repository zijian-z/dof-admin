package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadExecutableEnv(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable() error = %v", err)
	}

	envPath := filepath.Join(filepath.Dir(exe), ".env")
	if err := os.WriteFile(envPath, []byte(`DNF_ADMIN_TEST_ENV="loaded from env"`+"\n"), 0o600); err != nil {
		t.Fatalf("write .env error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Remove(envPath)
		_ = os.Unsetenv("DNF_ADMIN_TEST_ENV")
	})
	_ = os.Unsetenv("DNF_ADMIN_TEST_ENV")

	if err := loadExecutableEnv(".env"); err != nil {
		t.Fatalf("loadExecutableEnv() error = %v", err)
	}
	if got := os.Getenv("DNF_ADMIN_TEST_ENV"); got != "loaded from env" {
		t.Fatalf("DNF_ADMIN_TEST_ENV = %q, want %q", got, "loaded from env")
	}
}
