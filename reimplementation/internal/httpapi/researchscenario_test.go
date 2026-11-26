package httpapi

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/research/specter-reimplementation/internal/auth"
	"github.com/research/specter-reimplementation/internal/config"
)

func TestResearchScenarioRouteBoundaries(t *testing.T) {
	secret := "test-only-research-scenario-secret"
	service := auth.New(nil, secret, time.Minute, time.Hour)
	handler := New(config.Config{}, nil, service, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	makeToken := func(role string) string {
		t.Helper()
		claims := auth.Claims{Username: "test-user", Roles: []string{role}, RegisteredClaims: jwt.RegisteredClaims{
			Subject: "1", Issuer: "specter-reimplementation", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		}}
		value, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	admin, viewer := makeToken("admin"), makeToken("viewer")
	cases := []struct {
		name, method, path, token, body string
		status                          int
	}{
		{"journal unauthenticated", "GET", "/api/research-scenarios/runs/test/journal", "", "", 401},
		{"viewer journal", "GET", "/api/research-scenarios/runs/test/journal", viewer, "", 403},
		{"oversized journal id", "GET", "/api/research-scenarios/runs/" + strings.Repeat("x", 129) + "/journal", admin, "", 400},
		{"catalog unauthenticated", "GET", "/api/research-scenarios", "", "", 401},
		{"history unauthenticated", "GET", "/api/research-scenarios/runs", "", "", 401},
		{"run unauthenticated", "POST", "/api/research-scenarios/runs", "", `{}`, 401},
		{"viewer catalog", "GET", "/api/research-scenarios", viewer, "", 403},
		{"viewer history", "GET", "/api/research-scenarios/runs", viewer, "", 403},
		{"viewer run", "POST", "/api/research-scenarios/runs", viewer, `{}`, 403},
		{"catalog", "GET", "/api/research-scenarios", admin, "", 200},
		{"unknown", "POST", "/api/research-scenarios/runs", admin, `{"scenarioId":"unknown"}`, 400},
		{"extra property", "POST", "/api/research-scenarios/runs", admin, `{"scenarioId":"clean-series","extra":true}`, 400},
		{"trailing JSON", "POST", "/api/research-scenarios/runs", admin, `{"scenarioId":"clean-series"}{}`, 400},
		{"oversized", "POST", "/api/research-scenarios/runs", admin, `{"scenarioId":"` + strings.Repeat("x", 4096) + `"}`, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			if w.Code != tc.status {
				t.Fatalf("got %d want %d: %s", w.Code, tc.status, w.Body.String())
			}
			if tc.name == "catalog" && !strings.Contains(w.Body.String(), "clean-series") {
				t.Fatal("missing preset")
			}
		})
	}
}
