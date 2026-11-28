// Package gateway implements the bounded HTTP body codec used by the agent
// gateway. The primitive encoding is compatible with the observed managed
// beacon: gzip around little-endian .NET BinaryWriter values. Message type and
// protocol version are carried by the HTTP route/header; the legacy body has no
// inner magic, version, or length prefix.
package gateway

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"
	"unicode/utf8"
)

const (
	Version                    = 1
	TaskEnvelopeVersion        = 1
	CommandTaskEnvelopeVersion = 2
	MaxFrameSize               = 1 << 20
	MaxTaskSize                = 64 << 10
	MaxResultSize              = 256 << 10
	MaxOutputChunkSize         = 32 << 10
	MaxTaskOutputSize          = 8 << 20
	MaxTasksPerCheckin         = 32
	MaxStringSize              = 16 << 10
	MinTaskTimeout             = 100 * time.Millisecond
	MaxTaskTimeout             = 5 * time.Minute
	dotNetUnixEpochTicks       = int64(621355968000000000)
	dotNetTicksMask            = uint64(0x3fffffffffffffff)
	dotNetUTCKind              = uint64(0x4000000000000000)
)

var (
	ErrMalformed = errors.New("malformed gateway frame")
	ErrTooLarge  = errors.New("gateway frame exceeds limit")
)

type Registration struct {
	Build, HostID, Architecture, AgentID, FQDN, OS, Username, Path string
	PID                                                            int32
	Interval, Window                                               time.Duration
	Expiration                                                     time.Time
}

type Checkin struct {
	AgentID   string
	Sequence  uint64
	Cursor    uint64
	Limit     uint32
	Timestamp time.Time
}

type Task struct {
	Version         uint16
	ID              string
	IdempotencyKey  string
	Cursor          uint64
	Kind            string
	Payload         []byte
	Deadline        time.Time
	Timeout         time.Duration
	CancelRequested bool
}

type TaskBatch struct {
	Sequence, Cursor uint64
	Tasks            []Task
}

type TaskError struct {
	Code, Message string
	Retryable     bool
}

type Result struct {
	TaskID, SubmissionID string
	Sequence             uint64
	Success              bool
	Payload              []byte
	Error                *TaskError
}

// TaskOutputChunk is a bounded, independently authenticated piece of process
// output. Indexes are monotonic per task and stream; the server can therefore
// reject gaps and acknowledge retries idempotently without interpreting data.
type TaskOutputChunk struct {
	TaskID, SubmissionID string
	Sequence             uint64
	Stream               string
	Index                uint32
	Data                 []byte
	EOF, Truncated       bool
}

type FileTransferManifest struct {
	TaskID, SubmissionID string
	Sequence             uint64
	Name, MediaType      string
	Size                 uint64
	SHA256               string
}

type FileTransferComplete struct {
	TaskID, TransferID string
	Sequence           uint64
	SHA256             string
}

type Ack struct{ Sequence, Cursor uint64 }
type Receipt struct {
	TaskID           string
	Sequence, Cursor uint64
}

func EncodeRegistration(v Registration) ([]byte, error) {
	return encode(func(w *writer) {
		w.str(v.Build)
		w.str(v.HostID)
		w.str(v.Architecture)
		w.str(v.AgentID)
		w.str(v.FQDN)
		w.str(v.OS)
		w.str(v.Username)
		w.str(v.Path)
		w.i32(v.PID)
		w.span(v.Interval)
		w.span(v.Window)
		w.date(v.Expiration)
	})
}

func DecodeRegistration(frame []byte) (Registration, error) {
	r, err := newReader(frame)
	if err != nil {
		return Registration{}, err
	}
	v := Registration{Build: r.str(), HostID: r.str(), Architecture: r.str(), AgentID: r.str(), FQDN: r.str(), OS: r.str(), Username: r.str(), Path: r.str(), PID: r.i32(), Interval: r.span(), Window: r.span(), Expiration: r.date()}
	return v, r.finish()
}

func EncodeCheckin(v Checkin) ([]byte, error) {
	if v.Limit == 0 || v.Limit > MaxTasksPerCheckin {
		return nil, fmt.Errorf("%w: invalid task limit", ErrMalformed)
	}
	return encode(func(w *writer) {
		w.str(v.AgentID)
		w.u64(v.Sequence)
		w.u64(v.Cursor)
		w.u32(v.Limit)
		w.date(v.Timestamp)
	})
}

func DecodeCheckin(frame []byte) (Checkin, error) {
	r, err := newReader(frame)
	if err != nil {
		return Checkin{}, err
	}
	v := Checkin{AgentID: r.str(), Sequence: r.u64(), Cursor: r.u64(), Limit: r.u32(), Timestamp: r.date()}
	if v.Limit == 0 || v.Limit > MaxTasksPerCheckin {
		r.fail("invalid task limit")
	}
	return v, r.finish()
}

func EncodeTaskBatch(v TaskBatch) ([]byte, error) {
	if len(v.Tasks) > MaxTasksPerCheckin {
		return nil, ErrTooLarge
	}
	return encode(func(w *writer) {
		w.u64(v.Sequence)
		w.u64(v.Cursor)
		w.u32(uint32(len(v.Tasks)))
		for _, task := range v.Tasks {
			if !validTask(task) {
				w.err = fmt.Errorf("%w: invalid task envelope", ErrMalformed)
				return
			}
			w.u16(task.Version)
			w.str(task.ID)
			w.str(task.IdempotencyKey)
			w.u64(task.Cursor)
			w.str(task.Kind)
			w.blob(task.Payload, MaxTaskSize)
			w.date(task.Deadline)
			w.u64(uint64(task.Timeout))
			w.boolean(task.CancelRequested)
		}
	})
}

func DecodeTaskBatch(frame []byte) (TaskBatch, error) {
	r, err := newReader(frame)
	if err != nil {
		return TaskBatch{}, err
	}
	v := TaskBatch{Sequence: r.u64(), Cursor: r.u64()}
	count := r.u32()
	if count > MaxTasksPerCheckin {
		r.fail("too many tasks")
		count = 0
	}
	v.Tasks = make([]Task, 0, count)
	for range count {
		task := Task{Version: r.u16(), ID: r.str(), IdempotencyKey: r.str(), Cursor: r.u64(), Kind: r.str(), Payload: r.blob(MaxTaskSize), Deadline: r.date(), Timeout: time.Duration(r.u64()), CancelRequested: r.boolean()}
		if !validTask(task) {
			r.fail("invalid task envelope")
		}
		v.Tasks = append(v.Tasks, task)
	}
	return v, r.finish()
}

func EncodeResult(v Result) ([]byte, error) {
	if v.Success && v.Error != nil || !v.Success && (v.Error == nil || v.Error.Code == "") {
		return nil, fmt.Errorf("%w: invalid structured error", ErrMalformed)
	}
	return encode(func(w *writer) {
		w.str(v.TaskID)
		w.str(v.SubmissionID)
		w.u64(v.Sequence)
		w.boolean(v.Success)
		w.blob(v.Payload, MaxResultSize)
		w.boolean(v.Error != nil)
		if v.Error != nil {
			w.str(v.Error.Code)
			w.str(v.Error.Message)
			w.boolean(v.Error.Retryable)
		}
	})
}

func DecodeResult(frame []byte) (Result, error) {
	r, err := newReader(frame)
	if err != nil {
		return Result{}, err
	}
	v := Result{TaskID: r.str(), SubmissionID: r.str(), Sequence: r.u64(), Success: r.boolean(), Payload: r.blob(MaxResultSize)}
	if r.boolean() {
		v.Error = &TaskError{Code: r.str(), Message: r.str(), Retryable: r.boolean()}
	}
	if v.Success && v.Error != nil {
		r.fail("successful result contains error")
	}
	if !v.Success && (v.Error == nil || v.Error.Code == "") {
		r.fail("failed result has no error code")
	}
	return v, r.finish()
}

func EncodeTaskOutputChunk(v TaskOutputChunk) ([]byte, error) {
	if !validTaskOutputChunk(v) {
		return nil, fmt.Errorf("%w: invalid task output chunk", ErrMalformed)
	}
	return encode(func(w *writer) {
		w.str(v.TaskID)
		w.str(v.SubmissionID)
		w.u64(v.Sequence)
		w.str(v.Stream)
		w.u32(v.Index)
		w.blob(v.Data, MaxOutputChunkSize)
		w.boolean(v.EOF)
		w.boolean(v.Truncated)
	})
}

func DecodeTaskOutputChunk(frame []byte) (TaskOutputChunk, error) {
	r, err := newReader(frame)
	if err != nil {
		return TaskOutputChunk{}, err
	}
	v := TaskOutputChunk{
		TaskID: r.str(), SubmissionID: r.str(), Sequence: r.u64(),
		Stream: r.str(), Index: r.u32(), Data: r.blob(MaxOutputChunkSize),
		EOF: r.boolean(), Truncated: r.boolean(),
	}
	if !validTaskOutputChunk(v) {
		r.fail("invalid task output chunk")
	}
	return v, r.finish()
}

func EncodeFileTransferManifest(v FileTransferManifest) ([]byte, error) {
	if !validFileManifest(v) {
		return nil, fmt.Errorf("%w: invalid file transfer manifest", ErrMalformed)
	}
	return encode(func(w *writer) {
		w.str(v.TaskID)
		w.str(v.SubmissionID)
		w.u64(v.Sequence)
		w.str(v.Name)
		w.str(v.MediaType)
		w.u64(v.Size)
		w.str(v.SHA256)
	})
}

func DecodeFileTransferManifest(frame []byte) (FileTransferManifest, error) {
	r, err := newReader(frame)
	if err != nil {
		return FileTransferManifest{}, err
	}
	v := FileTransferManifest{TaskID: r.str(), SubmissionID: r.str(), Sequence: r.u64(), Name: r.str(), MediaType: r.str(), Size: r.u64(), SHA256: r.str()}
	if !validFileManifest(v) {
		r.fail("invalid file transfer manifest")
	}
	return v, r.finish()
}

func EncodeFileTransferComplete(v FileTransferComplete) ([]byte, error) {
	if v.TaskID == "" || v.TransferID == "" || v.Sequence == 0 || !validDigest(v.SHA256) {
		return nil, fmt.Errorf("%w: invalid file transfer completion", ErrMalformed)
	}
	return encode(func(w *writer) { w.str(v.TaskID); w.str(v.TransferID); w.u64(v.Sequence); w.str(v.SHA256) })
}

func DecodeFileTransferComplete(frame []byte) (FileTransferComplete, error) {
	r, err := newReader(frame)
	if err != nil {
		return FileTransferComplete{}, err
	}
	v := FileTransferComplete{TaskID: r.str(), TransferID: r.str(), Sequence: r.u64(), SHA256: r.str()}
	if v.TaskID == "" || v.TransferID == "" || v.Sequence == 0 || !validDigest(v.SHA256) {
		r.fail("invalid file transfer completion")
	}
	return v, r.finish()
}

func EncodeAck(v Ack) ([]byte, error) {
	return encode(func(w *writer) { w.u64(v.Sequence); w.u64(v.Cursor) })
}
func DecodeAck(frame []byte) (Ack, error) {
	r, err := newReader(frame)
	if err != nil {
		return Ack{}, err
	}
	v := Ack{r.u64(), r.u64()}
	return v, r.finish()
}
func EncodeReceipt(v Receipt) ([]byte, error) {
	return encode(func(w *writer) { w.str(v.TaskID); w.u64(v.Sequence); w.u64(v.Cursor) })
}
func DecodeReceipt(frame []byte) (Receipt, error) {
	r, err := newReader(frame)
	if err != nil {
		return Receipt{}, err
	}
	v := Receipt{TaskID: r.str(), Sequence: r.u64(), Cursor: r.u64()}
	return v, r.finish()
}

type writer struct {
	bytes.Buffer
	err error
}

func (w *writer) raw(v any) {
	if w.err == nil {
		w.err = binary.Write(&w.Buffer, binary.LittleEndian, v)
	}
}
func (w *writer) i32(v int32)  { w.raw(v) }
func (w *writer) u16(v uint16) { w.raw(v) }
func (w *writer) u32(v uint32) { w.raw(v) }
func (w *writer) u64(v uint64) { w.raw(v) }
func (w *writer) boolean(v bool) {
	if v {
		w.WriteByte(1)
	} else {
		w.WriteByte(0)
	}
}
func (w *writer) str(v string) {
	if w.err != nil {
		return
	}
	if !utf8.ValidString(v) || len(v) > MaxStringSize {
		w.err = ErrTooLarge
		return
	}
	w.seven(uint32(len(v)))
	_, w.err = w.WriteString(v)
}
func (w *writer) seven(v uint32) {
	for v >= 0x80 {
		w.WriteByte(byte(v) | 0x80)
		v >>= 7
	}
	w.WriteByte(byte(v))
}
func (w *writer) blob(v []byte, max int) {
	if len(v) > max {
		w.err = ErrTooLarge
		return
	}
	w.u32(uint32(len(v)))
	if w.err == nil {
		_, w.err = w.Write(v)
	}
}
func (w *writer) span(v time.Duration) {
	ms := v.Milliseconds()
	days := ms / 86400000
	ms %= 86400000
	hours := ms / 3600000
	ms %= 3600000
	mins := ms / 60000
	ms %= 60000
	secs := ms / 1000
	ms %= 1000
	w.i32(int32(days))
	w.i32(int32(hours))
	w.i32(int32(mins))
	w.i32(int32(secs))
	w.i32(int32(ms))
}
func (w *writer) date(v time.Time) {
	ticks := uint64(dotNetUnixEpochTicks + v.UTC().UnixNano()/100)
	w.u64(ticks | dotNetUTCKind)
}

func ValidTaskKind(kind string) bool {
	switch kind {
	case "echo", "system-info", "sleep", "exec", "shell", "file-download":
		return true
	default:
		return false
	}
}

func ValidTaskKindForVersion(version uint16, kind string) bool {
	switch version {
	case TaskEnvelopeVersion:
		return kind == "echo" || kind == "system-info" || kind == "sleep"
	case CommandTaskEnvelopeVersion:
		return kind == "exec" || kind == "shell" || kind == "file-download"
	default:
		return false
	}
}

func validTask(v Task) bool {
	return ValidTaskKindForVersion(v.Version, v.Kind) && v.ID != "" && v.IdempotencyKey != "" &&
		len(v.Payload) <= MaxTaskSize && !v.Deadline.IsZero() &&
		v.Timeout >= MinTaskTimeout && v.Timeout <= MaxTaskTimeout
}

func validTaskOutputChunk(v TaskOutputChunk) bool {
	return v.TaskID != "" && v.SubmissionID != "" && v.Sequence > 0 &&
		(v.Stream == "stdout" || v.Stream == "stderr") && len(v.Data) <= MaxOutputChunkSize &&
		(len(v.Data) > 0 || v.EOF)
}

func validFileManifest(v FileTransferManifest) bool {
	return v.TaskID != "" && v.SubmissionID != "" && v.Sequence > 0 && v.Name != "" &&
		v.Size <= 1<<30 && len(v.MediaType) <= 128 && validDigest(v.SHA256)
}

func validDigest(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !(character >= '0' && character <= '9' || character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func encode(fn func(*writer)) ([]byte, error) {
	w := &writer{}
	fn(w)
	if w.err != nil {
		return nil, w.err
	}
	if w.Len() > MaxFrameSize {
		return nil, ErrTooLarge
	}
	var out bytes.Buffer
	gz, _ := gzip.NewWriterLevel(&out, gzip.BestSpeed)
	gz.Header.ModTime = time.Unix(0, 0)
	gz.Header.OS = 255
	if _, err := gz.Write(w.Bytes()); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	if out.Len() > MaxFrameSize {
		return nil, ErrTooLarge
	}
	return out.Bytes(), nil
}

type reader struct {
	*bytes.Reader
	err error
}

func newReader(frame []byte) (*reader, error) {
	if len(frame) < 18 || len(frame) > MaxFrameSize {
		return nil, ErrMalformed
	}
	gz, err := gzip.NewReader(bytes.NewReader(frame))
	if err != nil {
		return nil, fmt.Errorf("%w: gzip", ErrMalformed)
	}
	plain, err := io.ReadAll(io.LimitReader(gz, MaxFrameSize+1))
	closeErr := gz.Close()
	if err != nil || closeErr != nil || len(plain) > MaxFrameSize {
		return nil, fmt.Errorf("%w: gzip body", ErrMalformed)
	}
	return &reader{Reader: bytes.NewReader(plain)}, nil
}
func (r *reader) fail(s string) {
	if r.err == nil {
		r.err = fmt.Errorf("%w: %s", ErrMalformed, s)
	}
}
func (r *reader) raw(v any) {
	if r.err == nil {
		if err := binary.Read(r.Reader, binary.LittleEndian, v); err != nil {
			r.fail("truncated")
		}
	}
}
func (r *reader) i32() (v int32)  { r.raw(&v); return }
func (r *reader) u16() (v uint16) { r.raw(&v); return }
func (r *reader) u32() (v uint32) { r.raw(&v); return }
func (r *reader) u64() (v uint64) { r.raw(&v); return }
func (r *reader) boolean() bool {
	b, err := r.ReadByte()
	if err != nil || b > 1 {
		r.fail("invalid boolean")
	}
	return b == 1
}
func (r *reader) seven() uint32 {
	var v uint32
	for i := 0; i < 5; i++ {
		b, err := r.ReadByte()
		if err != nil {
			r.fail("truncated string length")
			return 0
		}
		if i == 4 && b > 0x0f {
			r.fail("invalid string length")
			return 0
		}
		v |= uint32(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return v
		}
	}
	r.fail("invalid string length")
	return 0
}
func (r *reader) str() string {
	n := r.seven()
	if n > MaxStringSize || uint64(n) > uint64(r.Len()) {
		r.fail("invalid string")
		return ""
	}
	b := make([]byte, n)
	_, _ = io.ReadFull(r, b)
	if !utf8.Valid(b) {
		r.fail("invalid utf-8")
		return ""
	}
	return string(b)
}
func (r *reader) blob(max int) []byte {
	n := r.u32()
	if n > uint32(max) || uint64(n) > uint64(r.Len()) {
		r.fail("invalid blob")
		return nil
	}
	b := make([]byte, n)
	_, _ = io.ReadFull(r, b)
	return b
}
func (r *reader) span() time.Duration {
	d, h, m, s, ms := r.i32(), r.i32(), r.i32(), r.i32(), r.i32()
	if h < 0 || h > 23 || m < 0 || m > 59 || s < 0 || s > 59 || ms < 0 || ms > 999 {
		r.fail("invalid timespan")
	}
	return time.Duration(d)*24*time.Hour + time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(s)*time.Second + time.Duration(ms)*time.Millisecond
}
func (r *reader) date() time.Time {
	raw := r.u64()
	ticks := int64(raw & dotNetTicksMask)
	if ticks < dotNetUnixEpochTicks {
		r.fail("invalid datetime")
		return time.Time{}
	}
	return time.Unix(0, (ticks-dotNetUnixEpochTicks)*100).UTC()
}
func (r *reader) finish() error {
	if r.err != nil {
		return r.err
	}
	if r.Len() != 0 {
		return fmt.Errorf("%w: trailing bytes", ErrMalformed)
	}
	return nil
}
