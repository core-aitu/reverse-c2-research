package store

import (
	"context"
	"strings"
	"testing"
)

func TestParseVersion(t *testing.T) {
	value, err := parseVersion("7.0.1.42")
	if err != nil {
		t.Fatal(err)
	}
	if value.Major != 7 || value.Minor != 0 || value.Build != 1 || value.Revision != 42 {
		t.Fatalf("unexpected version: %+v", value)
	}
	for _, invalid := range []string{"7", "7.0.1", "7.0.1.0.0", "07.0.1.0", "7.-1.0.0", "7.65536.0.0", "latest"} {
		if _, err := parseVersion(invalid); err == nil {
			t.Errorf("accepted invalid version %q", invalid)
		}
	}
}

func TestSafeContentKey(t *testing.T) {
	for _, valid := range []string{"fixture-host", "report 2026.bin", "host.example.test"} {
		if !safeContentKey(valid, 255) {
			t.Errorf("rejected valid key %q", valid)
		}
	}
	for _, invalid := range []string{"", ".", "..", "../escape", `folder\\file`, "line\nbreak", "nul\x00byte"} {
		if safeContentKey(invalid, 255) {
			t.Errorf("accepted invalid key %q", invalid)
		}
	}
}

func TestNewPersistentContentRequiresObjectStorage(t *testing.T) {
	storage := &Store{}
	if _, err := storage.PutArtifactContent(context.Background(), "host", "file.bin", "application/octet-stream", []byte("x")); err == nil || !strings.Contains(err.Error(), "object storage") {
		t.Fatalf("artifact write without object storage: %v", err)
	}
	if _, err := storage.PutUpdatePackage(context.Background(), "7.0.1.0", "application/octet-stream", strings.Repeat("0", 64), []byte("x")); err == nil || !strings.Contains(err.Error(), "object storage") {
		t.Fatalf("update write without object storage: %v", err)
	}
}
