package httpapi

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/store"
)

func (s *Server) catalogRoutes() {
	s.mux.Handle("GET /api/Scripts", s.protected(http.HandlerFunc(s.scripts)))
	s.mux.Handle("PUT /api/Scripts", s.requireRole("admin", http.HandlerFunc(s.putScript)))
	s.mux.Handle("DELETE /api/Scripts", s.requireRole("admin", http.HandlerFunc(s.deleteScript)))

	s.mux.Handle("GET /api/Wordlists", s.protected(http.HandlerFunc(s.wordlists)))
	s.mux.Handle("POST /api/Wordlists", s.requireRole("admin", http.HandlerFunc(s.createWordlist)))
	s.mux.Handle("GET /api/Wordlists/{id}", s.protected(http.HandlerFunc(s.wordlist)))
	s.mux.Handle("PUT /api/Wordlists/{id}", s.requireRole("admin", http.HandlerFunc(s.updateWordlist)))
	s.mux.Handle("DELETE /api/Wordlists/{id}", s.requireRole("admin", http.HandlerFunc(s.deleteWordlist)))

	s.mux.Handle("GET /api/Listeners", s.protected(http.HandlerFunc(s.listeners)))
	s.mux.Handle("POST /api/Listeners", s.requireRole("admin", http.HandlerFunc(s.createListener)))
	s.mux.Handle("PUT /api/Listeners", s.requireRole("admin", http.HandlerFunc(s.updateListener)))
	s.mux.Handle("DELETE /api/Listeners", s.requireRole("admin", http.HandlerFunc(s.deleteListener)))

	s.mux.Handle("GET /api/Certificates", s.protected(http.HandlerFunc(s.certificates)))
	s.mux.Handle("PUT /api/Certificates", s.requireRole("admin", http.HandlerFunc(s.putCertificate)))
	s.mux.Handle("DELETE /api/Certificates/Certificates/{name}", s.requireRole("admin", http.HandlerFunc(s.deleteCertificate)))

	s.mux.Handle("GET /api/Modules", s.protected(http.HandlerFunc(s.modules)))
	s.mux.Handle("POST /api/Modules", s.requireRole("admin", http.HandlerFunc(s.putModule)))
	s.mux.Handle("GET /api/Modules/Modules/{name}", s.protected(http.HandlerFunc(s.moduleContent)))
	s.mux.Handle("DELETE /api/Modules/Modules/{name}", s.requireRole("admin", http.HandlerFunc(s.deleteModule)))

	s.mux.Handle("GET /api/PowerShellNotebooks", s.protected(http.HandlerFunc(s.notebooks)))
	s.mux.Handle("POST /api/PowerShellNotebooks", s.requireRole("admin", http.HandlerFunc(s.createNotebook)))
	s.mux.Handle("PUT /api/PowerShellNotebooks/upsert", s.requireRole("admin", http.HandlerFunc(s.upsertNotebook)))
	s.mux.Handle("GET /api/PowerShellNotebooks/{notebookId}", s.protected(http.HandlerFunc(s.notebook)))
	s.mux.Handle("PUT /api/PowerShellNotebooks/{notebookId}", s.requireRole("admin", http.HandlerFunc(s.updateNotebook)))
	s.mux.Handle("DELETE /api/PowerShellNotebooks/{notebookId}", s.requireRole("admin", http.HandlerFunc(s.deleteNotebook)))
	blockedNotebookSession := s.blockedCapability("notebook-powershell-session", "configure an isolated PowerShell worker")
	s.mux.Handle("POST /api/PowerShellNotebooks/{notebookId}/session", s.requireRole("admin", blockedNotebookSession))
	s.mux.Handle("DELETE /api/PowerShellNotebooks/{notebookId}/session", s.requireRole("admin", blockedNotebookSession))
	s.mux.Handle("POST /api/PowerShellNotebooks/{notebookId}/session/reset", s.requireRole("admin", blockedNotebookSession))
}

func (s *Server) scripts(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.Scripts(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) putScript(w http.ResponseWriter, r *http.Request) {
	var value domain.SpecterScript
	if !decodeJSON(w, r, &value) {
		return
	}
	if err := s.store.PutScript(r.Context(), value); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "ScriptUpdated", value.Identifier, value.Name)
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) deleteScript(w http.ResponseWriter, r *http.Request) {
	identifier := strings.TrimSpace(r.URL.Query().Get("identifier"))
	if identifier == "" {
		writeError(w, http.StatusBadRequest, "identifier is required")
		return
	}
	if err := s.store.DeleteScript(r.Context(), identifier); err != nil {
		writeCatalogError(w, err, "script not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "ScriptDeleted", identifier, "script metadata removed")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) wordlists(w http.ResponseWriter, r *http.Request) {
	tags := r.URL.Query()["tags"]
	if len(tags) == 1 && strings.Contains(tags[0], ",") {
		tags = strings.Split(tags[0], ",")
	}
	values, err := s.store.Wordlists(r.Context(), tags)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) wordlist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	value, err := s.store.Wordlist(r.Context(), id)
	if err != nil {
		writeCatalogError(w, err, "wordlist not found")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) createWordlist(w http.ResponseWriter, r *http.Request) {
	var request domain.AddWordlistRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	value, err := s.store.CreateWordlist(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "WordlistCreated", strconv.FormatInt(value.ID, 10), value.Name)
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) updateWordlist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	var request domain.AddWordlistRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	value, err := s.store.UpdateWordlist(r.Context(), id, request)
	if err != nil {
		writeCatalogError(w, err, "wordlist not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "WordlistUpdated", strconv.FormatInt(id, 10), value.Name)
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) deleteWordlist(w http.ResponseWriter, r *http.Request) {
	id, ok := pathInt64(w, r, "id")
	if !ok {
		return
	}
	if err := s.store.DeleteWordlist(r.Context(), id); err != nil {
		writeCatalogError(w, err, "wordlist not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "WordlistDeleted", strconv.FormatInt(id, 10), "wordlist removed")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) listeners(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.Listeners(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) createListener(w http.ResponseWriter, r *http.Request) {
	var request domain.AddHTTPListenerRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if err := s.store.CreateListener(r.Context(), request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "ListenerCreated", request.Prefix, "configuration only; no socket opened")
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) updateListener(w http.ResponseWriter, r *http.Request) {
	var request domain.UpdateListenerRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if err := s.store.UpdateListener(r.Context(), request); err != nil {
		writeCatalogError(w, err, "listener not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "ListenerUpdated", request.Prefix, strconv.FormatBool(request.Enabled))
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) deleteListener(w http.ResponseWriter, r *http.Request) {
	var prefix string
	if !decodeJSON(w, r, &prefix) {
		return
	}
	if err := s.store.DeleteListener(r.Context(), prefix); err != nil {
		writeCatalogError(w, err, "listener not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "ListenerDeleted", prefix, "listener configuration removed")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) certificates(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.Certificates(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) putCertificate(w http.ResponseWriter, r *http.Request) {
	var value domain.CertificateInfo
	if !decodeJSON(w, r, &value) {
		return
	}
	if err := s.store.PutCertificate(r.Context(), value); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "CertificateMetadataUpdated", value.Name, value.CommonName)
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) deleteCertificate(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.store.DeleteCertificate(r.Context(), name); err != nil {
		writeCatalogError(w, err, "certificate not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "CertificateMetadataDeleted", name, "metadata removed")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) modules(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.Modules(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) putModule(w http.ResponseWriter, r *http.Request) {
	// The generated legacy client represents byte[] as a JSON base64 string.
	// The blob is retained for compatibility but is never loaded or executed.
	r.Body = http.MaxBytesReader(w, r.Body, 6<<20)
	var content []byte
	if !decodeJSON(w, r, &content) {
		return
	}
	if len(content) == 0 || len(content) > 4<<20 {
		writeError(w, http.StatusBadRequest, "module content must be between 1 byte and 4 MiB")
		return
	}
	digest := sha256.Sum256(content)
	name := "module-" + hex.EncodeToString(digest[:6]) + ".bin"
	if err := s.store.PutModule(r.Context(), name, content, hex.EncodeToString(digest[:])); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "ModuleStored", name, "opaque blob; never loaded or executed")
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) moduleContent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	content, digest, err := s.store.ModuleContent(r.Context(), name)
	if err != nil {
		writeCatalogError(w, err, "module not found")
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeDownloadName(name)+`"`)
	w.Header().Set("X-Content-SHA256", digest)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) deleteModule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.store.DeleteModule(r.Context(), name); err != nil {
		writeCatalogError(w, err, "module not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "ModuleDeleted", name, "opaque blob removed")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func safeDownloadName(name string) string {
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	return name
}

func (s *Server) notebooks(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.Notebooks(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) notebook(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.Notebook(r.Context(), r.PathValue("notebookId"))
	if err != nil {
		writeCatalogError(w, err, "notebook not found")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) createNotebook(w http.ResponseWriter, r *http.Request) {
	var request domain.NotebookRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	value, err := s.store.CreateNotebook(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "NotebookCreated", value.NotebookID, value.Name)
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) updateNotebook(w http.ResponseWriter, r *http.Request) {
	var request domain.NotebookRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	value, err := s.store.UpdateNotebook(r.Context(), r.PathValue("notebookId"), request)
	if err != nil {
		writeCatalogError(w, err, "notebook not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "NotebookUpdated", value.NotebookID, value.Name)
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) upsertNotebook(w http.ResponseWriter, r *http.Request) {
	var value domain.PowerShellNotebook
	if !decodeJSON(w, r, &value) {
		return
	}
	request := domain.NotebookRequest{Name: value.Name, Description: value.Description, Cells: value.Cells}
	if strings.TrimSpace(value.NotebookID) == "" {
		created, err := s.store.CreateNotebook(r.Context(), request)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.audit(r.Context(), principal(r).Username, "NotebookCreated", created.NotebookID, created.Name)
		writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
		return
	}
	updated, err := s.store.UpsertNotebook(r.Context(), value.NotebookID, request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "NotebookUpserted", updated.NotebookID, updated.Name)
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) deleteNotebook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("notebookId")
	if err := s.store.DeleteNotebook(r.Context(), id); err != nil {
		writeCatalogError(w, err, "notebook not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "NotebookDeleted", id, "notebook removed")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func pathInt64(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	value, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || value < 1 {
		writeError(w, http.StatusBadRequest, "invalid "+name)
		return 0, false
	}
	return value, true
}

func writeCatalogError(w http.ResponseWriter, err error, notFound string) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, notFound)
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}
