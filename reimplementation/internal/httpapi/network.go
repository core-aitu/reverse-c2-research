package httpapi

import (
	"net/http"

	"github.com/research/specter-reimplementation/internal/domain"
)

func (s *Server) networkRoutes() {
	s.mux.Handle("GET /api/NetworkIntelligence/GetAllHosts", s.protected(http.HandlerFunc(s.netHosts)))
	s.mux.Handle("GET /api/NetworkIntelligence/GetHostById", s.protected(http.HandlerFunc(s.netHost)))
	s.mux.Handle("POST /api/NetworkIntelligence/UpsertHost", s.requireRole("admin", http.HandlerFunc(s.upsertNetHost)))
	s.mux.Handle("POST /api/NetworkIntelligence/DeleteHost", s.requireRole("admin", http.HandlerFunc(s.deleteNetHost)))
	s.mux.Handle("GET /api/NetworkIntelligence/GetIpAddressesByHostId", s.protected(http.HandlerFunc(s.netHostIPs)))
	s.mux.Handle("POST /api/NetworkIntelligence/UpsertIpAddress", s.requireRole("admin", http.HandlerFunc(s.upsertNetHostIP)))
	s.mux.Handle("GET /api/NetworkIntelligence/GetOpenPortsByHostId", s.protected(http.HandlerFunc(s.netOpenPorts)))
	s.mux.Handle("POST /api/NetworkIntelligence/UpsertOpenPort", s.requireRole("admin", http.HandlerFunc(s.upsertNetOpenPort)))
	s.mux.Handle("GET /api/NetworkIntelligence/ResolveNetHostId", s.protected(http.HandlerFunc(s.resolveNetHostID)))
}

func (s *Server) netHosts(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.NetHosts(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}
func (s *Server) netHost(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.NetHost(r.Context(), r.URL.Query().Get("hostId"))
	if err != nil {
		writeCatalogError(w, err, "network host not found")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) upsertNetHost(w http.ResponseWriter, r *http.Request) {
	var request domain.UpsertHostRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	value, err := s.store.UpsertNetHost(r.Context(), request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "NetworkHostUpserted", value.HostID, value.Hostname)
	writeJSON(w, http.StatusOK, value)
}
func (s *Server) deleteNetHost(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("hostId")
	if err := s.store.DeleteNetHost(r.Context(), id); err != nil {
		writeCatalogError(w, err, "network host not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "NetworkHostDeleted", id, "host and related observations removed")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}
func (s *Server) netHostIPs(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.NetHostIPs(r.Context(), r.URL.Query().Get("hostId"))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}
func (s *Server) upsertNetHostIP(w http.ResponseWriter, r *http.Request) {
	var value domain.UpsertIPAddressRequest
	if !decodeJSON(w, r, &value) {
		return
	}
	if err := s.store.UpsertNetHostIP(r.Context(), value); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "NetworkIPAddressUpserted", value.HostID, value.IpAddress)
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}
func (s *Server) netOpenPorts(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.NetOpenPorts(r.Context(), r.URL.Query().Get("hostId"))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}
func (s *Server) upsertNetOpenPort(w http.ResponseWriter, r *http.Request) {
	var value domain.UpsertOpenPortRequest
	if !decodeJSON(w, r, &value) {
		return
	}
	if err := s.store.UpsertNetOpenPort(r.Context(), value); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "NetworkOpenPortUpserted", value.HostID, value.Protocol)
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}
func (s *Server) resolveNetHostID(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.ResolveNetHostID(r.Context(), r.URL.Query().Get("fqdn"), r.URL.Query().Get("ip"))
	if err != nil {
		writeCatalogError(w, err, "network host not found")
		return
	}
	writeJSON(w, http.StatusOK, value)
}
