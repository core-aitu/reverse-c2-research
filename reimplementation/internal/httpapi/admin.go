package httpapi

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/store"
)

func (s *Server) adminCatalogRoutes() {
	s.mux.Handle("GET /api/Artifacts", s.protected(http.HandlerFunc(s.artifacts)))
	s.mux.Handle("GET /api/Artifacts/Artifacts/{host}", s.protected(http.HandlerFunc(s.hostArtifacts)))
	s.mux.Handle("GET /api/Artifacts/Artifacts/{host}/{name}", s.protected(http.HandlerFunc(s.artifactContent)))
	s.mux.Handle("DELETE /api/Artifacts/Artifacts/{host}/{name}", s.requireRole("admin", http.HandlerFunc(s.deleteArtifact)))
	s.mux.Handle("PUT /api/artifacts/content/{host}/{name}", s.requireRole("admin", http.HandlerFunc(s.putArtifactContent)))

	s.mux.Handle("GET /api/License", s.protected(http.HandlerFunc(s.license)))
	s.mux.Handle("PUT /api/License", s.requireRole("admin", http.HandlerFunc(s.putLicense)))

	s.mux.Handle("GET /api/Updates", s.protected(http.HandlerFunc(s.version)))
	s.mux.Handle("POST /api/Updates", s.requireRole("admin", http.HandlerFunc(s.checkUpdates)))
	s.mux.Handle("GET /api/Updates/Updates/status", s.protected(http.HandlerFunc(s.updateStatus)))
	s.mux.Handle("GET /api/Updates/Updates/client", s.protected(http.HandlerFunc(s.updateClient)))
	s.mux.Handle("GET /api/updates/packages", s.protected(http.HandlerFunc(s.updatePackages)))
	s.mux.Handle("PUT /api/updates/packages/{version}", s.requireRole("admin", http.HandlerFunc(s.putUpdatePackage)))
	s.mux.Handle("DELETE /api/updates/packages/{version}", s.requireRole("admin", http.HandlerFunc(s.deleteUpdatePackage)))
	s.mux.Handle("GET /api/objects/{id}/content", s.requireRole("admin", http.HandlerFunc(s.objectContent)))
}

func (s *Server) objectContent(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.ObjectInfo(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "object not found")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	if value.Size == 0 {
		if r.Header.Get("Range") != "" {
			w.Header().Set("Content-Range", "bytes */0")
			writeError(w, http.StatusRequestedRangeNotSatisfiable, "empty object has no byte ranges")
			return
		}
		reader, _, openErr := s.store.OpenObject(r.Context(), value.ID, 0, -1)
		if openErr != nil {
			s.internalError(w, openErr)
			return
		}
		reader.Close()
		w.Header().Set("Content-Type", value.MediaType)
		w.Header().Set("Content-Disposition", `attachment; filename="`+safeDownloadName(value.OriginalName)+`"`)
		w.Header().Set("X-Content-SHA256", value.SHA256)
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Length", "0")
		w.WriteHeader(http.StatusOK)
		return
	}
	start, end, partial, err := parseObjectRange(r.Header.Get("Range"), value.Size)
	if err != nil {
		w.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(value.Size, 10))
		writeError(w, http.StatusRequestedRangeNotSatisfiable, "invalid object range")
		return
	}
	reader, _, err := s.store.OpenObject(r.Context(), value.ID, start, end)
	if err != nil {
		s.internalError(w, err)
		return
	}
	defer reader.Close()
	length := end - start + 1
	w.Header().Set("Content-Type", value.MediaType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeDownloadName(value.OriginalName)+`"`)
	w.Header().Set("X-Content-SHA256", value.SHA256)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	if partial {
		w.Header().Set("Content-Range", "bytes "+strconv.FormatInt(start, 10)+"-"+strconv.FormatInt(end, 10)+"/"+strconv.FormatInt(value.Size, 10))
		w.WriteHeader(http.StatusPartialContent)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	_, _ = io.CopyN(w, reader, length)
}

func parseObjectRange(header string, size int64) (int64, int64, bool, error) {
	if size < 1 {
		return 0, 0, false, errors.New("empty object")
	}
	if header == "" {
		return 0, size - 1, false, nil
	}
	if !strings.HasPrefix(header, "bytes=") || strings.Contains(header, ",") {
		return 0, 0, false, errors.New("unsupported range")
	}
	parts := strings.SplitN(strings.TrimPrefix(header, "bytes="), "-", 2)
	if len(parts) != 2 {
		return 0, 0, false, errors.New("invalid range")
	}
	if parts[0] == "" {
		suffix, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffix < 1 {
			return 0, 0, false, errors.New("invalid suffix")
		}
		if suffix > size {
			suffix = size
		}
		return size - suffix, size - 1, true, nil
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 || start >= size {
		return 0, 0, false, errors.New("invalid start")
	}
	end := size - 1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start {
			return 0, 0, false, errors.New("invalid end")
		}
		if end >= size {
			end = size - 1
		}
	}
	return start, end, true, nil
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

func (s *Server) artifactContent(w http.ResponseWriter, r *http.Request) {
	content, value, err := s.store.ArtifactContent(r.Context(), r.PathValue("host"), r.PathValue("name"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "artifact content not found")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	w.Header().Set("Content-Type", value.MediaType)
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeDownloadName(value.Name)+`"`)
	w.Header().Set("X-Content-SHA256", value.SHA256)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(value.Length, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func (s *Server) putArtifactContent(w http.ResponseWriter, r *http.Request) {
	content, ok := readBinaryBody(w, r, store.MaxArtifactContent)
	if !ok {
		return
	}
	value, err := s.store.PutArtifactContent(r.Context(), r.PathValue("host"), r.PathValue("name"), requestMediaType(r), content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "ArtifactContentStored", value.Name, value.SHA256+"; "+value.Host)
	writeJSON(w, http.StatusOK, value)
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
	s.audit(r.Context(), principal(r).Username, "UpdateCatalogCheckCompleted", "package-catalog", "no package installed")
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

func (s *Server) updatePackages(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.UpdatePackages(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) putUpdatePackage(w http.ResponseWriter, r *http.Request) {
	content, ok := readBinaryBody(w, r, store.MaxUpdatePackage)
	if !ok {
		return
	}
	value, err := s.store.PutUpdatePackage(r.Context(), r.PathValue("version"), requestMediaType(r), r.Header.Get("X-Content-SHA256"), content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "UpdatePackageStored", r.PathValue("version"), value.SHA256)
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) deleteUpdatePackage(w http.ResponseWriter, r *http.Request) {
	version := r.PathValue("version")
	if err := s.store.DeleteUpdatePackage(r.Context(), version); err != nil {
		writeCatalogError(w, err, "update package not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "UpdatePackageDeleted", version, "package content removed")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) updateClient(w http.ResponseWriter, r *http.Request) {
	content, value, err := s.store.ActiveUpdatePackageContent(r.Context())
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "no active update package")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	version := versionString(value.Version)
	w.Header().Set("Content-Type", value.MediaType)
	w.Header().Set("Content-Disposition", `attachment; filename="specter-client-`+version+`.pkg"`)
	w.Header().Set("X-Content-SHA256", value.SHA256)
	w.Header().Set("X-Specter-Version", version)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(value.Length, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

func readBinaryBody(w http.ResponseWriter, r *http.Request, maximum int) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, int64(maximum))
	content, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "binary body exceeds the configured limit")
		return nil, false
	}
	if len(content) == 0 {
		writeError(w, http.StatusBadRequest, "binary body is empty")
		return nil, false
	}
	return content, true
}

func requestMediaType(r *http.Request) string {
	value := strings.TrimSpace(r.Header.Get("Content-Type"))
	if value == "" {
		return "application/octet-stream"
	}
	parsed, _, err := mime.ParseMediaType(value)
	if err != nil || len(parsed) > 128 {
		return "application/octet-stream"
	}
	return parsed
}

func versionString(value domain.SerializableVersion) string {
	return strconv.Itoa(value.Major) + "." + strconv.Itoa(value.Minor) + "." + strconv.Itoa(value.Build) + "." + strconv.Itoa(value.Revision)
}
