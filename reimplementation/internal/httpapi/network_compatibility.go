package httpapi

import (
	"net/http"
	"strings"
)

func (s *Server) networkCompatibilityRoutes() {
	s.mux.Handle("GET /api/NetworkIntelligence/GetAllCredentials", s.protected(s.resourceList("network-credentials")))
	s.mux.Handle("GET /api/NetworkIntelligence/GetCredentialsByHostId", s.protected(s.filteredResources("network-credentials", "hostId")))
	s.mux.Handle("POST /api/NetworkIntelligence/UpsertCredential", s.requireRole("admin", s.resourcePut("network-credentials", "credentialId")))
	s.mux.Handle("GET /api/NetworkIntelligence/GetServicesByHostId", s.protected(s.filteredResources("network-services", "hostId")))
	s.mux.Handle("POST /api/NetworkIntelligence/UpsertService", s.requireRole("admin", s.resourcePut("network-services", "serviceId")))
	s.mux.Handle("GET /api/NetworkIntelligence/GetProcessesByHostId", s.protected(s.filteredResources("network-processes", "hostId")))
	s.mux.Handle("POST /api/NetworkIntelligence/ReplaceProcessSnapshot", s.requireRole("admin", s.resourcePut("network-process-snapshots", "hostId")))
	s.mux.Handle("GET /api/NetworkIntelligence/GetUsersByHostId", s.protected(s.filteredResources("network-users", "hostId")))
	s.mux.Handle("POST /api/NetworkIntelligence/UpsertUser", s.requireRole("admin", s.resourcePut("network-users", "userId")))
	s.mux.Handle("GET /api/NetworkIntelligence/GetCachedFilesByHostId", s.protected(s.filteredResources("network-cached-files", "hostId")))
	s.mux.Handle("GET /api/NetworkIntelligence/GetCachedFileById", s.protected(s.resourceGet("network-cached-files", "fileId")))
	s.mux.Handle("GET /api/NetworkIntelligence/DownloadCachedFileContent", s.requireRole("admin", s.blockedCapability("cached remote file download", "retain discovery metadata without remote retrieval")))
	s.mux.Handle("POST /api/NetworkIntelligence/UpsertCachedFile", s.requireRole("admin", s.resourcePut("network-cached-files", "fileId")))
}

func (s *Server) filteredResources(kind, queryField string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values, err := s.store.Resources(r.Context(), kind)
		if err != nil {
			s.internalError(w, err)
			return
		}
		filter := strings.TrimSpace(r.URL.Query().Get(queryField))
		result := make([]any, 0, len(values))
		for _, value := range values {
			representation := resourceRepresentation(value)
			if filter == "" || representationField(representation, queryField) == filter {
				result = append(result, representation)
			}
		}
		writeJSON(w, http.StatusOK, result)
	})
}

func representationField(value any, field string) string {
	if object, ok := value.(map[string]any); ok {
		return scalarString(object[field])
	}
	return ""
}
