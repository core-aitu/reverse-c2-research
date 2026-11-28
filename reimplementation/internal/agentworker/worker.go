// Package agentworker executes versioned beacon tasks inside a bounded pool.
package agentworker

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/research/specter-reimplementation/internal/gateway"
)

const (
	MaxParallel      = 4
	MaxEnvironment   = 32
	MaxArgumentCount = 256
	MaxRemoteFile    = int64(1 << 30)
)

type OutputSink func(context.Context, gateway.TaskOutputChunk) error
type FileSink func(context.Context, gateway.Task, FileDownloadResult) (any, error)
type handler func(context.Context, []byte, string) (any, error)

type CommandTaskPayload struct {
	Executable  string            `json:"executable,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Interpreter string            `json:"interpreter,omitempty"`
	Command     string            `json:"command,omitempty"`
	CWD         string            `json:"cwd,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
}

type CommandResult struct {
	ExitCode    int       `json:"exitCode"`
	StartedAt   time.Time `json:"startedAt"`
	CompletedAt time.Time `json:"completedAt"`
	DurationMS  int64     `json:"durationMs"`
	StdoutBytes int64     `json:"stdoutBytes"`
	StderrBytes int64     `json:"stderrBytes"`
	Truncated   bool      `json:"truncated"`
}

type FileDownloadPayload struct {
	Path string `json:"path"`
}

type FileDownloadResult struct {
	Path      string `json:"path"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	SHA256    string `json:"sha256"`
	MediaType string `json:"mediaType"`
}

type Worker struct {
	sem      chan struct{}
	handlers map[string]handler
}

type taskFailure struct {
	code, message string
	retryable     bool
}

func (e *taskFailure) Error() string { return e.message }

func New(parallel int) (*Worker, error) {
	if parallel < 1 || parallel > MaxParallel {
		return nil, errors.New("worker parallelism exceeds policy")
	}
	w := &Worker{sem: make(chan struct{}, parallel), handlers: map[string]handler{}}
	w.handlers["echo"] = echo
	w.handlers["system-info"] = systemInfo
	w.handlers["sleep"] = sleep
	return w, nil
}

func Capabilities() []string {
	values := []string{"echo", "system-info", "sleep", "exec", "file-download"}
	if runtime.GOOS == "windows" {
		values = append(values, "shell:cmd")
	} else if _, err := exec.LookPath("sh"); err == nil {
		values = append(values, "shell:sh")
	}
	if _, err := powerShellPath(); err == nil {
		values = append(values, "shell:powershell")
	}
	sort.Strings(values)
	return values
}

func (w *Worker) Execute(ctx context.Context, task gateway.Task, sequence uint64) gateway.Result {
	return w.ExecuteStreaming(ctx, task, sequence, nil, nil)
}

func (w *Worker) ExecuteStreaming(ctx context.Context, task gateway.Task, sequence uint64, output OutputSink, files FileSink) (result gateway.Result) {
	result = gateway.Result{TaskID: task.ID, SubmissionID: submissionID(task), Sequence: sequence}
	fail := func(code, message string, retryable bool) gateway.Result {
		result.Error = &gateway.TaskError{Code: code, Message: message, Retryable: retryable}
		return result
	}
	if task.CancelRequested {
		return fail("cancelled", "task cancellation requested", false)
	}
	if time.Now().After(task.Deadline) {
		return fail("deadline_exceeded", "task deadline expired", false)
	}
	if !gateway.ValidTaskKindForVersion(task.Version, task.Kind) {
		if _, testHandler := w.handlers[task.Kind]; !testHandler {
			return fail("unsupported_task", "task type or envelope version is not supported", false)
		}
	}
	select {
	case w.sem <- struct{}{}:
		defer func() { <-w.sem }()
	case <-ctx.Done():
		return fail("cancelled", "worker stopped", true)
	}
	deadline := time.Now().Add(task.Timeout)
	if task.Deadline.Before(deadline) {
		deadline = task.Deadline
	}
	taskContext, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	defer func() {
		if recover() != nil {
			result = fail("worker_crash", "task handler crashed", true)
		}
	}()

	var value any
	var err error
	switch task.Kind {
	case "exec", "shell":
		value, err = runCommand(taskContext, task, output)
	case "file-download":
		value, err = downloadFile(taskContext, task, files)
	default:
		temporary, tempErr := os.MkdirTemp("", "specter-task-*")
		if tempErr != nil {
			return fail("temporary_directory", "could not create isolated task directory", true)
		}
		defer os.RemoveAll(temporary)
		h, ok := w.handlers[task.Kind]
		if !ok {
			return fail("unsupported_task", "task type is not allowlisted", false)
		}
		value, err = h(taskContext, task.Payload, temporary)
	}
	if err != nil {
		var failure *taskFailure
		if errors.As(err, &failure) {
			return fail(failure.code, failure.message, failure.retryable)
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return fail("timeout", "task exceeded its wall-time limit", true)
		}
		if errors.Is(err, context.Canceled) {
			return fail("cancelled", "task was cancelled", true)
		}
		return fail("handler_error", "task handler failed", true)
	}
	encoded, err := json.Marshal(value)
	if err != nil || len(encoded) > gateway.MaxResultSize {
		return fail("output_limit", "task result metadata exceeds policy", false)
	}
	result.Success, result.Payload = true, encoded
	return result
}

func strictJSON(payload []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values")
	}
	return nil
}

func runCommand(ctx context.Context, task gateway.Task, sink OutputSink) (CommandResult, error) {
	var payload CommandTaskPayload
	if strictJSON(task.Payload, &payload) != nil || !validCommandPayload(task.Kind, payload) {
		return CommandResult{}, &taskFailure{code: "invalid_payload", message: "command payload exceeds policy"}
	}
	var executable string
	var args []string
	if task.Kind == "exec" {
		executable, args = payload.Executable, payload.Args
	} else {
		var err error
		executable, args, err = shellCommand(payload.Interpreter, payload.Command)
		if err != nil {
			return CommandResult{}, &taskFailure{code: "unsupported_interpreter", message: err.Error()}
		}
	}
	command := exec.Command(executable, args...)
	command.Dir = payload.CWD
	command.Env = mergedEnvironment(os.Environ(), payload.Env)
	prepareProcess(command)
	emitter := newOutputEmitter(ctx, task, sink)
	command.Stdout = emitter.writer("stdout")
	command.Stderr = emitter.writer("stderr")
	started := time.Now().UTC()
	if err := command.Start(); err != nil {
		return CommandResult{}, &taskFailure{code: "process_start", message: "process could not be started"}
	}
	tree, err := attachProcess(command)
	if err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return CommandResult{}, &taskFailure{code: "process_control", message: "process containment could not be established", retryable: true}
	}
	defer tree.close()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	var waitErr error
	select {
	case waitErr = <-done:
	case <-ctx.Done():
		tree.terminate()
		<-done
		_ = emitter.close()
		return CommandResult{}, ctx.Err()
	}
	if err := emitter.close(); err != nil {
		return CommandResult{}, &taskFailure{code: "output_delivery", message: "process output could not be delivered", retryable: true}
	}
	completed := time.Now().UTC()
	exitCode := command.ProcessState.ExitCode()
	if waitErr != nil {
		var exitError *exec.ExitError
		if !errors.As(waitErr, &exitError) {
			return CommandResult{}, &taskFailure{code: "process_wait", message: "process wait failed", retryable: true}
		}
	}
	stdout, stderr, truncated := emitter.stats()
	return CommandResult{ExitCode: exitCode, StartedAt: started, CompletedAt: completed, DurationMS: completed.Sub(started).Milliseconds(), StdoutBytes: stdout, StderrBytes: stderr, Truncated: truncated}, nil
}

func validCommandPayload(kind string, value CommandTaskPayload) bool {
	if len(value.CWD) > 4096 || strings.IndexByte(value.CWD, 0) >= 0 || len(value.Args) > MaxArgumentCount || len(value.Env) > MaxEnvironment {
		return false
	}
	for _, argument := range value.Args {
		if len(argument) > gateway.MaxStringSize || strings.IndexByte(argument, 0) >= 0 {
			return false
		}
	}
	for key, envValue := range value.Env {
		if !validEnvironmentKey(key) || len(envValue) > 4096 || strings.IndexByte(envValue, 0) >= 0 {
			return false
		}
	}
	if kind == "exec" {
		return value.Executable != "" && len(value.Executable) <= 4096 && strings.IndexByte(value.Executable, 0) < 0 && value.Interpreter == "" && value.Command == ""
	}
	return (value.Interpreter == "sh" || value.Interpreter == "cmd" || value.Interpreter == "powershell") && value.Command != "" && len(value.Command) <= gateway.MaxTaskSize/2 && value.Executable == "" && len(value.Args) == 0
}

func validEnvironmentKey(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for index, character := range value {
		if !(character == '_' || unicode.IsLetter(character) || index > 0 && unicode.IsDigit(character)) {
			return false
		}
	}
	return true
}

func mergedEnvironment(base []string, values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	replaced := map[string]bool{}
	for _, key := range keys {
		replaced[key] = true
	}
	result := make([]string, 0, len(base)+len(keys))
	for _, entry := range base {
		key, _, _ := strings.Cut(entry, "=")
		if !replaced[key] {
			result = append(result, entry)
		}
	}
	for _, key := range keys {
		result = append(result, key+"="+values[key])
	}
	return result
}

func shellCommand(interpreter, script string) (string, []string, error) {
	switch interpreter {
	case "sh":
		if runtime.GOOS == "windows" {
			return "", nil, errors.New("sh is not available on this platform")
		}
		path, err := exec.LookPath("sh")
		return path, []string{"-c", script}, err
	case "cmd":
		if runtime.GOOS != "windows" {
			return "", nil, errors.New("cmd is not available on this platform")
		}
		path, err := exec.LookPath("cmd.exe")
		return path, []string{"/S", "/C", script}, err
	case "powershell":
		path, err := powerShellPath()
		return path, []string{"-NoLogo", "-NoProfile", "-NonInteractive", "-Command", script}, err
	default:
		return "", nil, errors.New("unsupported interpreter")
	}
}

func powerShellPath() (string, error) {
	for _, candidate := range []string{"pwsh", "powershell.exe", "powershell"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", errors.New("PowerShell is not installed")
}

func downloadFile(ctx context.Context, task gateway.Task, sink FileSink) (any, error) {
	var payload FileDownloadPayload
	if strictJSON(task.Payload, &payload) != nil || payload.Path == "" || len(payload.Path) > 4096 || strings.IndexByte(payload.Path, 0) >= 0 {
		return nil, &taskFailure{code: "invalid_payload", message: "file path is invalid"}
	}
	file, err := os.Open(payload.Path)
	if err != nil {
		return nil, &taskFailure{code: "file_open", message: "file could not be opened"}
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() < 0 || info.Size() > MaxRemoteFile {
		return nil, &taskFailure{code: "file_policy", message: "file exceeds retrieval policy"}
	}
	digest := sha256.New()
	if _, err = io.Copy(digest, &contextReader{ctx: ctx, reader: file}); err != nil {
		return nil, err
	}
	result := FileDownloadResult{Path: payload.Path, Name: filepath.Base(payload.Path), Size: info.Size(), SHA256: hex.EncodeToString(digest.Sum(nil)), MediaType: "application/octet-stream"}
	if sink != nil {
		return sink(ctx, task, result)
	}
	return result, nil
}

type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

func (r *contextReader) Read(buffer []byte) (int, error) {
	select {
	case <-r.ctx.Done():
		return 0, r.ctx.Err()
	default:
		return r.reader.Read(buffer)
	}
}

func submissionID(task gateway.Task) string { return task.ID + ":" + task.IdempotencyKey }

func echo(_ context.Context, payload []byte, _ string) (any, error) {
	var request struct {
		Value string `json:"value"`
	}
	if strictJSON(payload, &request) != nil || request.Value == "" || len(request.Value) > gateway.MaxTaskSize/2 {
		return nil, &taskFailure{code: "invalid_payload", message: "echo value is required"}
	}
	return request, nil
}

func systemInfo(_ context.Context, payload []byte, _ string) (any, error) {
	var request struct{}
	if strictJSON(payload, &request) != nil {
		return nil, &taskFailure{code: "invalid_payload", message: "system-info payload must be an empty object"}
	}
	hostname, _ := os.Hostname()
	return map[string]any{"os": runtime.GOOS, "architecture": runtime.GOARCH, "hostname": hostname, "capabilities": Capabilities()}, nil
}

func sleep(ctx context.Context, payload []byte, _ string) (any, error) {
	var request struct {
		DurationMS int64 `json:"durationMs"`
	}
	if strictJSON(payload, &request) != nil || request.DurationMS < 0 || request.DurationMS > gateway.MaxTaskTimeout.Milliseconds() {
		return nil, &taskFailure{code: "invalid_payload", message: "sleep duration exceeds policy"}
	}
	timer := time.NewTimer(time.Duration(request.DurationMS) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return map[string]int64{"sleptMs": request.DurationMS}, nil
	}
}

type outputEmitter struct {
	ctx         context.Context
	task        gateway.Task
	sink        OutputSink
	mu          sync.Mutex
	indexes     map[string]uint32
	stdout      int64
	stderr      int64
	total       int64
	truncated   bool
	closed      bool
	deliveryErr error
}

type outputWriter struct {
	emitter *outputEmitter
	stream  string
}

func newOutputEmitter(ctx context.Context, task gateway.Task, sink OutputSink) *outputEmitter {
	return &outputEmitter{ctx: ctx, task: task, sink: sink, indexes: map[string]uint32{"stdout": 0, "stderr": 0}}
}

func (e *outputEmitter) writer(stream string) io.Writer {
	return &outputWriter{emitter: e, stream: stream}
}

func (w *outputWriter) Write(content []byte) (int, error) {
	w.emitter.mu.Lock()
	defer w.emitter.mu.Unlock()
	if w.emitter.deliveryErr != nil {
		return 0, w.emitter.deliveryErr
	}
	original := len(content)
	remaining := int64(gateway.MaxTaskOutputSize) - w.emitter.total
	if remaining <= 0 {
		w.emitter.truncated = true
		return original, nil
	}
	if int64(len(content)) > remaining {
		content = content[:remaining]
		w.emitter.truncated = true
	}
	for len(content) > 0 {
		size := len(content)
		if size > gateway.MaxOutputChunkSize {
			size = gateway.MaxOutputChunkSize
		}
		piece := append([]byte(nil), content[:size]...)
		chunk := gateway.TaskOutputChunk{TaskID: w.emitter.task.ID, SubmissionID: submissionID(w.emitter.task), Stream: w.stream, Index: w.emitter.indexes[w.stream], Data: piece}
		if w.emitter.sink != nil {
			if err := w.emitter.sink(w.emitter.ctx, chunk); err != nil {
				w.emitter.deliveryErr = err
				return 0, err
			}
		}
		w.emitter.indexes[w.stream]++
		w.emitter.total += int64(size)
		if w.stream == "stdout" {
			w.emitter.stdout += int64(size)
		} else {
			w.emitter.stderr += int64(size)
		}
		content = content[size:]
	}
	return original, nil
}

func (e *outputEmitter) close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return e.deliveryErr
	}
	e.closed = true
	for _, stream := range []string{"stdout", "stderr"} {
		chunk := gateway.TaskOutputChunk{TaskID: e.task.ID, SubmissionID: submissionID(e.task), Stream: stream, Index: e.indexes[stream], EOF: true, Truncated: e.truncated}
		if e.sink != nil && e.deliveryErr == nil {
			e.deliveryErr = e.sink(e.ctx, chunk)
		}
	}
	return e.deliveryErr
}

func (e *outputEmitter) stats() (int64, int64, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stdout, e.stderr, e.truncated
}
