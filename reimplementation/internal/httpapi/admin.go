package httpapi

import (
	"net/http"

	"github.com/research/specter-reimplementation/internal/domain"
)

func (s *Server) adminCatalogRoutes() {
	s.mux.Handle("GET /api/Artifacts", s.protected(http.HandlerFunc(s.artifacts)))
	s.mux.Handle("GET /api/Artifacts/Artifacts/{host}", s.protected(http.HandlerFunc(s.hostArtifacts)))
	s.mux.Handle("GET /api/Artifacts/Artifacts/{host}/{name}", s.protected(http.HandlerFunc(s.artifactContentUnavailable)))
	s.mux.Handle("DELETE /api/Artifacts/Artifacts/{host}/{name}", s.requireRole("admin", http.HandlerFunc(s.deleteArtifact)))

	s.mux.Handle("GET /api/License", s.protected(http.HandlerFunc(s.license)))
	s.mux.Handle("PUT /api/License", s.requireRole("admin", http.HandlerFunc(s.putLicense)))

	s.mux.Handle("GET /api/Updates", s.protected(http.HandlerFunc(s.version)))
	s.mux.Handle("POST /api/Updates", s.requireRole("admin", http.HandlerFunc(s.checkUpdates)))
	s.mux.Handle("GET /api/Updates/Updates/status", s.protected(http.HandlerFunc(s.updateStatus)))
	s.mux.Handle("GET /api/Updates/Updates/client", s.protected(http.HandlerFunc(s.updateClientUnavailable)))
}

func (s *Server) artifacts(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.Artifacts(r.Context(), "")
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) hostArtifacts(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.Artifacts(r.Context(), r.PathValue("host"))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) artifactContentUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "artifact content is not present in the metadata-only clean-room store")
}

func (s *Server) deleteArtifact(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteArtifact(r.Context(), r.PathValue("host"), r.PathValue("name")); err != nil {
		writeCatalogError(w, err, "artifact not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "ArtifactDeleted", r.PathValue("name"), r.PathValue("host"))
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) license(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.License(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) putLicense(w http.ResponseWriter, r *http.Request) {
	var value domain.LicenseInfo
	if !decodeJSON(w, r, &value) {
		return
	}
	result, err := s.store.PutLicense(r.Context(), value)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "LicenseMetadataUpdated", value.Email, "key stored as SHA-256 digest only; not validated externally")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) version(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.Version(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) checkUpdates(w http.ResponseWriter, r *http.Request) {
	if err := s.store.CheckUpdates(r.Context()); err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r.Context(), principal(r).Username, "FixtureUpdateCheckCompleted", "7.0.0.0", "no package downloaded or installed")
	writeJSON(w, http.StatusOK, map[string]bool{"checked": true})
}

func (s *Server) updateStatus(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.UpdateStatus(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) updateClientUnavailable(w http.ResponseWriter, _ *http.Request) {
	writeError(w, http.StatusNotFound, "fixture update service does not provide a client package")
}
