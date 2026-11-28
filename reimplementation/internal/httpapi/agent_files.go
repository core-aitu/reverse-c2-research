package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/research/specter-reimplementation/internal/agentfile"
	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/gateway"
	"github.com/research/specter-reimplementation/internal/objectstore"
	"github.com/research/specter-reimplementation/internal/store"
)

func (s *Server) agentFileBeginV1(w http.ResponseWriter, r *http.Request) {
	if !s.store.HasObjectStore() {
		writeError(w, http.StatusServiceUnavailable, "agent file storage unavailable")
		return
	}
	frame, ok := s.protocolRequest(w, r)
	if !ok {
		return
	}
	agent, ok := s.agentPrincipal(w, r)
	if !ok {
		return
	}
	manifest, err := gateway.DecodeFileTransferManifest(frame)
	if err != nil {
		writeError(w, http.StatusBadRequest, "malformed file transfer manifest")
		return
	}
	if !s.verifyAgentRequest(w, r, agent.ID, frame, manifest.Sequence) {
		return
	}
	task, err := s.store.GatewayTask(r.Context(), agent.ID, manifest.TaskID)
	if err != nil || task.Kind != "file-download" || manifest.SubmissionID != task.ID+":"+task.IdempotencyKey {
		writeError(w, http.StatusConflict, "file transfer task mismatch")
		return
	}
	var payload struct {
		Path string `json:"path"`
	}
	if json.Unmarshal(task.Payload, &payload) != nil || payload.Path == "" {
		writeError(w, http.StatusConflict, "file transfer task payload is invalid")
		return
	}
	if err = s.store.AdvanceAgentSequence(r.Context(), agent.ID, manifest.Sequence, false); err != nil {
		writeError(w, http.StatusConflict, "replayed file transfer sequence")
		return
	}
	transferID := newID("file")
	upload, err := s.store.BeginObjectMultipart(r.Context(), "beacon-file", manifest.Name, manifest.MediaType)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	_, err = s.store.CreateAgentFileTransfer(r.Context(), domain.AgentFileTransfer{
		ID: transferID, TaskID: manifest.TaskID, AgentID: agent.ID, Name: manifest.Name,
		SourcePath: payload.Path, MediaType: manifest.MediaType, ExpectedSize: int64(manifest.Size), ExpectedSHA256: manifest.SHA256,
		MultipartUploadID: upload.ID, MultipartObjectKey: upload.Key,
	})
	if err != nil {
		_ = s.store.AbortObjectMultipart(r.Context(), upload)
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.audit(r.Context(), agent.ID, "AgentFileTransferStarted", transferID, manifest.TaskID+"; "+manifest.Name)
	w.Header().Set("X-Specter-Transfer-ID", transferID)
	w.Header().Set("X-Specter-Chunk-Size", strconv.Itoa(agentfile.ChunkSize))
	writeFrame(w, http.StatusCreated, mustAck(gateway.Ack{Sequence: manifest.Sequence}))
}

func (s *Server) agentFileChunkV1(w http.ResponseWriter, r *http.Request) {
	if !s.store.HasObjectStore() {
		writeError(w, http.StatusServiceUnavailable, "agent file storage unavailable")
		return
	}
	if r.Header.Get("X-Specter-Protocol-Version") != strconv.Itoa(gateway.Version) || requestMediaType(r) != "application/octet-stream" {
		writeError(w, http.StatusUnsupportedMediaType, "agent file chunks require binary media type")
		return
	}
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 {
		writeError(w, http.StatusBadRequest, "invalid file chunk index")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, agentfile.ChunkSize)
	content, err := io.ReadAll(r.Body)
	if err != nil || len(content) == 0 || len(content) > agentfile.ChunkSize {
		writeError(w, http.StatusRequestEntityTooLarge, "file chunk exceeds policy")
		return
	}
	agent, ok := s.agentPrincipal(w, r)
	if !ok {
		return
	}
	sequence, err := strconv.ParseUint(r.Header.Get("X-Specter-Sequence"), 10, 64)
	if err != nil || sequence == 0 || !s.verifyAgentRequest(w, r, agent.ID, content, sequence) {
		if err != nil || sequence == 0 {
			writeError(w, http.StatusBadRequest, "invalid file chunk sequence")
		}
		return
	}
	transfer, err := s.store.AgentFileTransfer(r.Context(), r.PathValue("id"), agent.ID)
	if err != nil || transfer.NextChunk != index {
		writeError(w, http.StatusConflict, "file transfer state mismatch")
		return
	}
	upload := objectstore.MultipartUpload{ID: transfer.MultipartUploadID, Key: transfer.MultipartObjectKey, Kind: "beacon-file", Name: transfer.Name, MediaType: transfer.MediaType}
	if !fileChunkFits(transfer.ExpectedSize, transfer.ReceivedSize, len(content)) {
		_ = s.store.AbortObjectMultipart(r.Context(), upload)
		_ = s.store.FailAgentFileTransfer(r.Context(), r.PathValue("id"), agent.ID, "failed")
		writeError(w, http.StatusRequestEntityTooLarge, "file transfer exceeds declared size")
		return
	}
	if err = s.store.AdvanceAgentSequence(r.Context(), agent.ID, sequence, false); err != nil {
		writeError(w, http.StatusConflict, "replayed file chunk sequence")
		return
	}
	part, err := s.store.PutObjectPart(r.Context(), upload, index+1, bytes.NewReader(content), int64(len(content)))
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	received := transfer.ReceivedSize + int64(len(content))
	if err = s.store.AdvanceAgentFileTransfer(r.Context(), r.PathValue("id"), agent.ID, index, received, part); err != nil {
		_ = s.store.AbortObjectMultipart(r.Context(), upload)
		_ = s.store.FailAgentFileTransfer(r.Context(), r.PathValue("id"), agent.ID, "failed")
		writeError(w, http.StatusConflict, "file transfer state mismatch")
		return
	}
	writeFrame(w, http.StatusOK, mustAck(gateway.Ack{Sequence: sequence}))
}

func (s *Server) agentFileCompleteV1(w http.ResponseWriter, r *http.Request) {
	if !s.store.HasObjectStore() {
		writeError(w, http.StatusServiceUnavailable, "agent file storage unavailable")
		return
	}
	frame, ok := s.protocolRequest(w, r)
	if !ok {
		return
	}
	agent, ok := s.agentPrincipal(w, r)
	if !ok {
		return
	}
	completion, err := gateway.DecodeFileTransferComplete(frame)
	if err != nil || completion.TransferID != r.PathValue("id") {
		writeError(w, http.StatusBadRequest, "malformed file transfer completion")
		return
	}
	if !s.verifyAgentRequest(w, r, agent.ID, frame, completion.Sequence) {
		return
	}
	transfer, err := s.store.AgentFileTransfer(r.Context(), completion.TransferID, agent.ID)
	if errors.Is(err, store.ErrNotFound) || transfer.TaskID != completion.TaskID || transfer.ExpectedSHA256 != completion.SHA256 {
		writeError(w, http.StatusConflict, "file transfer completion mismatch")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	if err = s.store.AdvanceAgentSequence(r.Context(), agent.ID, completion.Sequence, false); err != nil {
		writeError(w, http.StatusConflict, "replayed file completion sequence")
		return
	}
	upload := objectstore.MultipartUpload{ID: transfer.MultipartUploadID, Key: transfer.MultipartObjectKey, Kind: "beacon-file", Name: transfer.Name, MediaType: transfer.MediaType}
	parts := make([]objectstore.MultipartPart, 0, len(transfer.MultipartParts))
	for _, part := range transfer.MultipartParts {
		parts = append(parts, objectstore.MultipartPart{Number: part.Number, ETag: part.ETag, Size: part.Size})
	}
	var object store.ObjectRecord
	if transfer.ExpectedSize == 0 {
		_ = s.store.AbortObjectMultipart(r.Context(), upload)
		object, err = s.store.PutObject(r.Context(), "beacon-file", transfer.Name, transfer.MediaType, bytes.NewReader(nil), 0)
	} else {
		object, err = s.store.CompleteObjectMultipart(r.Context(), upload, parts, transfer.ExpectedSHA256, transfer.ExpectedSize)
	}
	if err != nil {
		_ = s.store.FailAgentFileTransfer(r.Context(), completion.TransferID, agent.ID, "failed")
		_ = s.store.AbortObjectMultipart(r.Context(), upload)
		writeError(w, http.StatusServiceUnavailable, "file object publication failed")
		return
	}
	if object.SHA256 != transfer.ExpectedSHA256 || object.Size != transfer.ExpectedSize {
		_ = s.store.FailAgentFileTransfer(r.Context(), completion.TransferID, agent.ID, "failed")
		writeError(w, http.StatusConflict, "published file digest mismatch")
		return
	}
	if err = s.store.CompleteAgentFileTransfer(r.Context(), completion.TransferID, agent.ID, object.ID, time.Now().UTC()); err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r.Context(), agent.ID, "AgentFileTransferCompleted", completion.TransferID, completion.TaskID+"; "+object.SHA256)
	w.Header().Set("X-Specter-Object-ID", object.ID)
	writeFrame(w, http.StatusOK, mustAck(gateway.Ack{Sequence: completion.Sequence}))
}

func fileChunkFits(expected, received int64, chunkSize int) bool {
	return expected >= 0 && received >= 0 && received <= expected && chunkSize > 0 && int64(chunkSize) <= expected-received
}
