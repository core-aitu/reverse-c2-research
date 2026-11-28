package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"testing"
	"time"
)

func TestChunkedUploadVerifiesDigestAndPublishes(t *testing.T) {
	manager, err := New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	content := make([]byte, ChunkSize+17)
	for i := range content {
		content[i] = byte(i)
	}
	digest := sha256.Sum256(content)
	manifest, err := manager.Begin(Manifest{ID: "transfer-1", Host: "lab-host", Name: "result.bin", MediaType: "application/octet-stream", Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:])})
	if err != nil || manifest.Resume || manifest.ChunkCount != 2 {
		t.Fatalf("manifest=%#v err=%v", manifest, err)
	}
	if err = manager.PutChunk(manifest.ID, 0, content[:ChunkSize]); err != nil {
		t.Fatal(err)
	}
	if err = manager.PutChunk(manifest.ID, 1, content[ChunkSize:]); err != nil {
		t.Fatal(err)
	}
	_, published, err := manager.Complete(manifest.ID)
	if err != nil || string(published) != string(content) {
		t.Fatalf("published=%d err=%v", len(published), err)
	}
	if err := manager.Published(manifest.ID); err != nil {
		t.Fatal(err)
	}
}

func TestTransferRejectsTraversalResumeOrderAndDigestMismatch(t *testing.T) {
	manager, _ := New(t.TempDir())
	for _, name := range []string{"../escape", "/absolute", `folder\\file`, ".."} {
		if _, err := manager.Begin(Manifest{ID: "id", Host: "host", Name: name, Size: 1, SHA256: string(make([]byte, 64))}); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	content := []byte("hello")
	digest := sha256.Sum256(content)
	manifest := Manifest{ID: "transfer-2", Host: "host", Name: "file.bin", Size: int64(len(content)), SHA256: hex.EncodeToString(digest[:])}
	if _, err := manager.Begin(manifest); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Begin(manifest); err == nil {
		t.Fatal("resume accepted")
	}
	if err := manager.PutChunk(manifest.ID, 1, content); err == nil {
		t.Fatal("out-of-order chunk accepted")
	}
	if err := manager.PutChunk(manifest.ID, 0, []byte("HELLO")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := manager.Complete(manifest.ID); err == nil {
		t.Fatal("digest mismatch accepted")
	}
}

func TestCleanupIncompleteTransfer(t *testing.T) {
	root := t.TempDir()
	manager, _ := New(root)
	path := manager.partPath("orphan")
	if err := os.WriteFile(path, []byte("partial"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	_ = os.Chtimes(path, old, old)
	removed, err := manager.Cleanup(time.Now().Add(-time.Hour))
	if err != nil || removed != 1 {
		t.Fatalf("removed=%d err=%v", removed, err)
	}
}
