package store

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/objectstore"
)

func TestConcurrentGatewayLeasingAndIdempotentResult(t *testing.T) {
	databaseURL := os.Getenv("SPECTER_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SPECTER_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	storage, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if err = storage.Migrate(ctx, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	legacyHost, legacyName := "legacy-host-"+suffix, "legacy-"+suffix+".bin"
	legacyContent := []byte("legacy-object-" + suffix)
	legacyDigest := sha256.Sum256(legacyContent)
	var legacyArtifactID int64
	err = storage.pool.QueryRow(ctx, `INSERT INTO artifacts(sha256,name,extension,username,filesize,sourcetype,source,sourcepath,createdtimestamp,ingestedtimestamp)
		VALUES($1,$2,'','',$3,'legacy-fixture',$4,'',now(),now()) RETURNING id`, hex.EncodeToString(legacyDigest[:]), legacyName, len(legacyContent), legacyHost).Scan(&legacyArtifactID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = storage.pool.Exec(ctx, `INSERT INTO app_artifact_contents(artifact_id,sha256,media_type,content) VALUES($1,$2,'application/octet-stream',$3)`, legacyArtifactID, hex.EncodeToString(legacyDigest[:]), legacyContent); err != nil {
		t.Fatal(err)
	}
	updateContent := []byte("legacy-update-" + suffix)
	updateDigest := sha256.Sum256(updateContent)
	updateBuild := time.Now().UnixNano()%60_000 + 1
	updateVersion := fmt.Sprintf("7.0.%d.0", updateBuild)
	if _, err = storage.pool.Exec(ctx, `UPDATE app_update_packages SET active=false WHERE active=true`); err != nil {
		t.Fatal(err)
	}
	if _, err = storage.pool.Exec(ctx, `INSERT INTO app_update_packages(version_key,major,minor,build,revision,sha256,media_type,content,active)
		VALUES($1,7,0,$2,0,$3,'application/octet-stream',$4,true)`, updateVersion, updateBuild, hex.EncodeToString(updateDigest[:]), updateContent); err != nil {
		t.Fatal(err)
	}
	storage.AttachObjectStore(objectstore.NewMemory())
	migrated, err := storage.MigrateLegacyObjects(ctx, 1000)
	if err != nil || migrated < 2 {
		t.Fatalf("legacy migration count=%d err=%v", migrated, err)
	}
	if migrated, err = storage.MigrateLegacyObjects(ctx, 1000); err != nil || migrated != 0 {
		t.Fatalf("legacy migration replay count=%d err=%v", migrated, err)
	}
	gotLegacy, _, err := storage.ArtifactContent(ctx, legacyHost, legacyName)
	if err != nil || !bytes.Equal(gotLegacy, legacyContent) {
		t.Fatalf("migrated artifact=%q err=%v", gotLegacy, err)
	}
	gotUpdate, _, err := storage.ActiveUpdatePackageContent(ctx)
	if err != nil || !bytes.Equal(gotUpdate, updateContent) {
		t.Fatalf("migrated update=%q err=%v", gotUpdate, err)
	}
	agentID := "concurrent-agent-" + suffix
	if _, err = storage.pool.Exec(ctx, `INSERT INTO app_agent_identities(id,name,identity_hash) VALUES($1,$1,$2)`, agentID, tokenDigest("identity-"+suffix)); err != nil {
		t.Fatal(err)
	}
	if err = storage.UseAgentNonce(ctx, agentID, "nonce-0123456789", 1, time.Now().Add(-time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = storage.UseAgentNonce(ctx, agentID, "nonce-0123456789", 1, time.Now().Add(-time.Hour)); err == nil {
		t.Fatal("replayed nonce accepted")
	}
	t.Cleanup(func() {
		_, _ = storage.pool.Exec(context.Background(), `DELETE FROM app_agent_identities WHERE id=$1`, agentID)
	})
	for index := 0; index < 2; index++ {
		_, err = storage.CreateGatewayTask(ctx, domain.GatewayTask{ID: fmt.Sprintf("task-%d-%s", index, suffix), AgentID: agentID, Version: 1, IdempotencyKey: fmt.Sprintf("idem-%d-%s", index, suffix), Kind: "echo", Payload: json.RawMessage(`{"value":"test"}`), Deadline: time.Now().Add(time.Minute), TimeoutMS: 1000})
		if err != nil {
			t.Fatal(err)
		}
	}
	start := make(chan struct{})
	leased := make(chan domain.GatewayTask, 2)
	errorsSeen := make(chan error, 2)
	var wait sync.WaitGroup
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			values, leaseErr := storage.LeaseGatewayTasks(ctx, agentID, 0, 1, time.Minute)
			if leaseErr != nil {
				errorsSeen <- leaseErr
				return
			}
			if len(values) != 1 {
				errorsSeen <- fmt.Errorf("leased %d tasks", len(values))
				return
			}
			leased <- values[0]
		}()
	}
	close(start)
	wait.Wait()
	close(leased)
	close(errorsSeen)
	for leaseErr := range errorsSeen {
		t.Fatal(leaseErr)
	}
	var tasks []domain.GatewayTask
	for task := range leased {
		tasks = append(tasks, task)
	}
	if len(tasks) != 2 || tasks[0].ID == tasks[1].ID {
		t.Fatalf("leases were not exclusive: %#v", tasks)
	}
	selected := tasks[0]
	if err = storage.AckGatewayTask(ctx, agentID, selected.ID, selected.Cursor); err != nil {
		t.Fatal(err)
	}
	start = make(chan struct{})
	results := make(chan bool, 2)
	errorsSeen = make(chan error, 2)
	for range 2 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			<-start
			idempotent, resultErr := storage.CompleteGatewayTask(ctx, agentID, selected.ID, "submission-"+suffix, 1, true, json.RawMessage(`{"value":"test"}`), "", "", false)
			if resultErr != nil {
				errorsSeen <- resultErr
				return
			}
			results <- idempotent
		}()
	}
	close(start)
	wait.Wait()
	close(results)
	close(errorsSeen)
	for resultErr := range errorsSeen {
		t.Fatal(resultErr)
	}
	first, replay := 0, 0
	for idempotent := range results {
		if idempotent {
			replay++
		} else {
			first++
		}
	}
	if first != 1 || replay != 1 {
		t.Fatalf("first=%d replay=%d", first, replay)
	}
	outputTask := tasks[1]
	if outputTask.ID == selected.ID {
		outputTask = tasks[0]
	}
	if err = storage.AckGatewayTask(ctx, agentID, outputTask.ID, outputTask.Cursor); err != nil {
		t.Fatal(err)
	}
	stdout := domain.TaskOutputChunk{TaskID: outputTask.ID, Stream: "stdout", Index: 0, Content: []byte("ordered-output")}
	if _, duplicate, chunkErr := storage.StoreTaskOutputChunk(ctx, agentID, stdout); chunkErr != nil || duplicate {
		t.Fatalf("first output chunk duplicate=%v err=%v", duplicate, chunkErr)
	}
	if _, duplicate, chunkErr := storage.StoreTaskOutputChunk(ctx, agentID, stdout); chunkErr != nil || !duplicate {
		t.Fatalf("output replay duplicate=%v err=%v", duplicate, chunkErr)
	}
	conflict := stdout
	conflict.Content = []byte("conflict")
	if _, _, chunkErr := storage.StoreTaskOutputChunk(ctx, agentID, conflict); chunkErr == nil {
		t.Fatal("conflicting duplicate output chunk accepted")
	}
	for _, chunk := range []domain.TaskOutputChunk{
		{TaskID: outputTask.ID, Stream: "stdout", Index: 1, EOF: true},
		{TaskID: outputTask.ID, Stream: "stderr", Index: 0, EOF: true},
	} {
		if _, _, chunkErr := storage.StoreTaskOutputChunk(ctx, agentID, chunk); chunkErr != nil {
			t.Fatal(chunkErr)
		}
	}
	outputs, err := storage.FinalizeTaskOutput(ctx, outputTask.ID)
	if err != nil || len(outputs) != 2 {
		t.Fatalf("archived outputs=%#v err=%v", outputs, err)
	}
	if chunks, chunkErr := storage.TaskOutputChunks(ctx, outputTask.ID, -1); chunkErr != nil || len(chunks) != 0 {
		t.Fatalf("temporary output chunks=%#v err=%v", chunks, chunkErr)
	}
	if outputs, err = storage.FinalizeTaskOutput(ctx, outputTask.ID); err != nil || len(outputs) != 2 {
		t.Fatalf("idempotent output archival=%#v err=%v", outputs, err)
	}
	defaultID := "default-timeout-" + suffix
	created, err := storage.CreateGatewayTask(ctx, domain.GatewayTask{ID: defaultID, AgentID: agentID, Version: 2, IdempotencyKey: "idem-" + defaultID, Kind: "exec", Payload: json.RawMessage(`{"executable":"/bin/true"}`), Deadline: time.Now().Add(2 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	if created.TimeoutMS != 60_000 {
		t.Fatalf("default timeout=%d", created.TimeoutMS)
	}
}

func TestAgentFileTransferConcurrencyLimit(t *testing.T) {
	databaseURL := os.Getenv("SPECTER_TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("SPECTER_TEST_DATABASE_URL is not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	storage, err := Open(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer storage.Close()
	if err = storage.Migrate(ctx, "../../migrations"); err != nil {
		t.Fatal(err)
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	agentID := "file-limit-agent-" + suffix
	if _, err = storage.pool.Exec(ctx, `INSERT INTO app_agent_identities(id,name,identity_hash) VALUES($1,$1,$2)`, agentID, tokenDigest("identity-"+suffix)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = storage.pool.Exec(context.Background(), `DELETE FROM app_agent_identities WHERE id=$1`, agentID)
	})
	for index := 0; index < 3; index++ {
		id := fmt.Sprintf("file-limit-task-%d-%s", index, suffix)
		if _, err = storage.CreateGatewayTask(ctx, domain.GatewayTask{ID: id, AgentID: agentID, Version: 2, IdempotencyKey: "idem-" + id, Kind: "file-download", Payload: json.RawMessage(`{"path":"/tmp/file"}`), Deadline: time.Now().Add(time.Minute), TimeoutMS: 5000}); err != nil {
			t.Fatal(err)
		}
	}
	tasks, err := storage.LeaseGatewayTasks(ctx, agentID, 0, 3, time.Minute)
	if err != nil || len(tasks) != 3 {
		t.Fatalf("leased=%d err=%v", len(tasks), err)
	}
	for _, task := range tasks {
		if err = storage.AckGatewayTask(ctx, agentID, task.ID, task.Cursor); err != nil {
			t.Fatal(err)
		}
	}
	digest := strings.Repeat("a", 64)
	create := func(index int) error {
		_, createErr := storage.CreateAgentFileTransfer(ctx, domain.AgentFileTransfer{
			ID: fmt.Sprintf("file-limit-%d-%s", index, suffix), TaskID: tasks[index].ID, AgentID: agentID,
			Name: "file.bin", SourcePath: "/tmp/file", ExpectedSize: 1, ExpectedSHA256: digest,
			MultipartUploadID: fmt.Sprintf("upload-%d", index), MultipartObjectKey: fmt.Sprintf("staging/%d", index),
		})
		return createErr
	}
	if err = create(0); err != nil {
		t.Fatal(err)
	}
	if err = create(1); err != nil {
		t.Fatal(err)
	}
	if err = create(2); err == nil {
		t.Fatal("third concurrent file transfer was accepted")
	}
	if err = storage.FailAgentFileTransfer(ctx, "file-limit-0-"+suffix, agentID, "aborted"); err != nil {
		t.Fatal(err)
	}
	if err = create(2); err != nil {
		t.Fatalf("released transfer slot was not reusable: %v", err)
	}
}
