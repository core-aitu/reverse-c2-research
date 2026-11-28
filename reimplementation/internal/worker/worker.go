package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/research/specter-reimplementation/internal/pipeline"
)

type Request struct {
	Version    int                 `json:"version"`
	Definition pipeline.Definition `json:"definition"`
	Input      string              `json:"input"`
}

type Response struct {
	Version int             `json:"version"`
	Result  pipeline.Result `json:"result"`
	Error   string          `json:"error,omitempty"`
}

func Run(request Request) Response {
	if request.Version != 1 {
		return Response{Version: 1, Error: "unsupported worker protocol version"}
	}
	result, err := pipeline.Execute(request.Definition, request.Input)
	if err != nil {
		return Response{Version: 1, Error: err.Error()}
	}
	return Response{Version: 1, Result: result}
}

func ExecuteProcess(ctx context.Context, binary string, request Request) (pipeline.Result, error) {
	input, err := json.Marshal(request)
	if err != nil {
		return pipeline.Result{}, err
	}
	if len(input) > 4<<20 {
		return pipeline.Result{}, errors.New("isolated worker request exceeds 4 MiB")
	}
	directory, err := os.MkdirTemp("", "specter-worker-")
	if err != nil {
		return pipeline.Result{}, errors.New("could not create isolated worker directory")
	}
	defer os.RemoveAll(directory)
	command := exec.CommandContext(ctx, binary)
	command.Env = isolatedEnvironment()
	command.Dir = directory
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return os.ErrProcessDone
		}
		return syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	}
	command.WaitDelay = time.Second
	command.Stdin = bytes.NewReader(input)
	stdout := cappedBuffer{maximum: 5 << 20}
	stderr := cappedBuffer{maximum: 1 << 10}
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		if stderr.overflow {
			return pipeline.Result{}, errors.New("isolated worker failed with oversized diagnostics")
		}
		return pipeline.Result{}, errors.New("isolated worker failed: " + stderr.String())
	}
	if stdout.overflow {
		return pipeline.Result{}, errors.New("isolated worker response exceeds 5 MiB")
	}
	var response Response
	decoder := json.NewDecoder(&stdout)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil || response.Version != 1 {
		return pipeline.Result{}, errors.New("invalid isolated worker response")
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return pipeline.Result{}, errors.New("invalid trailing isolated worker response")
	}
	if response.Error != "" {
		return pipeline.Result{}, errors.New(response.Error)
	}
	return response.Result, nil
}

func isolatedEnvironment() []string {
	return []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin", "HOME=/nonexistent", "LANG=C.UTF-8"}
}

type cappedBuffer struct {
	bytes.Buffer
	maximum  int
	overflow bool
}

func (buffer *cappedBuffer) Write(value []byte) (int, error) {
	original := len(value)
	remaining := buffer.maximum - buffer.Len()
	if remaining <= 0 {
		buffer.overflow = true
		return original, nil
	}
	if len(value) > remaining {
		value = value[:remaining]
		buffer.overflow = true
	}
	_, _ = buffer.Buffer.Write(value)
	return original, nil
}
