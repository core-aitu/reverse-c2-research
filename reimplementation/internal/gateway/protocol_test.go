package gateway

import (
	"bytes"
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestGoldenVectorCompatibility(t *testing.T) {
	registration := Registration{Build: "fixture", HostID: "host-01", Architecture: "X64", AgentID: "0123456789abcdef0123456789abcdef", FQDN: "lab.example", OS: "Linux", Username: "user", Path: "/opt/beacon", PID: 42, Interval: 500 * time.Millisecond, Window: 1500 * time.Millisecond, Expiration: time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)}
	deadline := time.Date(2026, 11, 26, 12, 5, 0, 0, time.UTC)
	cases := []struct {
		name   string
		encode func() ([]byte, error)
		golden string
	}{
		{"registration", func() ([]byte, error) { return EncodeRegistration(registration) }, "H4sIAAAAAAAE/2JPy6woKS1KZc/ILy7RNTBkjjAzUTAwNDI2MTUzt7BMTEpOSU1D53PnJCbppVYk5hbkpLL6ZOaVVrCUFqcWcevnF5ToJ6UmJufnaTFggi+MqAKMDAwMXxgZGBgWiJzZ4n3fAzAAh/M9N4wAAAA="},
		{"empty check-in", func() ([]byte, error) { return EncodeTaskBatch(TaskBatch{Sequence: 1}) }, "H4sIAAAAAAAE/wAUAOv/AQAAAAAAAAAAAAAAAAAAAAAAAAADAMugsmoUAAAA"},
		{"multi-task check-in", func() ([]byte, error) {
			return EncodeTaskBatch(TaskBatch{Sequence: 9, Cursor: 12, Tasks: []Task{{Version: 1, ID: "task-1", IdempotencyKey: "idem-1", Cursor: 11, Kind: "echo", Payload: []byte(`{"value":"one"}`), Deadline: deadline, Timeout: time.Second}, {Version: 1, ID: "task-2", IdempotencyKey: "idem-2", Cursor: 12, Kind: "echo", Payload: []byte(`{"value":"two"}`), Deadline: deadline, Timeout: time.Second}}})
		}, "H4sIAAAAAAAE/+JkgAAeMMnAwMTAwMDIwFaSWJyta8iWmZKaq2vIDZFiYElNzsjnZ2BgqFYqS8wpTVWyUsrPS1WqZfj3pG6r930PhlOzrMEKYQYYQQwwghmOxYCS8nwsBgAGAOZCCFyWAAAA"},
		{"success result", func() ([]byte, error) {
			return EncodeResult(Result{TaskID: "task-2", SubmissionID: "submit-1", Sequence: 10, Success: true, Payload: []byte(`{"value":"two"}`)})
		}, "H4sIAAAAAAAE/wAtANL/BnRhc2stMghzdWJtaXQtMQoAAAAAAAAAAQ8AAAB7InZhbHVlIjoidHdvIn0AAwDu2LAWLQAAAA=="},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.encode()
			if err != nil {
				t.Fatal(err)
			}
			want, err := base64.StdEncoding.DecodeString(tc.golden)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("encoded frame differs from research/validation/gateway-protocol/golden-vectors.json\n got: %s\nwant: %s", base64.StdEncoding.EncodeToString(got), tc.golden)
			}
		})
	}
}

func TestRegistrationCompatibilityRoundTrip(t *testing.T) {
	want := Registration{Build: "fixture", HostID: "host-01", Architecture: "X64", AgentID: "0123456789abcdef0123456789abcdef", FQDN: "lab.example", OS: "Linux", Username: "user", Path: "/opt/beacon", PID: 42, Interval: 500 * time.Millisecond, Window: 1500 * time.Millisecond, Expiration: time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)}
	frame, err := EncodeRegistration(want)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.HasPrefix(frame, []byte{0x1f, 0x8b}) {
		t.Fatalf("missing gzip magic: %x", frame[:2])
	}
	got, err := DecodeRegistration(frame)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("round trip mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestTaskBatchAndResultRoundTrip(t *testing.T) {
	deadline := time.Now().UTC().Add(time.Minute)
	batch := TaskBatch{Sequence: 9, Cursor: 12, Tasks: []Task{{Version: 1, ID: "task-1", IdempotencyKey: "idem-1", Cursor: 11, Kind: "echo", Payload: []byte(`{"value":"one"}`), Deadline: deadline, Timeout: time.Second}, {Version: 1, ID: "task-2", IdempotencyKey: "idem-2", Cursor: 12, Kind: "echo", Payload: []byte(`{"value":"two"}`), Deadline: deadline, Timeout: time.Second}}}
	frame, err := EncodeTaskBatch(batch)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeTaskBatch(frame)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Tasks) != 2 || got.Tasks[1].ID != "task-2" || got.Cursor != 12 {
		t.Fatalf("unexpected batch: %#v", got)
	}
	result := Result{TaskID: "task-2", SubmissionID: "submit-1", Sequence: 10, Success: false, Error: &TaskError{Code: "invalid_payload", Message: "value is required", Retryable: false}}
	frame, err = EncodeResult(result)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeResult(frame)
	if err != nil || decoded.TaskID != result.TaskID || decoded.Error == nil || decoded.Error.Code != result.Error.Code {
		t.Fatalf("result=%#v err=%v", decoded, err)
	}
}

func TestCommandEnvelopeAndOutputChunkRoundTrip(t *testing.T) {
	deadline := time.Now().UTC().Add(time.Minute)
	batch := TaskBatch{Sequence: 11, Cursor: 13, Tasks: []Task{{
		Version: CommandTaskEnvelopeVersion, ID: "task-command", IdempotencyKey: "idem-command", Cursor: 13,
		Kind: "shell", Payload: []byte(`{"interpreter":"sh","command":"printf hello"}`),
		Deadline: deadline, Timeout: time.Second,
	}}}
	frame, err := EncodeTaskBatch(batch)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeTaskBatch(frame)
	if err != nil || len(decoded.Tasks) != 1 || decoded.Tasks[0].Version != CommandTaskEnvelopeVersion {
		t.Fatalf("decoded=%#v err=%v", decoded, err)
	}
	chunk := TaskOutputChunk{TaskID: "task-command", SubmissionID: "task-command:idem-command", Sequence: 12, Stream: "stdout", Index: 3, Data: []byte("hello"), EOF: true}
	frame, err = EncodeTaskOutputChunk(chunk)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeTaskOutputChunk(frame)
	if err != nil || got.TaskID != chunk.TaskID || got.Stream != "stdout" || got.Index != 3 || !bytes.Equal(got.Data, chunk.Data) || !got.EOF {
		t.Fatalf("chunk=%#v err=%v", got, err)
	}
}

func TestMalformedFramesFailClosed(t *testing.T) {
	valid, _ := EncodeAck(Ack{1, 2})
	for _, frame := range [][]byte{nil, []byte("not-gzip"), valid[:len(valid)-2], append(append([]byte{}, valid...), 0)} {
		if _, err := DecodeAck(frame); !errors.Is(err, ErrMalformed) {
			t.Errorf("frame %x: got %v", frame, err)
		}
	}
}

func TestLimits(t *testing.T) {
	if _, err := EncodeTaskBatch(TaskBatch{Tasks: make([]Task, MaxTasksPerCheckin+1)}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v", err)
	}
	if _, err := EncodeResult(Result{TaskID: "x", SubmissionID: "y", Success: true, Payload: make([]byte, MaxResultSize+1)}); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v", err)
	}
	if _, err := EncodeTaskBatch(TaskBatch{Tasks: []Task{{Version: 1, ID: "task", IdempotencyKey: "idem", Kind: "shell", Deadline: time.Now().Add(time.Minute), Timeout: time.Second}}}); !errors.Is(err, ErrMalformed) {
		t.Fatalf("v1 command task accepted: %v", err)
	}
	if _, err := EncodeTaskOutputChunk(TaskOutputChunk{TaskID: "task", SubmissionID: "submission", Sequence: 1, Stream: "stdout", Data: make([]byte, MaxOutputChunkSize+1)}); !errors.Is(err, ErrMalformed) {
		t.Fatalf("oversized output chunk: %v", err)
	}
}
