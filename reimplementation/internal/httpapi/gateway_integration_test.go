package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/research/specter-reimplementation/internal/auth"
	"github.com/research/specter-reimplementation/internal/beacon"
	"github.com/research/specter-reimplementation/internal/config"
	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/events"
	"github.com/research/specter-reimplementation/internal/objectstore"
	"github.com/research/specter-reimplementation/internal/store"
)

type integrationHandler struct{ handler http.Handler }

func TestGatewayPostgresBeaconLifecycleAndRestarts(t *testing.T) {
	databaseURL := os.Getenv("SPECTER_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SPECTER_TEST_DATABASE_URL is not set")
	}
	soakDuration := time.Duration(0)
	if value := os.Getenv("SPECTER_SOAK_DURATION"); value != "" {
		var parseErr error
		soakDuration, parseErr = time.ParseDuration(value)
		if parseErr != nil || soakDuration < time.Minute {
			t.Fatalf("SPECTER_SOAK_DURATION must be a duration of at least one minute: %q", value)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second+soakDuration)
	defer cancel()
	storage, err := store.Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if err = storage.Migrate(ctx, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	var objects objectstore.Store = objectstore.NewMemory()
	if endpoint := os.Getenv("SPECTER_TEST_S3_ENDPOINT"); endpoint != "" {
		objects, err = objectstore.NewS3(ctx, objectstore.S3Config{
			Endpoint: endpoint, AccessKey: os.Getenv("SPECTER_TEST_S3_ACCESS_KEY"),
			SecretKey: os.Getenv("SPECTER_TEST_S3_SECRET_KEY"), Bucket: os.Getenv("SPECTER_TEST_S3_BUCKET"), Region: "us-east-1",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	storage.AttachObjectStore(objects)
	suffix := fmt.Sprint(time.Now().UnixNano())
	agentID, enrollmentID, enrollmentToken := "e2e-agent-"+suffix, "e2e-enroll-"+suffix, "e2e-token-"+suffix
	if err = storage.CreateAgentEnrollment(ctx, enrollmentID, enrollmentToken, "integration", time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{AllowedOrigin: "https://127.0.0.1", TransferDir: t.TempDir()}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	makeHandler := func() http.Handler {
		return New(cfg, storage, auth.New(storage, "01234567890123456789012345678901", 15*time.Minute, time.Hour), events.NewHub(), logger)
	}
	var active atomic.Value
	active.Store(integrationHandler{handler: makeHandler()})
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		active.Load().(integrationHandler).handler.ServeHTTP(w, r)
	}))
	defer tlsServer.Close()
	caPath := filepath.Join(t.TempDir(), "gateway-ca.pem")
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: tlsServer.Certificate().Raw})
	if err = os.WriteFile(caPath, certificate, 0600); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(t.TempDir(), "beacon-state.json")
	newClient := func(token string) *beacon.Client {
		client, clientErr := beacon.New(beacon.Config{Endpoint: tlsServer.URL, EnrollmentToken: token, AgentID: agentID, Name: "integration", StatePath: statePath, BuildVersion: "integration", CABundlePath: caPath, Interval: 100 * time.Millisecond, Jitter: time.Millisecond, MinBackoff: 100 * time.Millisecond, MaxBackoff: 300 * time.Millisecond, ConnectTimeout: time.Second, RequestTimeout: 2 * time.Second, IdentityRotationInterval: time.Hour, BatchSize: 2})
		if clientErr != nil {
			t.Fatal(clientErr)
		}
		return client
	}
	run := func(client *beacon.Client) (context.CancelFunc, <-chan error) {
		clientContext, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- client.Run(clientContext) }()
		return stop, done
	}
	stop, done := run(newClient(enrollmentToken))
	waitFor(t, 8*time.Second, func() bool {
		values, queryErr := storage.AgentIdentities(ctx)
		if queryErr != nil {
			return false
		}
		for _, value := range values {
			if value.ID == agentID {
				return true
			}
		}
		return false
	})
	queueIntegrationTask(t, ctx, storage, agentID, "first-"+suffix, "before-restart")
	waitTaskStatus(t, ctx, storage, agentID, "first-"+suffix, domain.TaskCompleted)

	execID := "exec-" + suffix
	queueV2IntegrationTask(t, ctx, storage, agentID, execID, "exec", map[string]any{"executable": "/bin/echo", "args": []string{"exec-stream-ok"}}, 5000)
	waitTaskStatus(t, ctx, storage, agentID, execID, domain.TaskCompleted)
	assertArchivedOutput(t, ctx, storage, execID, "exec-stream-ok")

	shellID := "shell-" + suffix
	queueV2IntegrationTask(t, ctx, storage, agentID, shellID, "shell", map[string]any{"interpreter": "sh", "command": "printf shell-stream-ok"}, 5000)
	waitTaskStatus(t, ctx, storage, agentID, shellID, domain.TaskCompleted)
	assertArchivedOutput(t, ctx, storage, shellID, "shell-stream-ok")

	remotePath := filepath.Join(t.TempDir(), "remote-file.bin")
	remoteContent := []byte("remote-file-content-" + suffix)
	if err = os.WriteFile(remotePath, remoteContent, 0600); err != nil {
		t.Fatal(err)
	}
	fileID := "file-" + suffix
	queueV2IntegrationTask(t, ctx, storage, agentID, fileID, "file-download", map[string]any{"path": remotePath}, 5000)
	waitTaskStatus(t, ctx, storage, agentID, fileID, domain.TaskCompleted)
	assertRetrievedFile(t, ctx, storage, agentID, fileID, remoteContent)
	emptyPath := filepath.Join(t.TempDir(), "empty-file.bin")
	if err = os.WriteFile(emptyPath, nil, 0600); err != nil {
		t.Fatal(err)
	}
	emptyFileID := "empty-file-" + suffix
	queueV2IntegrationTask(t, ctx, storage, agentID, emptyFileID, "file-download", map[string]any{"path": emptyPath}, 5000)
	waitTaskStatus(t, ctx, storage, agentID, emptyFileID, domain.TaskCompleted)
	assertRetrievedFile(t, ctx, storage, agentID, emptyFileID, nil)

	cancelID := "cancel-" + suffix
	queueV2IntegrationTask(t, ctx, storage, agentID, cancelID, "shell", map[string]any{"interpreter": "sh", "command": "sleep 30"}, 30000)
	waitTaskStatus(t, ctx, storage, agentID, cancelID, domain.TaskRunning)
	if _, err = storage.CancelGatewayTask(ctx, agentID, cancelID); err != nil {
		t.Fatal(err)
	}
	waitTaskStatus(t, ctx, storage, agentID, cancelID, domain.TaskCancelled)

	gatewayRestartID := "gateway-restart-" + suffix
	queueV2IntegrationTask(t, ctx, storage, agentID, gatewayRestartID, "shell", map[string]any{"interpreter": "sh", "command": "sleep 1; printf gateway-restart-ok"}, 5000)
	waitTaskStatus(t, ctx, storage, agentID, gatewayRestartID, domain.TaskRunning)
	active.Store(integrationHandler{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "restarting", http.StatusServiceUnavailable)
	})})
	time.Sleep(400 * time.Millisecond)
	active.Store(integrationHandler{handler: makeHandler()})
	waitTaskStatus(t, ctx, storage, agentID, gatewayRestartID, domain.TaskCompleted)
	assertArchivedOutput(t, ctx, storage, gatewayRestartID, "gateway-restart-ok")

	beaconRestartID := "beacon-restart-" + suffix
	beaconRestartMarker := filepath.Join(t.TempDir(), "beacon-restart-count")
	beaconRestartCommand := fmt.Sprintf("printf 'once\\n' >> %q; sleep 30", beaconRestartMarker)
	queueV2IntegrationTask(t, ctx, storage, agentID, beaconRestartID, "shell", map[string]any{"interpreter": "sh", "command": beaconRestartCommand}, 30000)
	waitTaskStatus(t, ctx, storage, agentID, beaconRestartID, domain.TaskRunning)
	waitFor(t, 8*time.Second, func() bool {
		content, readErr := os.ReadFile(beaconRestartMarker)
		return readErr == nil && bytes.Count(content, []byte("once\n")) == 1
	})
	stop()
	<-done
	stop, done = run(newClient(""))
	waitTaskStatus(t, ctx, storage, agentID, beaconRestartID, domain.TaskFailed)
	markerContent, err := os.ReadFile(beaconRestartMarker)
	if err != nil || bytes.Count(markerContent, []byte("once\n")) != 1 {
		t.Fatalf("beacon restart side effect=%q err=%v", markerContent, err)
	}

	active.Store(integrationHandler{handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "restarting", http.StatusServiceUnavailable)
	})})
	queueIntegrationTask(t, ctx, storage, agentID, "second-"+suffix, "after-restart")
	time.Sleep(350 * time.Millisecond)
	active.Store(integrationHandler{handler: makeHandler()})
	waitTaskStatus(t, ctx, storage, agentID, "second-"+suffix, domain.TaskCompleted)
	if soakDuration > 0 {
		soakFile := filepath.Join(t.TempDir(), "soak-file.txt")
		soakContent := []byte("soak-file-content")
		if err = os.WriteFile(soakFile, soakContent, 0600); err != nil {
			t.Fatal(err)
		}
		baselineGoroutines := runtime.NumGoroutine()
		baselineFDs := openFileDescriptorCount()
		soakDeadline := time.Now().Add(soakDuration)
		for iteration := 0; time.Now().Before(soakDeadline); iteration++ {
			id := fmt.Sprintf("soak-%d-%s", iteration, suffix)
			switch iteration % 4 {
			case 0:
				queueIntegrationTask(t, ctx, storage, agentID, id, "continuous-check")
			case 1:
				queueV2IntegrationTask(t, ctx, storage, agentID, id, "exec", map[string]any{"executable": "/bin/echo", "args": []string{"soak-exec"}}, 5000)
			case 2:
				queueV2IntegrationTask(t, ctx, storage, agentID, id, "shell", map[string]any{"interpreter": "sh", "command": "printf soak-shell"}, 5000)
			case 3:
				queueV2IntegrationTask(t, ctx, storage, agentID, id, "file-download", map[string]any{"path": soakFile}, 5000)
			}
			waitTaskStatus(t, ctx, storage, agentID, id, domain.TaskCompleted)
			soakTask, taskErr := storage.GatewayTask(ctx, agentID, id)
			if taskErr != nil || soakTask.DispatchCount != 1 {
				t.Fatalf("soak task dispatch id=%s count=%d err=%v", id, soakTask.DispatchCount, taskErr)
			}
			switch iteration % 4 {
			case 1:
				assertArchivedOutput(t, ctx, storage, id, "soak-exec")
			case 2:
				assertArchivedOutput(t, ctx, storage, id, "soak-shell")
			case 3:
				assertRetrievedFile(t, ctx, storage, agentID, id, soakContent)
			}
			if iteration > 0 && iteration%60 == 0 {
				runtime.GC()
				if current := runtime.NumGoroutine(); current > baselineGoroutines+32 {
					t.Fatalf("soak goroutine growth: baseline=%d current=%d", baselineGoroutines, current)
				}
				if current := openFileDescriptorCount(); baselineFDs >= 0 && current > baselineFDs+16 {
					t.Fatalf("soak descriptor growth: baseline=%d current=%d", baselineFDs, current)
				}
			}
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			case <-time.After(time.Second):
			}
		}
	}
	stop()
	<-done

	tasks, err := storage.GatewayTasks(ctx, agentID)
	if err != nil {
		t.Fatal(err)
	}
	minimumTasks := 9
	if len(tasks) < minimumTasks {
		t.Fatalf("tasks=%d", len(tasks))
	}
	for _, task := range tasks {
		if task.ID == cancelID {
			if task.Status != domain.TaskCancelled || task.DispatchCount != 1 {
				t.Fatalf("cancelled task after lifecycle: %#v", task)
			}
			continue
		}
		if task.ID == beaconRestartID {
			if task.Status != domain.TaskFailed || task.DispatchCount != 1 || task.ErrorCode != "cancelled" {
				t.Fatalf("beacon-restarted task after lifecycle: %#v", task)
			}
			continue
		}
		if task.Status != domain.TaskCompleted || task.DispatchCount != 1 {
			t.Fatalf("task after restart: %#v", task)
		}
	}
}

func openFileDescriptorCount() int {
	entries, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return -1
	}
	return len(entries)
}

func queueV2IntegrationTask(t *testing.T, ctx context.Context, storage *store.Store, agentID, id, kind string, payload any, timeoutMS int64) {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	_, err = storage.CreateGatewayTask(ctx, domain.GatewayTask{ID: id, AgentID: agentID, Version: 2, IdempotencyKey: "idem-" + id, Kind: kind, Payload: encoded, Deadline: time.Now().Add(time.Minute), TimeoutMS: timeoutMS})
	if err != nil {
		t.Fatal(err)
	}
}

func assertArchivedOutput(t *testing.T, ctx context.Context, storage *store.Store, taskID, expected string) {
	t.Helper()
	outputs, err := storage.TaskOutputObjects(ctx, taskID)
	if err != nil || len(outputs) != 2 {
		t.Fatalf("task outputs=%#v err=%v", outputs, err)
	}
	for _, output := range outputs {
		reader, _, openErr := storage.OpenObject(ctx, output.Object.ID, 0, -1)
		if openErr != nil {
			t.Fatal(openErr)
		}
		content, readErr := io.ReadAll(reader)
		reader.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if output.Stream == "stdout" && !bytes.Contains(content, []byte(expected)) {
			t.Fatalf("stdout %q does not contain %q", content, expected)
		}
	}
}

func assertRetrievedFile(t *testing.T, ctx context.Context, storage *store.Store, agentID, taskID string, expected []byte) {
	t.Helper()
	task, err := storage.GatewayTask(ctx, agentID, taskID)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		ObjectID string `json:"objectId"`
	}
	if err = json.Unmarshal(task.Result, &result); err != nil || result.ObjectID == "" {
		t.Fatalf("file result=%s err=%v", task.Result, err)
	}
	reader, _, err := storage.OpenObject(ctx, result.ObjectID, 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || !bytes.Equal(content, expected) {
		t.Fatalf("retrieved content=%q err=%v", content, err)
	}
}

func queueIntegrationTask(t *testing.T, ctx context.Context, storage *store.Store, agentID, id, value string) {
	t.Helper()
	_, err := storage.CreateGatewayTask(ctx, domain.GatewayTask{ID: id, AgentID: agentID, Version: 1, IdempotencyKey: "idem-" + id, Kind: "echo", Payload: json.RawMessage(fmt.Sprintf(`{"value":%q}`, value)), Deadline: time.Now().Add(time.Minute), TimeoutMS: 5000})
	if err != nil {
		t.Fatal(err)
	}
}

func waitTaskStatus(t *testing.T, ctx context.Context, storage *store.Store, agentID, id, status string) {
	t.Helper()
	waitFor(t, 8*time.Second, func() bool {
		values, err := storage.GatewayTasks(ctx, agentID)
		if err != nil {
			return false
		}
		for _, value := range values {
			if value.ID == id {
				return value.Status == status
			}
		}
		return false
	})
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal("condition timed out")
}
