package beacon

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/research/specter-reimplementation/internal/agentworker"
	"github.com/research/specter-reimplementation/internal/gateway"
)

const (
	maxPendingResults = 64
	maxPendingAge     = 24 * time.Hour
)

type Config struct {
	Endpoint, EnrollmentToken, AgentID, Name, StatePath string
	BuildVersion                                        string
	CABundlePath                                        string
	Interval, Jitter                                    time.Duration
	MinBackoff, MaxBackoff                              time.Duration
	ConnectTimeout, RequestTimeout                      time.Duration
	IdentityRotationInterval                            time.Duration
	BatchSize                                           uint32
	Parallelism                                         int
	AllowInsecureLoopback                               bool
	HTTPClient                                          *http.Client
}

type PendingResult struct {
	Result    gateway.Result `json:"result"`
	CreatedAt time.Time      `json:"createdAt"`
}

type State struct {
	Version           int             `json:"version"`
	AgentID           string          `json:"agentId"`
	Identity          string          `json:"identity"`
	Cursor            uint64          `json:"cursor"`
	Sequence          uint64          `json:"sequence"`
	Pending           []PendingResult `json:"pending"`
	IdentityRotatedAt time.Time       `json:"identityRotatedAt"`
}

type Client struct {
	cfg     Config
	state   State
	stateMu sync.Mutex
	worker  *agentworker.Worker
}

type requestError struct {
	status    int
	permanent bool
	err       error
}

func (e *requestError) Error() string {
	if e.status != 0 {
		return fmt.Sprintf("gateway returned HTTP %d", e.status)
	}
	return e.err.Error()
}
func (e *requestError) Unwrap() error { return e.err }

func New(cfg Config) (*Client, error) {
	parsed, err := url.Parse(cfg.Endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("endpoint must be an absolute HTTP(S) URL without userinfo")
	}
	if parsed.Scheme != "https" && !(cfg.AllowInsecureLoopback && isLoopback(parsed.Hostname())) {
		return nil, errors.New("beacon endpoint must use TLS; plain HTTP is restricted to explicit loopback labs")
	}
	if cfg.Jitter == 0 {
		cfg.Jitter = cfg.Interval / 10
	}
	if cfg.MinBackoff == 0 {
		cfg.MinBackoff = 500 * time.Millisecond
	}
	if cfg.MaxBackoff == 0 {
		cfg.MaxBackoff = time.Minute
	}
	if cfg.ConnectTimeout == 0 {
		cfg.ConnectTimeout = 5 * time.Second
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 15 * time.Second
	}
	if cfg.Parallelism == 0 {
		cfg.Parallelism = 1
	}
	if cfg.IdentityRotationInterval == 0 {
		cfg.IdentityRotationInterval = 24 * time.Hour
	}
	if cfg.BuildVersion == "" {
		cfg.BuildVersion = "dev"
	}
	if len(cfg.Endpoint) > 2048 || len(cfg.EnrollmentToken) > 512 || len(cfg.AgentID) > 128 || len(cfg.Name) > 128 || len(cfg.BuildVersion) > 128 || cfg.Interval < 100*time.Millisecond || cfg.Interval > 24*time.Hour || cfg.Jitter < 0 || cfg.Jitter > cfg.Interval/2 || cfg.MinBackoff < 100*time.Millisecond || cfg.MaxBackoff < cfg.MinBackoff || cfg.MaxBackoff > 10*time.Minute || cfg.ConnectTimeout < 100*time.Millisecond || cfg.RequestTimeout < cfg.ConnectTimeout || cfg.RequestTimeout > 10*time.Minute || cfg.IdentityRotationInterval < time.Hour || cfg.IdentityRotationInterval > 30*24*time.Hour || cfg.BatchSize == 0 || cfg.BatchSize > gateway.MaxTasksPerCheckin {
		return nil, errors.New("beacon configuration exceeds policy")
	}
	if cfg.StatePath == "" {
		return nil, errors.New("state path is required")
	}
	if cfg.HTTPClient == nil {
		roots, err := loadRoots(cfg.CABundlePath)
		if err != nil {
			return nil, err
		}
		transport := &http.Transport{
			Proxy:                 http.ProxyFromEnvironment,
			DialContext:           (&net.Dialer{Timeout: cfg.ConnectTimeout, KeepAlive: 30 * time.Second}).DialContext,
			TLSClientConfig:       &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots},
			TLSHandshakeTimeout:   cfg.ConnectTimeout,
			ResponseHeaderTimeout: cfg.RequestTimeout,
			IdleConnTimeout:       90 * time.Second,
			MaxIdleConnsPerHost:   2,
		}
		cfg.HTTPClient = &http.Client{Transport: transport, Timeout: cfg.RequestTimeout}
	}
	worker, err := agentworker.New(cfg.Parallelism)
	if err != nil {
		return nil, err
	}
	c := &Client{cfg: cfg, worker: worker, state: State{Version: 1}}
	if content, err := os.ReadFile(cfg.StatePath); err == nil {
		if len(content) > gateway.MaxFrameSize || json.Unmarshal(content, &c.state) != nil || c.state.Version != 1 || len(c.state.Pending) > maxPendingResults {
			return nil, errors.New("invalid beacon state")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if c.state.AgentID == "" {
		c.state.AgentID = cfg.AgentID
	}
	if c.state.Identity != "" && c.state.IdentityRotatedAt.IsZero() {
		c.state.IdentityRotatedAt = time.Now().UTC()
	}
	return c, nil
}

func (c *Client) Run(ctx context.Context) error {
	defer c.cfg.HTTPClient.CloseIdleConnections()
	backoff, delay := c.cfg.MinBackoff, time.Duration(0)
	for {
		if err := wait(ctx, delay); err != nil {
			return err
		}
		wasUnenrolled := c.state.Identity == ""
		var err error
		if wasUnenrolled {
			err = c.enroll(ctx)
		} else {
			err = c.checkin(ctx)
		}
		if err == nil {
			backoff = c.cfg.MinBackoff
			if wasUnenrolled {
				delay = 0
			} else {
				delay = c.jitter(c.cfg.Interval)
			}
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !temporary(err) {
			return err
		}
		delay = c.jitter(backoff)
		if backoff < c.cfg.MaxBackoff/2 {
			backoff *= 2
		} else {
			backoff = c.cfg.MaxBackoff
		}
	}
}

func (c *Client) enroll(ctx context.Context) error {
	host, _ := os.Hostname()
	executable, _ := os.Executable()
	registration := gateway.Registration{Build: c.cfg.BuildVersion, HostID: host, Architecture: runtime.GOARCH, AgentID: c.state.AgentID, FQDN: host, OS: runtime.GOOS, Username: "redacted", Path: filepath.Base(executable), PID: int32(os.Getpid()), Interval: c.cfg.Interval, Expiration: time.Now().UTC().Add(24 * time.Hour)}
	body, err := gateway.EncodeRegistration(registration)
	if err != nil {
		return err
	}
	response, err := c.requestWithHeaders(ctx, http.MethodPost, "/api/agent/v1/enroll", "Enrollment "+c.cfg.EnrollmentToken, 0, body, "application/vnd.specter.gateway+gzip", map[string]string{
		"X-Specter-Capabilities": strings.Join(agentworker.Capabilities(), ","),
	})
	if err != nil {
		return err
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, gateway.MaxFrameSize))
	c.state.AgentID = response.Header.Get("X-Specter-Agent-ID")
	c.state.Identity = response.Header.Get("X-Specter-Identity")
	c.state.IdentityRotatedAt = time.Now().UTC()
	if c.state.AgentID == "" || c.state.Identity == "" {
		return errors.New("enrollment response omitted identity")
	}
	if err := c.save(); err != nil {
		return err
	}
	c.cfg.EnrollmentToken = ""
	return nil
}

func (c *Client) checkin(ctx context.Context) error {
	if c.state.IdentityRotatedAt.Add(c.cfg.IdentityRotationInterval).Before(time.Now().UTC()) {
		if err := c.rotateIdentity(ctx); err != nil {
			return err
		}
	}
	if err := c.flush(ctx); err != nil {
		return err
	}
	if len(c.state.Pending) >= maxPendingResults {
		return &requestError{err: errors.New("pending result queue is full")}
	}
	sequence, err := c.nextSequence()
	if err != nil {
		return err
	}
	body, err := gateway.EncodeCheckin(gateway.Checkin{AgentID: c.state.AgentID, Sequence: sequence, Cursor: c.state.Cursor, Limit: c.cfg.BatchSize, Timestamp: time.Now().UTC()})
	if err != nil {
		return err
	}
	response, err := c.request(ctx, "/api/agent/v1/checkin", "Bearer "+c.state.Identity, sequence, body)
	if err != nil {
		return err
	}
	frame, err := readBounded(response.Body)
	if err != nil {
		return err
	}
	batch, err := gateway.DecodeTaskBatch(frame)
	if err != nil {
		return err
	}
	for _, task := range batch.Tasks {
		if task.Cursor <= c.state.Cursor {
			continue
		}
		if len(c.state.Pending) >= maxPendingResults {
			return &requestError{err: errors.New("pending result queue is full")}
		}
		sequence, err := c.nextSequence()
		if err != nil {
			return err
		}
		receipt, _ := gateway.EncodeReceipt(gateway.Receipt{TaskID: task.ID, Sequence: sequence, Cursor: task.Cursor})
		response, err := c.request(ctx, "/api/agent/v1/ack", "Bearer "+c.state.Identity, sequence, receipt)
		if err != nil {
			return err
		}
		response.Body.Close()
		result, err := c.executeTask(ctx, task)
		if err != nil {
			return err
		}
		sequence, err = c.nextSequence()
		if err != nil {
			return err
		}
		result.Sequence = sequence
		c.state.Cursor = task.Cursor
		c.state.Pending = append(c.state.Pending, PendingResult{Result: result, CreatedAt: time.Now().UTC()})
		if err := c.save(); err != nil {
			return err
		}
		if err := c.flush(ctx); err != nil {
			return err
		}
	}
	return c.save()
}

func (c *Client) executeTask(ctx context.Context, task gateway.Task) (gateway.Result, error) {
	taskContext, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan gateway.Result, 1)
	go func() {
		done <- c.worker.ExecuteStreaming(taskContext, task, 0, func(outputContext context.Context, chunk gateway.TaskOutputChunk) error {
			return c.sendTaskOutput(outputContext, chunk)
		}, c.uploadFile)
	}()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case result := <-done:
			return result, nil
		case <-ctx.Done():
			cancel()
			return <-done, nil
		case <-ticker.C:
			sequence, err := c.nextSequence()
			if err != nil {
				cancel()
				<-done
				return gateway.Result{}, err
			}
			body, _ := gateway.EncodeReceipt(gateway.Receipt{TaskID: task.ID, Sequence: sequence, Cursor: task.Cursor})
			response, err := c.request(ctx, "/api/agent/v1/task-status", "Bearer "+c.state.Identity, sequence, body)
			if err != nil {
				if temporary(err) {
					continue
				}
				cancel()
				<-done
				return gateway.Result{}, err
			}
			cancelRequested := response.Header.Get("X-Specter-Cancel-Requested") == "true"
			if _, err = readBounded(response.Body); err != nil {
				cancel()
				<-done
				return gateway.Result{}, err
			}
			if cancelRequested {
				cancel()
				return <-done, nil
			}
		}
	}
}

func (c *Client) rotateIdentity(ctx context.Context) error {
	sequence, err := c.nextSequence()
	if err != nil {
		return err
	}
	body, _ := gateway.EncodeAck(gateway.Ack{Sequence: sequence, Cursor: c.state.Cursor})
	response, err := c.request(ctx, "/api/agent/v1/rotate", "Bearer "+c.state.Identity, sequence, body)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	identity := response.Header.Get("X-Specter-Identity")
	if identity == "" {
		return errors.New("rotation response omitted identity")
	}
	c.state.Identity = identity
	c.state.IdentityRotatedAt = time.Now().UTC()
	return c.save()
}

func (c *Client) nextSequence() (uint64, error) {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	c.state.Sequence++
	return c.state.Sequence, c.saveLocked()
}

func (c *Client) sendTaskOutput(ctx context.Context, chunk gateway.TaskOutputChunk) error {
	sequence, err := c.nextSequence()
	if err != nil {
		return err
	}
	chunk.Sequence = sequence
	body, err := gateway.EncodeTaskOutputChunk(chunk)
	if err != nil {
		return err
	}
	response, err := c.request(ctx, "/api/agent/v1/task-output", "Bearer "+c.state.Identity, sequence, body)
	if err != nil {
		return err
	}
	frame, err := readBounded(response.Body)
	if err != nil {
		return err
	}
	_, err = gateway.DecodeAck(frame)
	return err
}

func (c *Client) uploadFile(ctx context.Context, task gateway.Task, metadata agentworker.FileDownloadResult) (any, error) {
	sequence, err := c.nextSequence()
	if err != nil {
		return nil, err
	}
	manifest := gateway.FileTransferManifest{
		TaskID: task.ID, SubmissionID: task.ID + ":" + task.IdempotencyKey, Sequence: sequence,
		Name: metadata.Name, MediaType: metadata.MediaType, Size: uint64(metadata.Size), SHA256: metadata.SHA256,
	}
	body, err := gateway.EncodeFileTransferManifest(manifest)
	if err != nil {
		return nil, err
	}
	response, err := c.request(ctx, "/api/agent/v1/files/begin", "Bearer "+c.state.Identity, sequence, body)
	if err != nil {
		return nil, err
	}
	transferID := response.Header.Get("X-Specter-Transfer-ID")
	chunkSize, parseErr := strconv.ParseInt(response.Header.Get("X-Specter-Chunk-Size"), 10, 64)
	_, readErr := readBounded(response.Body)
	if transferID == "" || parseErr != nil || chunkSize < 1<<20 || chunkSize > 8<<20 || readErr != nil {
		return nil, errors.New("file transfer begin response is invalid")
	}
	file, err := os.Open(metadata.Path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	buffer := make([]byte, int(chunkSize))
	for index := 0; ; index++ {
		count, readErr := io.ReadFull(file, buffer)
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil && !errors.Is(readErr, io.ErrUnexpectedEOF) {
			return nil, readErr
		}
		sequence, err = c.nextSequence()
		if err != nil {
			return nil, err
		}
		path := "/api/agent/v1/files/" + url.PathEscape(transferID) + "/chunks/" + strconv.Itoa(index)
		response, err = c.requestWithHeaders(ctx, http.MethodPut, path, "Bearer "+c.state.Identity, sequence, buffer[:count], "application/octet-stream", map[string]string{"X-Specter-Chunk-Index": strconv.Itoa(index)})
		if err != nil {
			return nil, err
		}
		frame, frameErr := readBounded(response.Body)
		if frameErr != nil {
			return nil, frameErr
		}
		if _, frameErr = gateway.DecodeAck(frame); frameErr != nil {
			return nil, frameErr
		}
		if errors.Is(readErr, io.ErrUnexpectedEOF) {
			break
		}
	}
	sequence, err = c.nextSequence()
	if err != nil {
		return nil, err
	}
	complete, err := gateway.EncodeFileTransferComplete(gateway.FileTransferComplete{TaskID: task.ID, TransferID: transferID, Sequence: sequence, SHA256: metadata.SHA256})
	if err != nil {
		return nil, err
	}
	response, err = c.request(ctx, "/api/agent/v1/files/"+url.PathEscape(transferID)+"/complete", "Bearer "+c.state.Identity, sequence, complete)
	if err != nil {
		return nil, err
	}
	objectID := response.Header.Get("X-Specter-Object-ID")
	frame, err := readBounded(response.Body)
	if err != nil {
		return nil, err
	}
	if _, err = gateway.DecodeAck(frame); err != nil {
		return nil, err
	}
	return map[string]any{"path": metadata.Path, "name": metadata.Name, "size": metadata.Size, "sha256": metadata.SHA256, "mediaType": metadata.MediaType, "objectId": objectID}, nil
}

func (c *Client) flush(ctx context.Context) error {
	for len(c.state.Pending) > 0 {
		pending := &c.state.Pending[0]
		if pending.CreatedAt.Add(maxPendingAge).Before(time.Now().UTC()) {
			pending.Result.Success = false
			pending.Result.Payload = nil
			pending.Result.Error = &gateway.TaskError{Code: "stale_result", Message: "local result exceeded retention policy", Retryable: true}
		}
		body, err := gateway.EncodeResult(pending.Result)
		if err != nil {
			return err
		}
		response, err := c.request(ctx, "/api/agent/v1/results", "Bearer "+c.state.Identity, pending.Result.Sequence, body)
		if err != nil {
			return err
		}
		frame, err := readBounded(response.Body)
		if err != nil {
			return err
		}
		if _, err := gateway.DecodeAck(frame); err != nil {
			return err
		}
		c.state.Pending = c.state.Pending[1:]
		if err := c.save(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Client) request(ctx context.Context, path, authorization string, sequence uint64, body []byte) (*http.Response, error) {
	return c.requestWithHeaders(ctx, http.MethodPost, path, authorization, sequence, body, "application/vnd.specter.gateway+gzip", nil)
}

func (c *Client) requestWithHeaders(ctx context.Context, method, path, authorization string, sequence uint64, body []byte, mediaType string, headers map[string]string) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.cfg.Endpoint, "/")+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", mediaType)
	request.Header.Set("X-Specter-Protocol-Version", fmt.Sprint(gateway.Version))
	for name, value := range headers {
		request.Header.Set(name, value)
	}
	if strings.HasPrefix(authorization, "Bearer ") {
		timestamp := time.Now().UTC().Format(time.RFC3339Nano)
		nonce, err := randomNonce()
		if err != nil {
			return nil, err
		}
		identity := strings.TrimPrefix(authorization, "Bearer ")
		request.Header.Set("X-Specter-Timestamp", timestamp)
		request.Header.Set("X-Specter-Nonce", nonce)
		request.Header.Set("X-Specter-Sequence", fmt.Sprint(sequence))
		request.Header.Set("X-Specter-Signature", gateway.RequestSignature(identity, request.Method, request.URL.Path, timestamp, nonce, sequence, body))
	}
	response, err := c.cfg.HTTPClient.Do(request)
	if err != nil {
		return nil, classifyTransport(err)
	}
	if response.StatusCode/100 != 2 {
		defer response.Body.Close()
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		retryable := response.StatusCode == http.StatusRequestTimeout || response.StatusCode == http.StatusTooEarly || response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= 500
		return nil, &requestError{status: response.StatusCode, permanent: !retryable}
	}
	if response.Header.Get("X-Specter-Protocol-Version") != fmt.Sprint(gateway.Version) {
		response.Body.Close()
		return nil, &requestError{permanent: true, err: errors.New("gateway response protocol downgrade rejected")}
	}
	return response, nil
}

func readBounded(body io.ReadCloser) ([]byte, error) {
	defer body.Close()
	content, err := io.ReadAll(io.LimitReader(body, gateway.MaxFrameSize+1))
	if err != nil {
		return nil, err
	}
	if len(content) > gateway.MaxFrameSize {
		return nil, gateway.ErrTooLarge
	}
	return content, nil
}

func (c *Client) save() error {
	c.stateMu.Lock()
	defer c.stateMu.Unlock()
	return c.saveLocked()
}

func (c *Client) saveLocked() error {
	content, err := json.MarshalIndent(c.state, "", "  ")
	if err != nil {
		return err
	}
	directory := filepath.Dir(c.cfg.StatePath)
	if err := os.MkdirAll(directory, 0700); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".beacon-state-*")
	if err != nil {
		return err
	}
	name := temporary.Name()
	defer os.Remove(name)
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(name, c.cfg.StatePath)
}

func loadRoots(path string) (*x509.CertPool, error) {
	if path == "" {
		return nil, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read CA bundle: %w", err)
	}
	if len(content) == 0 || len(content) > 4<<20 {
		return nil, errors.New("CA bundle exceeds policy")
	}
	defer clear(content)
	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if !roots.AppendCertsFromPEM(content) {
		return nil, errors.New("CA bundle contains no certificates")
	}
	return roots, nil
}

func classifyTransport(err error) error {
	var unknownAuthority x509.UnknownAuthorityError
	var hostname x509.HostnameError
	var certificateInvalid x509.CertificateInvalidError
	if errors.As(err, &unknownAuthority) || errors.As(err, &hostname) || errors.As(err, &certificateInvalid) {
		return &requestError{permanent: true, err: errors.New("TLS certificate validation failed")}
	}
	return &requestError{err: err}
}

func temporary(err error) bool {
	var requestErr *requestError
	if errors.As(err, &requestErr) {
		return !requestErr.permanent
	}
	return false
}

func (c *Client) jitter(base time.Duration) time.Duration {
	if c.cfg.Jitter == 0 {
		return base
	}
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return base
	}
	span := int64(c.cfg.Jitter)*2 + 1
	offset := int64(binary.LittleEndian.Uint64(random[:])%uint64(span)) - int64(c.cfg.Jitter)
	value := base + time.Duration(offset)
	if value < 100*time.Millisecond {
		return 100 * time.Millisecond
	}
	return value
}

func randomNonce() (string, error) {
	value := make([]byte, 18)
	defer clear(value)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

func wait(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
