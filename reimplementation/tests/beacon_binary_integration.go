//go:build !e2e_fixture

package main

import (
	"context"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/research/specter-reimplementation/internal/gateway"
)

func main() {
	binary := flag.String("binary", "dist/specter-beacon", "fresh beacon binary")
	version := flag.String("version", "dev", "expected embedded build version")
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	resultSeen := make(chan gateway.Result, 1)
	var once sync.Once
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Specter-Protocol-Version", strconv.Itoa(gateway.Version))
		body, _ := io.ReadAll(r.Body)
		if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			sequence, _ := strconv.ParseUint(r.Header.Get("X-Specter-Sequence"), 10, 64)
			if !gateway.VerifyRequestSignature("binary-identity", r.Method, r.URL.Path, r.Header.Get("X-Specter-Timestamp"), r.Header.Get("X-Specter-Nonce"), sequence, body, r.Header.Get("X-Specter-Signature"), time.Now().UTC()) {
				http.Error(w, "bad signature", http.StatusUnauthorized)
				return
			}
		}
		w.Header().Set("Content-Type", "application/vnd.specter.gateway+gzip")
		switch r.URL.Path {
		case "/api/agent/v1/enroll":
			registration, err := gateway.DecodeRegistration(body)
			if err != nil || registration.Build != *version {
				http.Error(w, "bad registration", http.StatusBadRequest)
				return
			}
			w.Header().Set("X-Specter-Agent-ID", "binary-agent")
			w.Header().Set("X-Specter-Identity", "binary-identity")
			frame, _ := gateway.EncodeAck(gateway.Ack{})
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write(frame)
		case "/api/agent/v1/checkin":
			checkin, err := gateway.DecodeCheckin(body)
			if err != nil {
				http.Error(w, "bad checkin", http.StatusBadRequest)
				return
			}
			batch := gateway.TaskBatch{Sequence: checkin.Sequence, Cursor: checkin.Cursor}
			once.Do(func() {
				batch.Cursor = 1
				batch.Tasks = []gateway.Task{{Version: 1, ID: "binary-task", IdempotencyKey: "binary-idempotency", Cursor: 1, Kind: "echo", Payload: []byte(`{"value":"fresh-binary"}`), Deadline: time.Now().Add(time.Minute), Timeout: time.Second}}
			})
			frame, _ := gateway.EncodeTaskBatch(batch)
			_, _ = w.Write(frame)
		case "/api/agent/v1/ack":
			receipt, err := gateway.DecodeReceipt(body)
			if err != nil || receipt.TaskID != "binary-task" {
				http.Error(w, "bad receipt", http.StatusBadRequest)
				return
			}
			frame, _ := gateway.EncodeAck(gateway.Ack{Sequence: receipt.Sequence, Cursor: receipt.Cursor})
			_, _ = w.Write(frame)
		case "/api/agent/v1/results":
			result, err := gateway.DecodeResult(body)
			if err != nil || !result.Success || string(result.Payload) != `{"value":"fresh-binary"}` {
				http.Error(w, "bad result", http.StatusBadRequest)
				return
			}
			select {
			case resultSeen <- result:
			default:
			}
			frame, _ := gateway.EncodeAck(gateway.Ack{Sequence: result.Sequence, Cursor: 1})
			_, _ = w.Write(frame)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	directory, err := os.MkdirTemp("", "specter-binary-integration-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(directory)
	caPath := filepath.Join(directory, "ca.pem")
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err = os.WriteFile(caPath, certificate, 0600); err != nil {
		panic(err)
	}
	command := exec.CommandContext(ctx, *binary,
		"-endpoint", server.URL,
		"-ca-bundle", caPath,
		"-enrollment-token", "one-use",
		"-agent-id", "binary-agent",
		"-state", filepath.Join(directory, "state.json"),
		"-interval", "100ms",
		"-jitter", "10ms",
		"-connect-timeout", "1s",
		"-request-timeout", "2s",
		"-identity-rotation", "1h",
		"-batch", "1",
	)
	output := &strings.Builder{}
	command.Stdout, command.Stderr = output, output
	if err = command.Start(); err != nil {
		panic(err)
	}
	select {
	case result := <-resultSeen:
		if result.TaskID != "binary-task" {
			panic("unexpected task result")
		}
	case <-ctx.Done():
		_ = command.Process.Kill()
		panic(fmt.Sprintf("binary integration timeout: %s", output.String()))
	}
	if runtime.GOOS == "windows" {
		if err = command.Process.Kill(); err != nil {
			panic(err)
		}
		_ = command.Wait()
	} else {
		if err = command.Process.Signal(os.Interrupt); err != nil {
			_ = command.Process.Kill()
			panic(err)
		}
		if err = command.Wait(); err != nil {
			panic(fmt.Sprintf("beacon exit: %v: %s", err, output.String()))
		}
	}
	fmt.Println("fresh beacon binary completed signed TLS echo lifecycle")
}
