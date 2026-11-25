package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/research/specter-reimplementation/internal/config"
)

type contractManifest struct {
	OperationCount int `json:"operationCount"`
	Operations     []struct {
		OperationID string `json:"operationId"`
		Method      string `json:"method"`
		Path        string `json:"path"`
	} `json:"operations"`
}

var pathParameter = regexp.MustCompile(`\{[^/{}]+\}`)

func TestManifestRoutesAreRegistered(t *testing.T) {
	content, err := os.ReadFile("../../contracts/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest contractManifest
	if err := json.Unmarshal(content, &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest.OperationCount != 202 || len(manifest.Operations) != 202 {
		t.Fatalf("expected the pinned 202-operation contract, got declared=%d actual=%d", manifest.OperationCount, len(manifest.Operations))
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	handler := New(config.Config{AllowedOrigin: "http://localhost:3000"}, nil, nil, nil, logger)
	seen := map[string]string{}
	for _, operation := range manifest.Operations {
		key := operation.Method + " " + strings.TrimSuffix(operation.Path, "?")
		if previous, exists := seen[key]; exists {
			t.Fatalf("duplicate route %s (%s and %s)", key, previous, operation.OperationID)
		}
		seen[key] = operation.OperationID
		path := pathParameter.ReplaceAllString(strings.TrimSuffix(operation.Path, "?"), "fixture")
		request := httptest.NewRequest(operation.Method, "http://example.test"+path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code == http.StatusNotFound {
			t.Errorf("unregistered contract route: %s %s (%s)", operation.Method, operation.Path, operation.OperationID)
		}
	}
}
