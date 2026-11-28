package beacon

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/research/specter-reimplementation/internal/gateway"
)

type closeTrackingTransport struct {
	mu     sync.Mutex
	closed bool
}

func (t *closeTrackingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("denied"))}, nil
}

func (t *closeTrackingTransport) CloseIdleConnections() {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
}

func (t *closeTrackingTransport) wasClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

func TestBeaconEnrollsEchoesAndPersistsCursor(t *testing.T) {
	var mu sync.Mutex
	results := map[string]gateway.Result{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Specter-Protocol-Version", "1")
		if r.Header.Get("X-Specter-Protocol-Version") != "1" {
			t.Errorf("missing protocol version")
		}
		body, _ := io.ReadAll(r.Body)
		if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			sequence, _ := strconv.ParseUint(r.Header.Get("X-Specter-Sequence"), 10, 64)
			if !gateway.VerifyRequestSignature("identity-test", r.Method, r.URL.Path, r.Header.Get("X-Specter-Timestamp"), r.Header.Get("X-Specter-Nonce"), sequence, body, r.Header.Get("X-Specter-Signature"), time.Now().UTC()) {
				t.Errorf("invalid request signature for %s", r.URL.Path)
			}
		}
		switch r.URL.Path {
		case "/api/agent/v1/enroll":
			if r.Header.Get("Authorization") != "Enrollment one-use" {
				t.Errorf("bad enrollment auth")
			}
			if _, err := gateway.DecodeRegistration(body); err != nil {
				t.Errorf("registration: %v", err)
			}
			w.Header().Set("X-Specter-Agent-ID", "agent-test")
			w.Header().Set("X-Specter-Identity", "identity-test")
			frame, _ := gateway.EncodeAck(gateway.Ack{})
			_, _ = w.Write(frame)
		case "/api/agent/v1/checkin":
			checkin, err := gateway.DecodeCheckin(body)
			if err != nil {
				t.Errorf("checkin: %v", err)
			}
			deadline := time.Now().Add(time.Minute)
			batch := gateway.TaskBatch{Sequence: checkin.Sequence, Cursor: 2, Tasks: []gateway.Task{{Version: 1, ID: "task-1", IdempotencyKey: "idem-1", Cursor: 1, Kind: "echo", Payload: []byte(`{"value":"one"}`), Deadline: deadline, Timeout: time.Second}, {Version: 1, ID: "task-2", IdempotencyKey: "idem-2", Cursor: 2, Kind: "echo", Payload: []byte(`{"value":"two"}`), Deadline: deadline, Timeout: time.Second}}}
			frame, _ := gateway.EncodeTaskBatch(batch)
			_, _ = w.Write(frame)
		case "/api/agent/v1/ack":
			if _, err := gateway.DecodeReceipt(body); err != nil {
				t.Errorf("receipt: %v", err)
			}
			frame, _ := gateway.EncodeAck(gateway.Ack{})
			_, _ = w.Write(frame)
		case "/api/agent/v1/results":
			result, err := gateway.DecodeResult(body)
			if err != nil {
				t.Errorf("result: %v", err)
			}
			mu.Lock()
			results[result.TaskID] = result
			mu.Unlock()
			frame, _ := gateway.EncodeAck(gateway.Ack{Sequence: result.Sequence})
			_, _ = w.Write(frame)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	statePath := filepath.Join(t.TempDir(), "state.json")
	client, err := New(Config{Endpoint: server.URL, EnrollmentToken: "one-use", AgentID: "agent-test", Name: "test", StatePath: statePath, Interval: 100 * time.Millisecond, BatchSize: 2, AllowInsecureLoopback: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- client.Run(ctx) }()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := len(results)
		mu.Unlock()
		if count == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	<-done
	mu.Lock()
	if len(results) != 2 || !results["task-1"].Success || !results["task-2"].Success {
		mu.Unlock()
		t.Fatalf("unexpected results: %#v", results)
	}
	mu.Unlock()
	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state State
	if err = json.Unmarshal(content, &state); err != nil {
		t.Fatal(err)
	}
	if state.Cursor != 2 || len(state.Pending) != 0 || state.Identity != "identity-test" {
		t.Fatalf("unexpected state: %#v", state)
	}
	restarted, err := New(Config{Endpoint: server.URL, AgentID: "agent-test", Name: "test", StatePath: statePath, Interval: 100 * time.Millisecond, BatchSize: 2, AllowInsecureLoopback: true})
	if err != nil {
		t.Fatal(err)
	}
	restartContext, stopRestart := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer stopRestart()
	_ = restarted.Run(restartContext)
	mu.Lock()
	defer mu.Unlock()
	if len(results) != 2 {
		t.Fatalf("completed task executed after restart: %#v", results)
	}
}

func TestBeaconBackoffAvoidsTightRetryLoop(t *testing.T) {
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()
		http.Error(w, "temporary", http.StatusServiceUnavailable)
	}))
	defer server.Close()
	statePath := filepath.Join(t.TempDir(), "state.json")
	content, _ := json.Marshal(State{Version: 1, AgentID: "agent-test", Identity: "identity-test"})
	if err := os.WriteFile(statePath, content, 0600); err != nil {
		t.Fatal(err)
	}
	client, err := New(Config{Endpoint: server.URL, AgentID: "agent-test", Name: "test", StatePath: statePath, Interval: 100 * time.Millisecond, Jitter: time.Nanosecond, MinBackoff: 100 * time.Millisecond, MaxBackoff: 200 * time.Millisecond, ConnectTimeout: 100 * time.Millisecond, RequestTimeout: 150 * time.Millisecond, BatchSize: 1, AllowInsecureLoopback: true})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 370*time.Millisecond)
	defer cancel()
	if err = client.Run(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("run returned %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if requests < 2 || requests > 4 {
		t.Fatalf("unexpected retry count %d", requests)
	}
}

func TestBeaconClassifiesNetworkLossAndRequestTimeoutAsTemporary(t *testing.T) {
	state := State{Version: 1, AgentID: "agent-test", Identity: "identity-test", IdentityRotatedAt: time.Now()}
	for _, endpoint := range []string{"http://127.0.0.1:1"} {
		statePath := filepath.Join(t.TempDir(), "state.json")
		content, _ := json.Marshal(state)
		_ = os.WriteFile(statePath, content, 0600)
		client, err := New(Config{Endpoint: endpoint, AgentID: "agent-test", Name: "test", StatePath: statePath, Interval: time.Second, ConnectTimeout: 100 * time.Millisecond, RequestTimeout: 150 * time.Millisecond, BatchSize: 1, AllowInsecureLoopback: true})
		if err != nil {
			t.Fatal(err)
		}
		if err = client.checkin(context.Background()); err == nil || !temporary(err) {
			t.Fatalf("network loss classification: %v", err)
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		w.Header().Set("X-Specter-Protocol-Version", "1")
	}))
	defer server.Close()
	statePath := filepath.Join(t.TempDir(), "state.json")
	content, _ := json.Marshal(state)
	_ = os.WriteFile(statePath, content, 0600)
	client, err := New(Config{Endpoint: server.URL, AgentID: "agent-test", Name: "test", StatePath: statePath, Interval: time.Second, ConnectTimeout: 100 * time.Millisecond, RequestTimeout: 150 * time.Millisecond, BatchSize: 1, AllowInsecureLoopback: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.checkin(context.Background()); err == nil || !temporary(err) {
		t.Fatalf("request timeout classification: %v", err)
	}
}

func TestBeaconTreatsAuthenticationFailureAsPermanent(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		http.Error(w, "denied", http.StatusUnauthorized)
	}))
	defer server.Close()
	statePath := filepath.Join(t.TempDir(), "state.json")
	content, _ := json.Marshal(State{Version: 1, AgentID: "agent-test", Identity: "identity-test"})
	_ = os.WriteFile(statePath, content, 0600)
	client, err := New(Config{Endpoint: server.URL, AgentID: "agent-test", Name: "test", StatePath: statePath, Interval: 100 * time.Millisecond, BatchSize: 1, AllowInsecureLoopback: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Run(context.Background()); err == nil || requests != 1 {
		t.Fatalf("err=%v requests=%d", err, requests)
	}
}

func TestBeaconClosesIdleConnectionsOnShutdown(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	content, _ := json.Marshal(State{Version: 1, AgentID: "agent-test", Identity: "identity-test", IdentityRotatedAt: time.Now()})
	if err := os.WriteFile(statePath, content, 0600); err != nil {
		t.Fatal(err)
	}
	transport := &closeTrackingTransport{}
	client, err := New(Config{Endpoint: "https://gateway.example.test", AgentID: "agent-test", Name: "test", StatePath: statePath, Interval: time.Second, BatchSize: 1, HTTPClient: &http.Client{Transport: transport}})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.Run(context.Background()); err == nil {
		t.Fatal("permanent authentication error was not returned")
	}
	if !transport.wasClosed() {
		t.Fatal("idle connections were not closed during shutdown")
	}
}

func TestBeaconRejectsResponseProtocolDowngrade(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		checkin, _ := gateway.DecodeCheckin(func() []byte { body, _ := io.ReadAll(r.Body); return body }())
		w.Header().Set("X-Specter-Protocol-Version", "0")
		frame, _ := gateway.EncodeTaskBatch(gateway.TaskBatch{Sequence: checkin.Sequence})
		_, _ = w.Write(frame)
	}))
	defer server.Close()
	statePath := filepath.Join(t.TempDir(), "state.json")
	content, _ := json.Marshal(State{Version: 1, AgentID: "agent-test", Identity: "identity-test", IdentityRotatedAt: time.Now()})
	_ = os.WriteFile(statePath, content, 0600)
	client, err := New(Config{Endpoint: server.URL, AgentID: "agent-test", Name: "test", StatePath: statePath, Interval: time.Second, BatchSize: 1, AllowInsecureLoopback: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.checkin(context.Background()); err == nil || temporary(err) {
		t.Fatalf("response downgrade was not permanent: %v", err)
	}
}

func TestBeaconRequiresTLSOutsideExplicitLoopback(t *testing.T) {
	_, err := New(Config{Endpoint: "http://example.test", AgentID: "agent", Name: "test", StatePath: filepath.Join(t.TempDir(), "state.json"), Interval: time.Second, BatchSize: 1})
	if err == nil {
		t.Fatal("plain HTTP endpoint accepted")
	}
	_, err = New(Config{Endpoint: "http://127.0.0.1:8080", AgentID: "agent", Name: "test", StatePath: filepath.Join(t.TempDir(), "state.json"), Interval: time.Second, BatchSize: 1})
	if err == nil {
		t.Fatal("loopback HTTP accepted without explicit opt-in")
	}
}

func TestBeaconCustomCAAndHostnameVerification(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Specter-Protocol-Version", "1")
		if r.URL.Path != "/api/agent/v1/enroll" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("X-Specter-Agent-ID", "agent-test")
		w.Header().Set("X-Specter-Identity", "identity-test")
		frame, _ := gateway.EncodeAck(gateway.Ack{})
		_, _ = w.Write(frame)
	}))
	defer server.Close()
	caPath := filepath.Join(t.TempDir(), "ca.pem")
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(caPath, certificate, 0600); err != nil {
		t.Fatal(err)
	}
	client, err := New(Config{Endpoint: server.URL, EnrollmentToken: "one-use", AgentID: "agent-test", Name: "test", StatePath: filepath.Join(t.TempDir(), "state.json"), CABundlePath: caPath, Interval: time.Second, BatchSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.enroll(context.Background()); err != nil {
		t.Fatalf("custom CA rejected: %v", err)
	}
	wrongHost := strings.Replace(server.URL, "127.0.0.1", "localhost", 1)
	client, err = New(Config{Endpoint: wrongHost, EnrollmentToken: "one-use", AgentID: "agent-test", Name: "test", StatePath: filepath.Join(t.TempDir(), "state.json"), CABundlePath: caPath, Interval: time.Second, BatchSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err = client.enroll(context.Background()); err == nil || temporary(err) {
		t.Fatalf("hostname mismatch was not a permanent failure: %v", err)
	}
}

func TestRunningSleepIsCancelledBySignedStatusPoll(t *testing.T) {
	polls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Specter-Protocol-Version", "1")
		if r.URL.Path != "/api/agent/v1/task-status" {
			http.NotFound(w, r)
			return
		}
		polls++
		w.Header().Set("X-Specter-Cancel-Requested", "true")
		frame, _ := gateway.EncodeAck(gateway.Ack{})
		_, _ = w.Write(frame)
	}))
	defer server.Close()
	statePath := filepath.Join(t.TempDir(), "state.json")
	content, _ := json.Marshal(State{Version: 1, AgentID: "agent-test", Identity: "identity-test", IdentityRotatedAt: time.Now()})
	_ = os.WriteFile(statePath, content, 0600)
	client, err := New(Config{Endpoint: server.URL, AgentID: "agent-test", Name: "test", StatePath: statePath, Interval: time.Second, BatchSize: 1, AllowInsecureLoopback: true})
	if err != nil {
		t.Fatal(err)
	}
	task := gateway.Task{Version: 1, ID: "task-sleep", IdempotencyKey: "idem-sleep", Cursor: 1, Kind: "sleep", Payload: []byte(`{"durationMs":2000}`), Deadline: time.Now().Add(time.Minute), Timeout: 3 * time.Second}
	started := time.Now()
	result, err := client.executeTask(context.Background(), task)
	if err != nil || result.Error == nil || result.Error.Code != "cancelled" || polls != 1 || time.Since(started) > time.Second {
		t.Fatalf("result=%#v polls=%d duration=%s err=%v", result, polls, time.Since(started), err)
	}
}
