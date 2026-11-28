package httpapi

import (
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/research/specter-reimplementation/internal/transfer"
)

func (s *Server) artifactTransferRoutes() {
	s.mux.Handle("POST /api/artifact-transfers", s.requireRole("admin", http.HandlerFunc(s.beginArtifactTransfer)))
	s.mux.Handle("PUT /api/artifact-transfers/{id}/chunks/{index}", s.requireRole("admin", http.HandlerFunc(s.putArtifactTransferChunk)))
	s.mux.Handle("POST /api/artifact-transfers/{id}/complete", s.requireRole("admin", http.HandlerFunc(s.completeArtifactTransfer)))
	s.mux.Handle("DELETE /api/artifact-transfers/{id}", s.requireRole("admin", http.HandlerFunc(s.abortArtifactTransfer)))
	s.mux.Handle("GET /api/artifact-transfers/download/{host}/{name}/manifest", s.protected(http.HandlerFunc(s.artifactDownloadManifest)))
	s.mux.Handle("GET /api/artifact-transfers/download/{host}/{name}/chunks/{index}", s.protected(http.HandlerFunc(s.artifactDownloadChunk)))
}

func (s *Server) beginArtifactTransfer(w http.ResponseWriter, r *http.Request) {
	if s.transfers == nil {
		writeError(w, http.StatusServiceUnavailable, "transfer storage unavailable")
		return
	}
	var request transfer.Manifest
	if !decodeJSON(w, r, &request) {
		return
	}
	request.ID = newID("transfer")
	manifest, err := s.transfers.Begin(request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "ArtifactTransferStarted", manifest.ID, manifest.Host+"/"+manifest.Name)
	writeJSON(w, http.StatusCreated, manifest)
}

func (s *Server) putArtifactTransferChunk(w http.ResponseWriter, r *http.Request) {
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil || index < 0 || index >= transfer.MaxChunks {
		writeError(w, http.StatusBadRequest, "invalid chunk index")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, transfer.ChunkSize)
	content, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "chunk exceeds policy")
		return
	}
	if err = s.transfers.PutChunk(r.PathValue("id"), index, content); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "ArtifactTransferChunkAccepted", r.PathValue("id"), strconv.Itoa(index))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) completeArtifactTransfer(w http.ResponseWriter, r *http.Request) {
	manifest, content, err := s.transfers.Complete(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	defer clear(content)
	value, err := s.store.PutArtifactContent(r.Context(), manifest.Host, manifest.Name, manifest.MediaType, content)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if err = s.transfers.Published(manifest.ID); err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r.Context(), principal(r).Username, "ArtifactTransferPublished", manifest.ID, manifest.SHA256)
	writeJSON(w, http.StatusCreated, value)
}

func (s *Server) abortArtifactTransfer(w http.ResponseWriter, r *http.Request) {
	if err := s.transfers.Abort(r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, "transfer not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "ArtifactTransferAborted", r.PathValue("id"), "temporary content removed")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) artifactDownloadManifest(w http.ResponseWriter, r *http.Request) {
	content, value, err := s.store.ArtifactContent(r.Context(), r.PathValue("host"), r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact content not found")
		return
	}
	defer clear(content)
	manifest, err := transfer.BuildDownloadManifest("download-"+value.SHA256[:16], value.Host, value.Name, value.MediaType, content)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r.Context(), principal(r).Username, "ArtifactDownloadManifest", value.Name, value.SHA256)
	writeJSON(w, http.StatusOK, manifest)
}

func (s *Server) artifactDownloadChunk(w http.ResponseWriter, r *http.Request) {
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chunk index")
		return
	}
	content, value, err := s.store.ArtifactContent(r.Context(), r.PathValue("host"), r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusNotFound, "artifact content not found")
		return
	}
	defer clear(content)
	chunk, err := transfer.Chunk(content, index)
	if err != nil {
		writeError(w, http.StatusRequestedRangeNotSatisfiable, err.Error())
		return
	}
	w.Header().Set("Content-Type", value.MediaType)
	w.Header().Set("X-Content-SHA256", value.SHA256)
	w.Header().Set("X-Chunk-Index", strconv.Itoa(index))
	s.audit(r.Context(), principal(r).Username, "ArtifactDownloadChunk", value.Name, strconv.Itoa(index)+" at "+time.Now().UTC().Format(time.RFC3339))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(chunk)
}
