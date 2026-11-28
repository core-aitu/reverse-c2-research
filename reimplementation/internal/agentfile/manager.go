package agentfile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
)

const (
	ChunkSize       = 8 << 20
	MaxFile         = 1 << 30
	MaxPerAgent     = 2
	MaxTransferName = 255
)

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Manifest struct {
	ID, AgentID, TaskID, Name, SourcePath, MediaType, SHA256 string
	Size                                                     int64
}

type upload struct {
	manifest Manifest
	file     *os.File
	next     int
	written  int64
}

type Manager struct {
	root    string
	mu      sync.Mutex
	uploads map[string]*upload
}

func New(root string) (*Manager, error) {
	if root == "" {
		return nil, errors.New("agent file transfer root is required")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	return &Manager{root: root, uploads: map[string]*upload{}}, nil
}

func (m *Manager) Begin(value Manifest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if value.ID == "" || value.AgentID == "" || value.TaskID == "" || value.Name == "" || len(value.Name) > MaxTransferName || value.Size < 1 || value.Size > MaxFile || !digestPattern.MatchString(value.SHA256) {
		return errors.New("invalid file transfer manifest")
	}
	active := 0
	for _, current := range m.uploads {
		if current.manifest.AgentID == value.AgentID {
			active++
		}
	}
	if active >= MaxPerAgent {
		return errors.New("agent file transfer concurrency exceeded")
	}
	if _, exists := m.uploads[value.ID]; exists {
		return errors.New("file transfer already exists")
	}
	file, err := os.OpenFile(filepath.Join(m.root, value.ID+".part"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	m.uploads[value.ID] = &upload{manifest: value, file: file}
	return nil
}

func (m *Manager) PutChunk(id, agentID string, index int, content []byte) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.uploads[id]
	if !ok || value.manifest.AgentID != agentID || index != value.next || len(content) == 0 || len(content) > ChunkSize || value.written+int64(len(content)) > value.manifest.Size {
		return 0, errors.New("invalid or out-of-order file chunk")
	}
	written, err := value.file.Write(content)
	if err != nil || written != len(content) {
		return value.written, io.ErrShortWrite
	}
	value.written += int64(written)
	value.next++
	return value.written, nil
}

func (m *Manager) Complete(ctx context.Context, id, agentID string) (Manifest, *os.File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	value, ok := m.uploads[id]
	if !ok || value.manifest.AgentID != agentID || value.written != value.manifest.Size {
		return Manifest{}, nil, errors.New("file transfer is incomplete")
	}
	if err := value.file.Sync(); err != nil {
		return Manifest{}, nil, err
	}
	if err := value.file.Close(); err != nil {
		return Manifest{}, nil, err
	}
	path := filepath.Join(m.root, id+".part")
	reader, err := os.Open(path)
	if err != nil {
		return Manifest{}, nil, err
	}
	hash := sha256.New()
	if _, err = io.Copy(hash, &contextReader{ctx: ctx, reader: reader}); err != nil {
		reader.Close()
		return Manifest{}, nil, err
	}
	if hex.EncodeToString(hash.Sum(nil)) != value.manifest.SHA256 {
		reader.Close()
		_ = os.Remove(path)
		delete(m.uploads, id)
		return Manifest{}, nil, errors.New("file transfer digest mismatch")
	}
	if _, err = reader.Seek(0, io.SeekStart); err != nil {
		reader.Close()
		return Manifest{}, nil, err
	}
	return value.manifest, reader, nil
}

func (m *Manager) Published(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if value, ok := m.uploads[id]; ok {
		_ = value.file.Close()
		delete(m.uploads, id)
	}
	_ = os.Remove(filepath.Join(m.root, id+".part"))
}

func (m *Manager) Abort(id string) {
	m.Published(id)
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
