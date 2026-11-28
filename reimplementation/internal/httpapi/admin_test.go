package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/research/specter-reimplementation/internal/domain"
)

func TestReadBinaryBodyLimitsInput(t *testing.T) {
	request := httptest.NewRequest("PUT", "http://example.test/upload", strings.NewReader("12345"))
	response := httptest.NewRecorder()
	if _, ok := readBinaryBody(response, request, 4); ok || response.Code != 413 {
		t.Fatalf("oversized body was not rejected: status=%d", response.Code)
	}

	request = httptest.NewRequest("PUT", "http://example.test/upload", strings.NewReader("data"))
	response = httptest.NewRecorder()
	content, ok := readBinaryBody(response, request, 4)
	if !ok || string(content) != "data" {
		t.Fatalf("valid body was rejected: ok=%v content=%q", ok, content)
	}
}

func TestRequestMediaTypeAndVersionString(t *testing.T) {
	request := httptest.NewRequest("PUT", "http://example.test/upload", nil)
	request.Header.Set("Content-Type", "application/zip; charset=binary")
	if value := requestMediaType(request); value != "application/zip" {
		t.Fatalf("unexpected media type: %s", value)
	}
	if value := versionString(domain.SerializableVersion{Major: 7, Minor: 0, Build: 1, Revision: 42}); value != "7.0.1.42" {
		t.Fatalf("unexpected version: %s", value)
	}
}

func TestChecksumManifestRequiresExactBinaryEntry(t *testing.T) {
	digest := strings.Repeat("a", 64)
	valid := []byte(digest + "  specter-beacon-linux-amd64\n")
	if !checksumManifestContains(valid, digest, "specter-beacon-linux-amd64") {
		t.Fatal("valid checksum entry rejected")
	}
	for _, content := range [][]byte{
		[]byte("prefix" + digest + "suffix"),
		[]byte(digest + "  another-file\n"),
		[]byte(strings.Repeat("b", 64) + "  specter-beacon-linux-amd64\n"),
	} {
		if checksumManifestContains(content, digest, "specter-beacon-linux-amd64") {
			t.Fatalf("invalid checksum manifest accepted: %q", content)
		}
	}
}
