// Package transfer implements ordered, non-resumable v1 chunk transfers.
package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	Version    = 1
	ChunkSize  = 1 << 20
	MaxFile    = 16 << 20
	MaxChunks  = MaxFile / ChunkSize
	MaxPending = 32
)

var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Manifest struct {
	Version    int    `json:"version"`
	ID         string `json:"id"`
	Host       string `json:"host"`
	Name       string `json:"name"`
	MediaType  string `json:"mediaType"`
	Size       int64  `json:"size"`
	SHA256     string `json:"sha256"`
	ChunkSize  int64  `json:"chunkSize"`
	ChunkCount int    `json:"chunkCount"`
	Resume     bool   `json:"resume"`
}

type upload struct {
	manifest  Manifest
	file      *os.File
	next      int
	written   int64
	completed bool
}

type Manager struct {
	root    string
	mu      sync.Mutex
	uploads map[string]*upload
}

func New(root string) (*Manager, error) {
	if root == "" {
		return nil, errors.New("transfer root is required")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	manager := &Manager{root: root, uploads: map[string]*upload{}}
	_, _ = manager.Cleanup(time.Now().Add(-24 * time.Hour))
	return manager, nil
}

func (m *Manager) Begin(manifest Manifest) (Manifest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	manifest.Version, manifest.ChunkSize, manifest.Resume = Version, ChunkSize, false
	manifest.ChunkCount = int((manifest.Size + ChunkSize - 1) / ChunkSize)
	if len(m.uploads) >= MaxPending || !safeComponent(manifest.ID) || !safeComponent(manifest.Host) || !safeComponent(manifest.Name) || manifest.Size < 1 || manifest.Size > MaxFile || manifest.ChunkCount < 1 || manifest.ChunkCount > MaxChunks || !digestPattern.MatchString(manifest.SHA256) || len(manifest.MediaType) > 128 {
		return Manifest{}, errors.New("invalid transfer manifest")
	}
	if _, exists := m.uploads[manifest.ID]; exists {
		return Manifest{}, errors.New("resume is not supported by transfer v1")
	}
	path := m.partPath(manifest.ID)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrExist) {
		return Manifest{}, errors.New("resume is not supported by transfer v1")
	}
	if err != nil {
		return Manifest{}, err
	}
	m.uploads[manifest.ID] = &upload{manifest: manifest, file: file}
	return manifest, nil
}

func (m *Manager) PutChunk(id string, index int, content []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	upload, ok := m.uploads[id]
	if !ok || index != upload.next || len(content) == 0 || len(content) > ChunkSize {
		return errors.New("invalid or out-of-order transfer chunk")
	}
	remaining := upload.manifest.Size - upload.written
	want := int64(ChunkSize)
	if remaining < want {
		want = remaining
	}
	if int64(len(content)) != want {
		return errors.New("chunk length does not match manifest")
	}
	n, err := upload.file.Write(content)
	if err != nil || n != len(content) {
		return io.ErrShortWrite
	}
	upload.written += int64(n)
	upload.next++
	return nil
}

func (m *Manager) Complete(id string) (Manifest, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	upload, ok := m.uploads[id]
	if !ok || upload.next != upload.manifest.ChunkCount || upload.written != upload.manifest.Size {
		return Manifest{}, nil, errors.New("transfer is incomplete")
	}
	if upload.completed {
		content, err := os.ReadFile(m.readyPath(id))
		return upload.manifest, content, err
	}
	if err := upload.file.Sync(); err != nil {
		return Manifest{}, nil, err
	}
	if err := upload.file.Close(); err != nil {
		return Manifest{}, nil, err
	}
	part, ready := m.partPath(id), m.readyPath(id)
	content, err := os.ReadFile(part)
	if err != nil {
		return Manifest{}, nil, err
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != upload.manifest.SHA256 {
		clear(content)
		_ = os.Remove(part)
		delete(m.uploads, id)
		return Manifest{}, nil, errors.New("transfer digest mismatch")
	}
	if err := os.Rename(part, ready); err != nil {
		clear(content)
		return Manifest{}, nil, err
	}
	upload.completed = true
	return upload.manifest, content, nil
}

func (m *Manager) Published(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	upload, ok := m.uploads[id]
	if !ok || !upload.completed {
		return errors.New("transfer is not ready for publication")
	}
	if err := os.Remove(m.readyPath(id)); err != nil {
		return err
	}
	delete(m.uploads, id)
	return nil
}

func (m *Manager) Abort(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if upload, ok := m.uploads[id]; ok {
		_ = upload.file.Close()
		delete(m.uploads, id)
	}
	partErr := os.Remove(m.partPath(id))
	readyErr := os.Remove(m.readyPath(id))
	if partErr != nil && !errors.Is(partErr, os.ErrNotExist) {
		return partErr
	}
	if readyErr != nil && !errors.Is(readyErr, os.ErrNotExist) {
		return readyErr
	}
	return nil
}

func (m *Manager) Cleanup(olderThan time.Time) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	entries, err := os.ReadDir(m.root)
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, entry := range entries {
		extension := filepath.Ext(entry.Name())
		if entry.IsDir() || (extension != ".part" && extension != ".ready") {
			continue
		}
		info, err := entry.Info()
		if err != nil || !info.ModTime().Before(olderThan) {
			continue
		}
		id := entry.Name()[:len(entry.Name())-len(extension)]
		if upload, active := m.uploads[id]; active {
			_ = upload.file.Close()
			delete(m.uploads, id)
		}
		if err := os.Remove(filepath.Join(m.root, entry.Name())); err != nil {
			return removed, err
		}
		removed++
	}
	return removed, nil
}

func BuildDownloadManifest(id, host, name, mediaType string, content []byte) (Manifest, error) {
	digest := sha256.Sum256(content)
	manifest := Manifest{ID: id, Host: host, Name: name, MediaType: mediaType, Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:])}
	manifest.Version, manifest.ChunkSize, manifest.Resume = Version, ChunkSize, false
	manifest.ChunkCount = int((manifest.Size + ChunkSize - 1) / ChunkSize)
	if !safeComponent(id) || !safeComponent(host) || !safeComponent(name) || len(content) == 0 || len(content) > MaxFile {
		return Manifest{}, errors.New("invalid download manifest")
	}
	return manifest, nil
}

func Chunk(content []byte, index int) ([]byte, error) {
	count := (len(content) + ChunkSize - 1) / ChunkSize
	if index < 0 || index >= count || count > MaxChunks {
		return nil, errors.New("chunk index outside manifest")
	}
	start := index * ChunkSize
	end := start + ChunkSize
	if end > len(content) {
		end = len(content)
	}
	return content[start:end], nil
}

func safeComponent(value string) bool {
	return value != "" && len(value) <= 255 && utf8.ValidString(value) && value != "." && value != ".." && filepath.Base(value) == value && !filepath.IsAbs(value) && !containsSeparator(value)
}

func containsSeparator(value string) bool {
	for _, char := range value {
		if char == '/' || char == '\\' || char == 0 || char < 0x20 {
			return true
		}
	}
	return false
}

func (m *Manager) partPath(id string) string {
	return filepath.Join(m.root, fmt.Sprintf("%s.part", id))
}
func (m *Manager) readyPath(id string) string {
	return filepath.Join(m.root, fmt.Sprintf("%s.ready", id))
}
