package httpapi

import (
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/research/specter-reimplementation/internal/domain"
)

func TestAgentProtocolRejectsDowngrade(t *testing.T) {
	server := &Server{}
	for _, version := range []string{"", "0", "2", "01"} {
		request := httptest.NewRequest(http.MethodPost, "/api/agent/v1/checkin", strings.NewReader("frame"))
		request.Header.Set("Content-Type", "application/vnd.specter.gateway+gzip")
		request.Header.Set("X-Specter-Protocol-Version", version)
		response := httptest.NewRecorder()
		if _, ok := server.protocolRequest(response, request); ok || response.Code != http.StatusUpgradeRequired {
			t.Fatalf("version %q: ok=%v status=%d", version, ok, response.Code)
		}
	}
}

func TestCorrelationFilterValidation(t *testing.T) {
	for _, value := range []string{"", "agent-1", "task:one.two_three"} {
		if !validCorrelationID(value) {
			t.Fatalf("valid correlation ID rejected: %q", value)
		}
	}
	for _, value := range []string{"../task", "task id", strings.Repeat("a", 129), "task?all=true"} {
		if validCorrelationID(value) {
			t.Fatalf("invalid correlation ID accepted: %q", value)
		}
	}
}

func TestOperationalDTOsContainNoPayloadOrSecret(t *testing.T) {
	task := operationalGatewayTask(domain.GatewayTask{ID: "task", Payload: json.RawMessage(`{"secret":"task-payload"}`), Result: json.RawMessage(`{"secret":"task-result"}`)})
	encoded, err := json.Marshal(struct {
		Agents []domain.AgentOperationalView       `json:"agents"`
		Tasks  []domain.GatewayTaskOperationalView `json:"tasks"`
		Tokens []domain.EnrollmentTokenInfo        `json:"tokens"`
	}{Agents: []domain.AgentOperationalView{{ID: "agent"}}, Tasks: []domain.GatewayTaskOperationalView{task}, Tokens: []domain.EnrollmentTokenInfo{{ID: "enrollment"}}})
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(encoded))
	for _, forbidden := range []string{"payload", "identity", "tokenhash", "secret"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("operational DTO exposed %q: %s", forbidden, text)
		}
	}
}

func TestAgentRoutesRequireTLS(t *testing.T) {
	server := &Server{}
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	handler := server.requireAgentTLS(next)
	request := httptest.NewRequest(http.MethodPost, "/api/agent/v1/checkin", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUpgradeRequired {
		t.Fatalf("plain agent route status=%d", response.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/agent/v1/checkin", nil)
	request.TLS = &tls.ConnectionState{}
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("TLS agent route status=%d", response.Code)
	}
}

func TestAdvertisedCapabilitiesAndTaskSelection(t *testing.T) {
	capabilities, err := parseAdvertisedCapabilities("shell:sh,exec,file-download,sleep,echo,system-info,exec")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(capabilities, ","); got != "echo,exec,file-download,shell:sh,sleep,system-info" {
		t.Fatalf("capabilities=%q", got)
	}
	if !supportsTask(capabilities, "exec", json.RawMessage(`{"executable":"/bin/true"}`)) ||
		!supportsTask(capabilities, "shell", json.RawMessage(`{"interpreter":"sh","command":"true"}`)) ||
		supportsTask(capabilities, "shell", json.RawMessage(`{"interpreter":"powershell","command":"true"}`)) {
		t.Fatal("task capability selection did not match the advertised executor")
	}
	for _, raw := range []string{
		"exec,file-download,sleep,system-info",
		"echo,sleep,system-info,shell:bash",
	} {
		if _, err = parseAdvertisedCapabilities(raw); err == nil {
			t.Fatalf("invalid capabilities accepted: %q", raw)
		}
	}
}

func TestFileChunkFitsDeclaredSize(t *testing.T) {
	tests := []struct {
		name               string
		expected, received int64
		chunkSize          int
		want               bool
	}{
		{name: "first chunk", expected: 10, chunkSize: 8, want: true},
		{name: "exact final chunk", expected: 10, received: 8, chunkSize: 2, want: true},
		{name: "declared size overrun", expected: 10, received: 8, chunkSize: 3},
		{name: "already complete", expected: 10, received: 10, chunkSize: 1},
		{name: "invalid persisted count", expected: 10, received: 11, chunkSize: 1},
		{name: "empty chunk", expected: 10, chunkSize: 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := fileChunkFits(test.expected, test.received, test.chunkSize); got != test.want {
				t.Fatalf("fileChunkFits(%d,%d,%d)=%v want %v", test.expected, test.received, test.chunkSize, got, test.want)
			}
		})
	}
}
