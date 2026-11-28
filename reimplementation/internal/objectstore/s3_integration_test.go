package objectstore

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"testing"
)

func TestS3RoundTrip(t *testing.T) {
	endpoint := os.Getenv("SPECTER_TEST_S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("SPECTER_TEST_S3_ENDPOINT is not configured")
	}
	ctx := context.Background()
	store, err := NewS3(ctx, S3Config{
		Endpoint: endpoint, AccessKey: os.Getenv("SPECTER_TEST_S3_ACCESS_KEY"),
		SecretKey: os.Getenv("SPECTER_TEST_S3_SECRET_KEY"), Bucket: os.Getenv("SPECTER_TEST_S3_BUCKET"),
		Region: "us-east-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("s3-integration-object")
	info, err := store.Put(ctx, "artifact", "integration.bin", "application/octet-stream", bytes.NewReader(content), int64(len(content)))
	if err != nil {
		t.Fatal(err)
	}
	reader, err := store.Open(ctx, info.Key, 3, 9)
	if err != nil {
		t.Fatal(err)
	}
	got, err := io.ReadAll(reader)
	reader.Close()
	if err != nil || string(got) != "integra" {
		t.Fatalf("unexpected range %q: %v", got, err)
	}
	if err := store.Delete(ctx, info.Key); err != nil {
		t.Fatal(err)
	}
	empty, err := store.Put(ctx, "beacon-file", "empty.bin", "application/octet-stream", bytes.NewReader(nil), 0)
	if err != nil || empty.Size != 0 {
		t.Fatalf("empty S3 object=%#v err=%v", empty, err)
	}
	if err = store.Delete(ctx, empty.Key); err != nil {
		t.Fatal(err)
	}
	multipart, err := store.BeginMultipart(ctx, "artifact", "multipart.bin", "application/octet-stream")
	if err != nil {
		t.Fatal(err)
	}
	partOne := bytes.Repeat([]byte("a"), 5<<20)
	partTwo := []byte("tail")
	first, err := store.PutPart(ctx, multipart, 1, bytes.NewReader(partOne), int64(len(partOne)))
	if err != nil {
		_ = store.AbortMultipart(ctx, multipart)
		t.Fatal(err)
	}
	first, err = store.PutPart(ctx, multipart, 1, bytes.NewReader(partOne), int64(len(partOne)))
	if err != nil {
		_ = store.AbortMultipart(ctx, multipart)
		t.Fatal(err)
	}
	second, err := store.PutPart(ctx, multipart, 2, bytes.NewReader(partTwo), int64(len(partTwo)))
	if err != nil {
		_ = store.AbortMultipart(ctx, multipart)
		t.Fatal(err)
	}
	all := append(append([]byte{}, partOne...), partTwo...)
	digest := sha256.Sum256(all)
	completed, err := store.CompleteMultipart(ctx, multipart, []MultipartPart{first, second}, hex.EncodeToString(digest[:]), int64(len(all)))
	if err != nil {
		t.Fatal(err)
	}
	if err = store.Delete(ctx, completed.Key); err != nil {
		t.Fatal(err)
	}
	aborted, err := store.BeginMultipart(ctx, "artifact", "aborted.bin", "application/octet-stream")
	if err != nil {
		t.Fatal(err)
	}
	if err = store.AbortMultipart(ctx, aborted); err != nil {
		t.Fatal(err)
	}
	if _, err = store.PutPart(ctx, aborted, 1, bytes.NewReader([]byte("x")), 1); err == nil {
		t.Fatal("aborted S3 multipart upload accepted a part")
	}
}
