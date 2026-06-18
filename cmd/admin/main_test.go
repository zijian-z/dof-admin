package main

import (
	dnfparser "dofadmin"
	"net/http"
	"net/http/httptest"
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

func TestMatchesEquipmentExcludeOldEquipment(t *testing.T) {
	oldItem := &dnfparser.Equipment{Item: dnfparser.Item{ID: 29093, Name: "(旧) 碎霸手套"}}
	currentItem := &dnfparser.Equipment{Item: dnfparser.Item{ID: 29113, Name: "碎霸手套"}}
	fullWidthOldItem := &dnfparser.Equipment{Item: dnfparser.Item{ID: 29094, Name: " （旧） 碎霸手套"}}

	if !matchesEquipment(oldItem, itemFilter{}) {
		t.Fatal("old equipment should match when old equipment filter is disabled")
	}
	if matchesEquipment(oldItem, itemFilter{ExcludeOldEquipment: true}) {
		t.Fatal("old equipment should be excluded when old equipment filter is enabled")
	}
	if !matchesEquipment(currentItem, itemFilter{ExcludeOldEquipment: true}) {
		t.Fatal("current equipment should match when old equipment filter is enabled")
	}
	if matchesEquipment(fullWidthOldItem, itemFilter{ExcludeOldEquipment: true}) {
		t.Fatal("full-width old equipment marker should be excluded")
	}
}

func TestHandleStaticFaviconICO(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	rec := httptest.NewRecorder()

	(&server{}).handleStatic(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/x-icon" {
		t.Fatalf("Content-Type = %q, want %q", got, "image/x-icon")
	}
	if body := rec.Body.String(); len(body) == 0 || body[0] == '<' {
		t.Fatalf("favicon response looks like HTML fallback: len=%d", len(body))
	}
}
