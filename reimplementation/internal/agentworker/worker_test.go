package agentworker

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/research/specter-reimplementation/internal/gateway"
)

func task(kind string, payload string) gateway.Task {
	return gateway.Task{Version: gateway.TaskEnvelopeVersion, ID: "task-1", IdempotencyKey: "idem-1", Kind: kind, Payload: []byte(payload), Deadline: time.Now().Add(time.Minute), Timeout: time.Second}
}

func commandTask(kind string, payload any) gateway.Task {
	encoded, _ := json.Marshal(payload)
	return gateway.Task{Version: gateway.CommandTaskEnvelopeVersion, ID: "task-command", IdempotencyKey: "idem-command", Kind: kind, Payload: encoded, Deadline: time.Now().Add(time.Minute), Timeout: 5 * time.Second}
}

func TestRepeatedTimeoutsLeaveNoWorkersOrTemporaryDirectories(t *testing.T) {
	beforeDirectories, err := filepath.Glob(filepath.Join(os.TempDir(), "specter-task-*"))
	if err != nil {
		t.Fatal(err)
	}
	beforeGoroutines := runtime.NumGoroutine()
	w, _ := New(2)
	for index := 0; index < 100; index++ {
		value := task("sleep", `{"durationMs":20}`)
		value.ID = "leak-check"
		value.Timeout = time.Millisecond
		if result := w.Execute(context.Background(), value, uint64(index)); result.Error == nil || result.Error.Code != "timeout" {
			t.Fatalf("timeout result: %#v", result)
		}
	}
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	afterDirectories, err := filepath.Glob(filepath.Join(os.TempDir(), "specter-task-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(afterDirectories) != len(beforeDirectories) {
		t.Fatalf("temporary directory leak: before=%d after=%d", len(beforeDirectories), len(afterDirectories))
	}
	if after := runtime.NumGoroutine(); after > beforeGoroutines+2 {
		t.Fatalf("goroutine leak: before=%d after=%d", beforeGoroutines, after)
	}
}

func TestSafeHandlers(t *testing.T) {
	w, _ := New(1)
	for _, tc := range []struct {
		kind, payload string
	}{
		{"echo", `{"value":"hello"}`},
		{"system-info", `{}`},
		{"sleep", `{"durationMs":1}`},
	} {
		result := w.Execute(context.Background(), task(tc.kind, tc.payload), 1)
		if !result.Success || result.Error != nil {
			t.Fatalf("%s failed: %#v", tc.kind, result)
		}
	}
}

func TestTimeoutCancellationCrashAndOutputLimit(t *testing.T) {
	w, _ := New(1)
	timed := task("sleep", `{"durationMs":100}`)
	timed.Timeout = 10 * time.Millisecond
	if got := w.Execute(context.Background(), timed, 1); got.Error == nil || got.Error.Code != "timeout" {
		t.Fatalf("timeout result: %#v", got)
	}
	cancelled := task("sleep", `{"durationMs":100}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := w.Execute(ctx, cancelled, 2); got.Error == nil || got.Error.Code != "cancelled" {
		t.Fatalf("cancel result: %#v", got)
	}
	w.handlers["echo"] = func(context.Context, []byte, string) (any, error) { panic("boom") }
	if got := w.Execute(context.Background(), task("echo", `{}`), 3); got.Error == nil || got.Error.Code != "worker_crash" {
		t.Fatalf("crash result: %#v", got)
	}
	w.handlers["echo"] = func(context.Context, []byte, string) (any, error) {
		return strings.Repeat("x", gateway.MaxResultSize+1), nil
	}
	if got := w.Execute(context.Background(), task("echo", `{}`), 4); got.Error == nil || got.Error.Code != "output_limit" {
		t.Fatalf("output result: %#v", got)
	}
}

func TestUnknownAndCancelledEnvelope(t *testing.T) {
	w, _ := New(1)
	if got := w.Execute(context.Background(), task("shell", `{}`), 1); got.Error == nil || got.Error.Code != "unsupported_task" {
		t.Fatalf("unknown result: %#v", got)
	}
	cancelled := task("echo", `{"value":"never"}`)
	cancelled.CancelRequested = true
	if got := w.Execute(context.Background(), cancelled, 2); got.Error == nil || got.Error.Code != "cancelled" {
		t.Fatalf("cancelled envelope: %#v", got)
	}
}

func TestExecAndShellStreamBoundedOutput(t *testing.T) {
	w, _ := New(1)
	var chunks []gateway.TaskOutputChunk
	sink := func(_ context.Context, chunk gateway.TaskOutputChunk) error {
		chunks = append(chunks, chunk)
		return nil
	}
	value := commandTask("exec", CommandTaskPayload{Executable: os.Args[0], Args: []string{"-test.run=TestCommandHelperProcess", "--", "hello"}, Env: map[string]string{"GO_WANT_COMMAND_HELPER": "1"}})
	result := w.ExecuteStreaming(context.Background(), value, 1, sink, nil)
	if !result.Success || result.Error != nil {
		t.Fatalf("exec failed: %#v", result)
	}
	var metadata CommandResult
	if err := json.Unmarshal(result.Payload, &metadata); err != nil || metadata.ExitCode != 0 || metadata.StdoutBytes == 0 {
		t.Fatalf("metadata=%#v err=%v", metadata, err)
	}
	if len(chunks) < 3 || chunks[len(chunks)-2].Stream != "stdout" || !chunks[len(chunks)-2].EOF || chunks[len(chunks)-1].Stream != "stderr" || !chunks[len(chunks)-1].EOF {
		t.Fatalf("unexpected chunks: %#v", chunks)
	}
	if runtime.GOOS != "windows" {
		chunks = nil
		shell := commandTask("shell", CommandTaskPayload{Interpreter: "sh", Command: "printf shell-ok"})
		result = w.ExecuteStreaming(context.Background(), shell, 2, sink, nil)
		if !result.Success || len(chunks) < 3 || string(chunks[0].Data) != "shell-ok" {
			t.Fatalf("shell result=%#v chunks=%#v", result, chunks)
		}
	} else {
		chunks = nil
		shell := commandTask("shell", CommandTaskPayload{Interpreter: "cmd", Command: "echo shell-ok"})
		result = w.ExecuteStreaming(context.Background(), shell, 2, sink, nil)
		if !result.Success || len(chunks) < 3 || !strings.Contains(string(chunks[0].Data), "shell-ok") {
			t.Fatalf("cmd result=%#v chunks=%#v", result, chunks)
		}
	}
	if _, err := powerShellPath(); err == nil {
		chunks = nil
		shell := commandTask("shell", CommandTaskPayload{Interpreter: "powershell", Command: "Write-Output powershell-ok"})
		result = w.ExecuteStreaming(context.Background(), shell, 3, sink, nil)
		if !result.Success || len(chunks) < 3 || !strings.Contains(string(chunks[0].Data), "powershell-ok") {
			t.Fatalf("powershell result=%#v chunks=%#v", result, chunks)
		}
	}
}

func TestCommandHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_COMMAND_HELPER") != "1" {
		return
	}
	_, _ = os.Stdout.WriteString("hello from helper")
	os.Exit(0)
}

func TestCommandCancellationAndOutputTruncation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix process-group assertion")
	}
	w, _ := New(1)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	value := commandTask("shell", CommandTaskPayload{Interpreter: "sh", Command: "sleep 10"})
	started := time.Now()
	result := w.ExecuteStreaming(ctx, value, 1, nil, nil)
	if result.Error == nil || (result.Error.Code != "timeout" && result.Error.Code != "cancelled") || time.Since(started) > 2*time.Second {
		t.Fatalf("cancellation result=%#v duration=%s", result, time.Since(started))
	}
	emitter := newOutputEmitter(context.Background(), value, nil)
	content := make([]byte, gateway.MaxTaskOutputSize+1024)
	if n, err := emitter.writer("stdout").Write(content); err != nil || n != len(content) {
		t.Fatalf("write n=%d err=%v", n, err)
	}
	stdout, _, truncated := emitter.stats()
	if stdout != gateway.MaxTaskOutputSize || !truncated {
		t.Fatalf("stdout=%d truncated=%v", stdout, truncated)
	}
}

func TestFileDownloadMetadataAndSink(t *testing.T) {
	path := filepath.Join(t.TempDir(), "report.bin")
	if err := os.WriteFile(path, []byte("fixture-content"), 0600); err != nil {
		t.Fatal(err)
	}
	w, _ := New(1)
	called := false
	value := commandTask("file-download", FileDownloadPayload{Path: path})
	result := w.ExecuteStreaming(context.Background(), value, 1, nil, func(_ context.Context, _ gateway.Task, metadata FileDownloadResult) (any, error) {
		called = true
		return metadata, nil
	})
	if !result.Success || !called || !strings.Contains(string(result.Payload), `"size":15`) {
		t.Fatalf("result=%#v called=%v", result, called)
	}
}
