package objectstore

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"regexp"
	"strings"
	"sync"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var kindPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

type Info struct {
	Key       string `json:"key"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
	MediaType string `json:"mediaType"`
}

type MultipartUpload struct {
	ID, Key, Kind, Name, MediaType string
}

type MultipartPart struct {
	Number int
	ETag   string
	Size   int64
}

type Store interface {
	Put(context.Context, string, string, string, io.Reader, int64) (Info, error)
	Open(context.Context, string, int64, int64) (io.ReadCloser, error)
	Delete(context.Context, string) error
	BeginMultipart(context.Context, string, string, string) (MultipartUpload, error)
	PutPart(context.Context, MultipartUpload, int, io.Reader, int64) (MultipartPart, error)
	CompleteMultipart(context.Context, MultipartUpload, []MultipartPart, string, int64) (Info, error)
	AbortMultipart(context.Context, MultipartUpload) error
	Ping(context.Context) error
}

type S3Config struct {
	Endpoint, AccessKey, SecretKey, Region, Bucket, Prefix string
	AutoCreate                                             bool
}

type S3 struct {
	client *minio.Client
	core   minio.Core
	bucket string
	prefix string
}

func NewS3(ctx context.Context, cfg S3Config) (*S3, error) {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.Path != "" && u.Path != "/" {
		return nil, errors.New("S3 endpoint must be an absolute HTTP(S) origin")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Bucket == "" {
		return nil, errors.New("S3 access key, secret key and bucket are required")
	}
	client, err := minio.New(u.Host, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""), Secure: u.Scheme == "https", Region: cfg.Region})
	if err != nil {
		return nil, err
	}
	exists, err := client.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, err
	}
	if !exists {
		if !cfg.AutoCreate {
			return nil, fmt.Errorf("S3 bucket %q does not exist", cfg.Bucket)
		}
		if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{Region: cfg.Region}); err != nil {
			return nil, err
		}
	}
	return &S3{client: client, core: minio.Core{Client: client}, bucket: cfg.Bucket, prefix: strings.Trim(cfg.Prefix, "/")}, nil
}

func (s *S3) Put(ctx context.Context, kind, name, mediaType string, reader io.Reader, size int64) (Info, error) {
	if !kindPattern.MatchString(kind) || size < 0 || name == "" || len(name) > 512 || len(mediaType) > 128 {
		return Info{}, errors.New("invalid object metadata")
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	stage, err := randomID()
	if err != nil {
		return Info{}, err
	}
	stageKey := s.key("staging", stage)
	hash := sha256.New()
	written, err := s.client.PutObject(ctx, s.bucket, stageKey, io.TeeReader(reader, hash), size, minio.PutObjectOptions{ContentType: mediaType})
	if err != nil {
		return Info{}, err
	}
	if written.Size != size {
		_ = s.client.RemoveObject(ctx, s.bucket, stageKey, minio.RemoveObjectOptions{})
		return Info{}, io.ErrUnexpectedEOF
	}
	digest := hex.EncodeToString(hash.Sum(nil))
	finalKey := s.key("objects", kind, digest[:2], digest)
	if _, statErr := s.client.StatObject(ctx, s.bucket, finalKey, minio.StatObjectOptions{}); statErr != nil {
		if _, err = s.client.CopyObject(ctx, minio.CopyDestOptions{Bucket: s.bucket, Object: finalKey}, minio.CopySrcOptions{Bucket: s.bucket, Object: stageKey}); err != nil {
			_ = s.client.RemoveObject(ctx, s.bucket, stageKey, minio.RemoveObjectOptions{})
			return Info{}, err
		}
	}
	if err := s.client.RemoveObject(ctx, s.bucket, stageKey, minio.RemoveObjectOptions{}); err != nil {
		return Info{}, err
	}
	return Info{Key: finalKey, Kind: kind, Name: name, SHA256: digest, Size: size, MediaType: mediaType}, nil
}

func (s *S3) Open(ctx context.Context, key string, start, end int64) (io.ReadCloser, error) {
	if !s.validKey(key) || start < 0 || end < -1 || end >= 0 && end < start {
		return nil, errors.New("invalid object range")
	}
	opts := minio.GetObjectOptions{}
	if start != 0 || end >= 0 {
		if err := opts.SetRange(start, end); err != nil {
			return nil, err
		}
	}
	object, err := s.client.GetObject(ctx, s.bucket, key, opts)
	if err != nil {
		return nil, err
	}
	if _, err := object.Stat(); err != nil {
		_ = object.Close()
		return nil, err
	}
	return object, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if !s.validKey(key) {
		return errors.New("invalid object key")
	}
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

func (s *S3) BeginMultipart(ctx context.Context, kind, name, mediaType string) (MultipartUpload, error) {
	if !kindPattern.MatchString(kind) || name == "" || len(name) > 512 || len(mediaType) > 128 {
		return MultipartUpload{}, errors.New("invalid multipart metadata")
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	stage, err := randomID()
	if err != nil {
		return MultipartUpload{}, err
	}
	key := s.key("staging", stage)
	id, err := s.core.NewMultipartUpload(ctx, s.bucket, key, minio.PutObjectOptions{ContentType: mediaType})
	if err != nil {
		return MultipartUpload{}, err
	}
	return MultipartUpload{ID: id, Key: key, Kind: kind, Name: name, MediaType: mediaType}, nil
}

func (s *S3) PutPart(ctx context.Context, upload MultipartUpload, number int, reader io.Reader, size int64) (MultipartPart, error) {
	if upload.ID == "" || !strings.HasPrefix(upload.Key, s.key("staging")+"/") || number < 1 || number > 10000 || size < 1 {
		return MultipartPart{}, errors.New("invalid multipart part")
	}
	part, err := s.core.PutObjectPart(ctx, s.bucket, upload.Key, upload.ID, number, reader, size, minio.PutObjectPartOptions{})
	if err != nil {
		return MultipartPart{}, err
	}
	return MultipartPart{Number: number, ETag: part.ETag, Size: part.Size}, nil
}

func (s *S3) CompleteMultipart(ctx context.Context, upload MultipartUpload, parts []MultipartPart, expectedSHA256 string, expectedSize int64) (Info, error) {
	if upload.ID == "" || len(parts) == 0 || !validDigest(expectedSHA256) || expectedSize < 1 {
		return Info{}, errors.New("invalid multipart completion")
	}
	completed := make([]minio.CompletePart, 0, len(parts))
	var size int64
	for index, part := range parts {
		if part.Number != index+1 || part.ETag == "" || part.Size < 1 {
			return Info{}, errors.New("multipart parts are not contiguous")
		}
		size += part.Size
		completed = append(completed, minio.CompletePart{PartNumber: part.Number, ETag: part.ETag})
	}
	if size != expectedSize {
		return Info{}, errors.New("multipart size mismatch")
	}
	if _, err := s.core.CompleteMultipartUpload(ctx, s.bucket, upload.Key, upload.ID, completed, minio.PutObjectOptions{ContentType: upload.MediaType}); err != nil {
		return Info{}, err
	}
	object, err := s.client.GetObject(ctx, s.bucket, upload.Key, minio.GetObjectOptions{})
	if err != nil {
		return Info{}, err
	}
	hash := sha256.New()
	written, err := io.Copy(hash, object)
	closeErr := object.Close()
	if err != nil || closeErr != nil || written != expectedSize || hex.EncodeToString(hash.Sum(nil)) != expectedSHA256 {
		_ = s.client.RemoveObject(ctx, s.bucket, upload.Key, minio.RemoveObjectOptions{})
		return Info{}, errors.New("multipart digest verification failed")
	}
	finalKey := s.key("objects", upload.Kind, expectedSHA256[:2], expectedSHA256)
	if _, err = s.client.CopyObject(ctx, minio.CopyDestOptions{Bucket: s.bucket, Object: finalKey}, minio.CopySrcOptions{Bucket: s.bucket, Object: upload.Key}); err != nil {
		return Info{}, err
	}
	if err = s.client.RemoveObject(ctx, s.bucket, upload.Key, minio.RemoveObjectOptions{}); err != nil {
		return Info{}, err
	}
	return Info{Key: finalKey, Kind: upload.Kind, Name: upload.Name, SHA256: expectedSHA256, Size: expectedSize, MediaType: upload.MediaType}, nil
}

func (s *S3) AbortMultipart(ctx context.Context, upload MultipartUpload) error {
	if upload.ID == "" || !strings.HasPrefix(upload.Key, s.key("staging")+"/") {
		return errors.New("invalid multipart upload")
	}
	return s.core.AbortMultipartUpload(ctx, s.bucket, upload.Key, upload.ID)
}

func (s *S3) Ping(ctx context.Context) error {
	_, err := s.client.BucketExists(ctx, s.bucket)
	return err
}

func (s *S3) key(parts ...string) string {
	values := append([]string{}, parts...)
	if s.prefix != "" {
		values = append([]string{s.prefix}, values...)
	}
	return path.Join(values...)
}

func (s *S3) validKey(key string) bool {
	return strings.HasPrefix(key, s.key("objects")+"/") && !strings.Contains(key, "..") && !strings.ContainsAny(key, "\x00\r\n\\")
}

func randomID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return hex.EncodeToString(value), nil
}

// Memory is used only by tests and explicit development wiring.
type Memory struct {
	mu        sync.RWMutex
	objects   map[string][]byte
	multipart map[string]map[int][]byte
}

func NewMemory() *Memory {
	return &Memory{objects: map[string][]byte{}, multipart: map[string]map[int][]byte{}}
}

func (m *Memory) Put(_ context.Context, kind, name, mediaType string, reader io.Reader, size int64) (Info, error) {
	if !kindPattern.MatchString(kind) || size < 0 || size > 1<<30 || name == "" {
		return Info{}, errors.New("invalid object metadata")
	}
	content, err := io.ReadAll(io.LimitReader(reader, size+1))
	if err != nil || int64(len(content)) != size {
		return Info{}, io.ErrUnexpectedEOF
	}
	digest := sha256.Sum256(content)
	digestText := hex.EncodeToString(digest[:])
	key := path.Join("objects", kind, digestText[:2], digestText)
	m.mu.Lock()
	m.objects[key] = bytes.Clone(content)
	m.mu.Unlock()
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	return Info{Key: key, Kind: kind, Name: name, SHA256: digestText, Size: size, MediaType: mediaType}, nil
}

func (m *Memory) Open(_ context.Context, key string, start, end int64) (io.ReadCloser, error) {
	m.mu.RLock()
	content, ok := m.objects[key]
	m.mu.RUnlock()
	if !ok || start < 0 || start > int64(len(content)) || start == int64(len(content)) && len(content) != 0 {
		return nil, errors.New("object not found")
	}
	if len(content) == 0 && start == 0 && end == -1 {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}
	if end < 0 || end >= int64(len(content)) {
		end = int64(len(content)) - 1
	}
	if end < start {
		return nil, errors.New("invalid object range")
	}
	return io.NopCloser(bytes.NewReader(bytes.Clone(content[start : end+1]))), nil
}

func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.objects, key)
	return nil
}

func (m *Memory) BeginMultipart(_ context.Context, kind, name, mediaType string) (MultipartUpload, error) {
	if !kindPattern.MatchString(kind) || name == "" {
		return MultipartUpload{}, errors.New("invalid multipart metadata")
	}
	id, err := randomID()
	if err != nil {
		return MultipartUpload{}, err
	}
	if mediaType == "" {
		mediaType = "application/octet-stream"
	}
	upload := MultipartUpload{ID: id, Key: path.Join("staging", id), Kind: kind, Name: name, MediaType: mediaType}
	m.mu.Lock()
	m.multipart[id] = map[int][]byte{}
	m.mu.Unlock()
	return upload, nil
}

func (m *Memory) PutPart(_ context.Context, upload MultipartUpload, number int, reader io.Reader, size int64) (MultipartPart, error) {
	if number < 1 || size < 1 {
		return MultipartPart{}, errors.New("invalid multipart part")
	}
	content, err := io.ReadAll(io.LimitReader(reader, size+1))
	if err != nil || int64(len(content)) != size {
		return MultipartPart{}, io.ErrUnexpectedEOF
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	parts, ok := m.multipart[upload.ID]
	if !ok {
		return MultipartPart{}, errors.New("multipart upload not found")
	}
	parts[number] = bytes.Clone(content)
	digest := sha256.Sum256(content)
	return MultipartPart{Number: number, ETag: hex.EncodeToString(digest[:]), Size: size}, nil
}

func (m *Memory) CompleteMultipart(ctx context.Context, upload MultipartUpload, parts []MultipartPart, expectedSHA256 string, expectedSize int64) (Info, error) {
	m.mu.Lock()
	stored, ok := m.multipart[upload.ID]
	if !ok {
		m.mu.Unlock()
		return Info{}, errors.New("multipart upload not found")
	}
	var content []byte
	for index, part := range parts {
		if part.Number != index+1 || stored[part.Number] == nil {
			m.mu.Unlock()
			return Info{}, errors.New("multipart parts are not contiguous")
		}
		content = append(content, stored[part.Number]...)
	}
	delete(m.multipart, upload.ID)
	m.mu.Unlock()
	if int64(len(content)) != expectedSize {
		return Info{}, errors.New("multipart size mismatch")
	}
	digest := sha256.Sum256(content)
	if hex.EncodeToString(digest[:]) != expectedSHA256 {
		return Info{}, errors.New("multipart digest verification failed")
	}
	return m.Put(ctx, upload.Kind, upload.Name, upload.MediaType, bytes.NewReader(content), int64(len(content)))
}

func (m *Memory) AbortMultipart(_ context.Context, upload MultipartUpload) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.multipart[upload.ID]; !ok {
		return errors.New("multipart upload not found")
	}
	delete(m.multipart, upload.ID)
	return nil
}

func (m *Memory) Ping(context.Context) error { return nil }

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
