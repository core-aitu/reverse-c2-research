package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"

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
	command := exec.CommandContext(ctx, binary)
	command.Env = []string{"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"}
	command.Dir = "/tmp"
	command.Stdin = bytes.NewReader(input)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		if stderr.Len() > 1024 {
			return pipeline.Result{}, errors.New("isolated worker failed with oversized diagnostics")
		}
		return pipeline.Result{}, errors.New("isolated worker failed: " + stderr.String())
	}
	if stdout.Len() > 5<<20 {
		return pipeline.Result{}, errors.New("isolated worker response exceeds 5 MiB")
	}
	var response Response
	decoder := json.NewDecoder(&stdout)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&response); err != nil || response.Version != 1 {
		return pipeline.Result{}, errors.New("invalid isolated worker response")
	}
	if response.Error != "" {
		return pipeline.Result{}, errors.New(response.Error)
	}
	return response.Result, nil
}
