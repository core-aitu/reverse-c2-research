package objectstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
)

func TestMemoryContentAddressingAndRange(t *testing.T) {
	store := NewMemory()
	content := []byte("0123456789")
	info, err := store.Put(context.Background(), "artifact", "sample.bin", "application/octet-stream", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	if info.Key != "objects/artifact/84/84d89877f0d4041efb6bf91a16f0248f2fd573e6af05c19f96bedb9f882f7882" {
		t.Fatalf("unexpected content-addressed key: %s", info.Key)
	}
	reader, err := store.Open(context.Background(), info.Key, 2, 5)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	got, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "2345" {
		t.Fatalf("unexpected range %q", got)
	}
}

func TestMemoryMultipartRetryCompleteAndAbort(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	upload, err := store.BeginMultipart(ctx, "beacon-file", "large.bin", "application/octet-stream")
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.PutPart(ctx, upload, 1, bytes.NewReader([]byte("hello ")), 6)
	if err != nil {
		t.Fatal(err)
	}
	// Retrying the same part number replaces it idempotently.
	first, err = store.PutPart(ctx, upload, 1, bytes.NewReader([]byte("hello ")), 6)
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.PutPart(ctx, upload, 2, bytes.NewReader([]byte("world")), 5)
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("hello world")
	digest := sha256.Sum256(content)
	info, err := store.CompleteMultipart(ctx, upload, []MultipartPart{first, second}, hex.EncodeToString(digest[:]), int64(len(content)))
	if err != nil || info.Size != int64(len(content)) {
		t.Fatalf("multipart completion: %#v %v", info, err)
	}
	aborted, err := store.BeginMultipart(ctx, "artifact", "abort.bin", "application/octet-stream")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AbortMultipart(ctx, aborted); err != nil {
		t.Fatal(err)
	}
	if _, err = store.PutPart(ctx, aborted, 1, bytes.NewReader([]byte("x")), 1); err == nil {
		t.Fatal("aborted multipart upload accepted a part")
	}
}

func TestMemoryRejectsTruncatedInput(t *testing.T) {
	_, err := NewMemory().Put(context.Background(), "artifact", "x", "", bytes.NewReader([]byte("short")), 10)
	if err == nil {
		t.Fatal("truncated object accepted")
	}
}

func TestMemoryStoresZeroLengthObject(t *testing.T) {
	store := NewMemory()
	info, err := store.Put(context.Background(), "beacon-file", "empty.bin", "application/octet-stream", bytes.NewReader(nil), 0)
	if err != nil || info.Size != 0 || info.SHA256 != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatalf("empty object=%#v err=%v", info, err)
	}
	reader, err := store.Open(context.Background(), info.Key, 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	content, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || len(content) != 0 {
		t.Fatalf("empty content=%q err=%v", content, err)
	}
}
