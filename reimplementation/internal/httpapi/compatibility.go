package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/research/specter-reimplementation/internal/capability"
	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/store"
)

// compatibilityRoutes registers every remaining route in the independently
// described 7.0.0 contract. Metadata and fixture state are persistent. Routes
// that would create a remote shell, deliver phishing, or build/load a payload
// are deliberately present but fail closed through blockedCapability.
func (s *Server) compatibilityRoutes() {
	s.mux.Handle("GET /api/{controller}/Acme/acme-challenge/{token}", s.protected(http.HandlerFunc(s.acmeChallenge)))
	s.mux.Handle("GET /api/AcmeAccount", s.protected(s.resourceList("acme-accounts")))
	s.mux.Handle("PUT /api/AcmeAccount", s.requireRole("admin", s.resourcePut("acme-accounts", "email")))
	s.mux.Handle("DELETE /api/AcmeAccount/AcmeAccount/{email}", s.requireRole("admin", s.resourceDelete("acme-accounts", "email")))
	s.mux.Handle("PUT /api/AcmeAccount/AcmeAccount/registerdns", s.requireRole("admin", http.HandlerFunc(s.acmeDNSFixture)))
	s.mux.Handle("PUT /api/AcmeAccount/AcmeAccount/validate", s.requireRole("admin", http.HandlerFunc(s.acmeValidateFixture)))

	s.mux.Handle("PUT /api/Certificates/Certificates/import", s.requireRole("admin", s.blockedCapability("certificate-private-key-import", "store certificate metadata with PUT /api/Certificates")))
	s.mux.Handle("GET /api/Documentation/embedding-providers", s.protected(http.HandlerFunc(s.embeddingProviders)))
	s.mux.Handle("GET /api/Events/Events/GetAllEvents", s.protected(http.HandlerFunc(s.allEvents)))
	s.mux.Handle("GET /api/Settings/Settings/defaultsettings", s.protected(http.HandlerFunc(s.defaultSettings)))
	s.mux.Handle("GET /api/serverversion", s.protected(http.HandlerFunc(s.version)))

	s.mux.Handle("GET /api/Downloads", s.protected(s.resourceList("downloads")))
	s.mux.Handle("PUT /api/Downloads", s.requireRole("admin", s.resourcePut("downloads", "")))
	s.mux.Handle("DELETE /api/Downloads", s.requireRole("admin", s.resourceClear("downloads")))
	s.mux.Handle("DELETE /api/Downloads/Downloads/{identifier}", s.requireRole("admin", s.resourceDelete("downloads", "identifier")))

	s.mux.Handle("GET /api/EmailAccounts", s.protected(s.resourceList("email-accounts")))
	s.mux.Handle("POST /api/EmailAccounts", s.requireRole("admin", s.resourcePut("email-accounts", "")))
	s.mux.Handle("GET /api/EmailAccounts/{id}", s.protected(s.resourceGet("email-accounts", "id")))
	s.mux.Handle("PUT /api/EmailAccounts/{id}", s.requireRole("admin", s.resourcePut("email-accounts", "id")))
	s.mux.Handle("DELETE /api/EmailAccounts/{id}", s.requireRole("admin", s.resourceDelete("email-accounts", "id")))
	s.mux.Handle("GET /api/EmailAccounts/Providers", s.protected(http.HandlerFunc(s.emailProviders)))
	s.mux.Handle("POST /api/EmailAccounts/Register", s.requireRole("admin", s.resourcePut("email-accounts", "")))

	s.mux.Handle("POST /api/tasks/runscript", s.requireRole("admin", s.blockedCapability("agent-script-execution", "use the echo/inventory protocol simulator")))
	s.mux.Handle("GET /api/tasks/runscript", s.protected(s.resourceList("legacy-script-tasks")))

	loader := s.blockedCapability("loader-materialization", "inspect pipeline documentation and safe DAG output metadata")
	s.mux.Handle("GET /api/Loaders/{loaderId}/bundle/{platform}", s.protected(loader))
	s.mux.Handle("GET /api/Loaders/{loaderId}/flat-bundle/{platform}", s.protected(loader))
	s.mux.Handle("GET /api/Loaders/{loaderId}/deps/{platform}", s.protected(loader))
	s.mux.Handle("GET /api/Loaders/{loaderId}/assembly/{platform}/{assemblyName}", s.protected(loader))
	s.mux.Handle("GET /api/Loaders/{loaderId}/assembly-manifest/{platform}", s.protected(loader))

	s.networkCompatibilityRoutes()

	s.mux.Handle("GET /api/NotebookFiles/{notebookId}", s.protected(http.HandlerFunc(s.notebookFiles)))
	s.mux.Handle("GET /api/NotebookFiles/{notebookId}/{filename}", s.protected(http.HandlerFunc(s.notebookFile)))
	s.mux.Handle("DELETE /api/NotebookFiles/{notebookId}/{filename}", s.requireRole("admin", http.HandlerFunc(s.deleteNotebookFile)))

	s.mux.Handle("GET /api/PayloadPipeline/PayloadPipeline/{pipelineId}/run", s.requireRole("admin", s.blockedCapability("pipeline-get-execution", "POST the safe typed DAG to the run endpoint")))
	s.mux.Handle("POST /api/PayloadPipeline/PayloadPipeline/test", s.protected(http.HandlerFunc(s.testPipelineCompatibility)))
	s.mux.Handle("GET /api/PayloadPipeline/PayloadPipeline/documentation", s.protected(http.HandlerFunc(s.pipelineDocumentation)))
	s.mux.Handle("POST /api/PayloadPipeline/PayloadPipeline/clear", s.requireRole("admin", http.HandlerFunc(s.clearPipelineCache)))

	s.mux.Handle("GET /api/Payloads", s.protected(s.resourceList("payload-definitions")))
	s.mux.Handle("POST /api/Payloads", s.requireRole("admin", s.resourcePut("payload-definitions", "build")))
	s.mux.Handle("DELETE /api/Payloads", s.requireRole("admin", s.resourceDelete("payload-definitions", "build")))
	s.mux.Handle("GET /api/Payloads/Payloads/{build}", s.protected(s.resourceGet("payload-definitions", "build")))
	s.mux.Handle("POST /api/download", s.requireRole("admin", s.blockedCapability("implant-download", "use the protocol simulator or safe DAG result metadata")))

	registerCRUD(s, "/api/Personas", "personas", "id")
	s.mux.Handle("POST /api/Phishing/Send", s.requireRole("admin", s.blockedCapability("phishing-delivery", "use the SMTP fixture adapter preview")))
	s.mux.Handle("GET /api/Phishing/Targets", s.protected(http.HandlerFunc(s.phishingTargets)))

	s.mux.Handle("GET /api/PowerShellSessions", s.protected(s.resourceList("powershell-sessions")))
	s.mux.Handle("POST /api/PowerShellSessions", s.requireRole("admin", s.fixtureSessionPut("powershell-sessions")))
	s.mux.Handle("DELETE /api/PowerShellSessions/{sessionId}", s.requireRole("admin", s.resourceDelete("powershell-sessions", "sessionId")))
	s.mux.Handle("POST /api/PowerShellSessions/{sessionId}/run", s.requireRole("admin", s.blockedCapability("powershell-command", "use notebook metadata or the echo simulator")))

	s.mux.Handle("GET /api/Sessions", s.protected(s.resourceList("legacy-sessions")))
	s.mux.Handle("PUT /api/Sessions", s.requireRole("admin", s.resourcePut("legacy-sessions", "sessionId")))
	s.mux.Handle("DELETE /api/Sessions", s.requireRole("admin", s.resourceDelete("legacy-sessions", "sessionid")))
	s.mux.Handle("PUT /api/Sessions/SessionUpateNicknameAndNotes", s.requireRole("admin", s.resourcePut("legacy-sessions", "sessionId")))

	s.sshCompatibilityRoutes()
	s.torCompatibilityRoutes()
	s.tunnelCompatibilityRoutes()
	s.webDAVCompatibilityRoutes()
	s.winRMCompatibilityRoutes()
}

func registerCRUD(s *Server, base, kind, idParameter string) {
	s.mux.Handle("GET "+base, s.protected(s.resourceList(kind)))
	s.mux.Handle("POST "+base, s.requireRole("admin", s.resourcePut(kind, "")))
	s.mux.Handle("GET "+base+"/{"+idParameter+"}", s.protected(s.resourceGet(kind, idParameter)))
	s.mux.Handle("PUT "+base+"/{"+idParameter+"}", s.requireRole("admin", s.resourcePut(kind, idParameter)))
	s.mux.Handle("DELETE "+base+"/{"+idParameter+"}", s.requireRole("admin", s.resourceDelete(kind, idParameter)))
}

func (s *Server) sshCompatibilityRoutes() {
	s.mux.Handle("GET /api/SshSessions", s.protected(s.resourceList("ssh-sessions")))
	s.mux.Handle("POST /api/SshSessions", s.requireRole("admin", s.fixtureSessionPut("ssh-sessions")))
	s.mux.Handle("DELETE /api/SshSessions/{sessionId}", s.requireRole("admin", s.resourceDelete("ssh-sessions", "sessionId")))
	s.mux.Handle("GET /api/SshSessions/{sessionId}/tasks", s.protected(s.resourceList("ssh-tasks")))
	s.mux.Handle("POST /api/SshSessions/{sessionId}/command", s.requireRole("admin", s.blockedCapability("ssh-command", "use the SSH fixture adapter")))
	s.mux.Handle("POST /api/SshSessions/{sessionId}/upload", s.requireRole("admin", s.blockedCapability("ssh-upload", "use fixture metadata")))
	s.mux.Handle("GET /api/SshSessions/{sessionId}/download", s.requireRole("admin", s.blockedCapability("ssh-download", "use fixture metadata")))
	registerCollection(s, "/api/SshTunnels", "ssh-tunnels", "id", true)
}

func (s *Server) torCompatibilityRoutes() {
	s.mux.Handle("POST /api/Tor/start", s.requireRole("admin", http.HandlerFunc(s.startTorFixture)))
	s.mux.Handle("POST /api/Tor/stop", s.requireRole("admin", http.HandlerFunc(s.stopTorFixture)))
	s.mux.Handle("GET /api/Tor/status", s.protected(http.HandlerFunc(s.torStatus)))
	s.mux.Handle("GET /api/Tor/exitnodes", s.protected(http.HandlerFunc(s.torExitNodes)))
	s.mux.Handle("POST /api/Tor/exitcountry", s.requireRole("admin", s.resourcePut("tor-settings", "exitCountry")))
	s.mux.Handle("POST /api/Tor/exitnode", s.requireRole("admin", s.resourcePut("tor-settings", "exitNode")))
	s.mux.Handle("DELETE /api/Tor/exit", s.requireRole("admin", s.resourceClear("tor-settings")))
	s.mux.Handle("POST /api/Tor/rotate", s.requireRole("admin", http.HandlerFunc(s.rotateTorFixture)))
	registerCollection(s, "/api/TorTunnels", "tor-tunnels", "id", true)
}

func (s *Server) tunnelCompatibilityRoutes() {
	s.mux.Handle("GET /api/Tunnels", s.protected(s.resourceList("tunnels")))
	s.mux.Handle("POST /api/Tunnels", s.requireRole("admin", s.fixtureSessionPut("tunnels")))
	s.mux.Handle("DELETE /api/Tunnels/Tunnels/{lport}", s.requireRole("admin", s.resourceDelete("tunnels", "lport")))
}

func (s *Server) webDAVCompatibilityRoutes() {
	s.mux.Handle("GET /api/WebDavFiles", s.protected(s.resourceList("webdav-files")))
	s.mux.Handle("POST /api/WebDavFiles", s.requireRole("admin", s.resourcePut("webdav-files", "path")))
	s.mux.Handle("DELETE /api/WebDavFiles", s.requireRole("admin", http.HandlerFunc(s.deleteWebDAVFile)))
	s.mux.Handle("GET /api/WebDavFiles/tree", s.protected(s.resourceList("webdav-files")))
	s.mux.Handle("POST /api/WebDavFiles/directory", s.requireRole("admin", s.resourcePut("webdav-directories", "path")))
}

func (s *Server) winRMCompatibilityRoutes() {
	s.mux.Handle("GET /api/WinRmSessions", s.protected(s.resourceList("winrm-sessions")))
	s.mux.Handle("POST /api/WinRmSessions", s.requireRole("admin", s.fixtureSessionPut("winrm-sessions")))
	s.mux.Handle("DELETE /api/WinRmSessions/{sessionId}", s.requireRole("admin", s.resourceDelete("winrm-sessions", "sessionId")))
	s.mux.Handle("GET /api/WinRmSessions/{sessionId}/tasks", s.protected(s.resourceList("winrm-tasks")))
	blockedCommand := s.blockedCapability("winrm-command", "use the WinRM fixture adapter")
	s.mux.Handle("POST /api/WinRmSessions/{sessionId}/command", s.requireRole("admin", blockedCommand))
	s.mux.Handle("POST /api/WinRmSessions/{sessionId}/upload", s.requireRole("admin", s.blockedCapability("winrm-upload", "use fixture metadata")))
	s.mux.Handle("GET /api/WinRmSessions/{sessionId}/download", s.requireRole("admin", s.blockedCapability("winrm-download", "use fixture metadata")))
	s.mux.Handle("GET /api/WinRmSessions/{sessionId}/services", s.protected(s.resourceList("winrm-services")))
	load := s.blockedCapability("winrm-in-memory-load", "use the protocol simulator and safe DAG")
	s.mux.Handle("POST /api/WinRmSessions/{sessionId}/load/pscript", s.requireRole("admin", load))
	s.mux.Handle("POST /api/WinRmSessions/{sessionId}/load/dll", s.requireRole("admin", load))
	s.mux.Handle("POST /api/WinRmSessions/{sessionId}/load/assembly", s.requireRole("admin", load))
	s.mux.Handle("POST /api/WinRmSessions/{sessionId}/load/amsi", s.requireRole("admin", load))
	s.mux.Handle("POST /api/WinRmSessions/{sessionId}/load/etw", s.requireRole("admin", load))
}

func registerCollection(s *Server, base, kind, idParameter string, fixture bool) {
	s.mux.Handle("GET "+base, s.protected(s.resourceList(kind)))
	put := s.resourcePut(kind, "")
	if fixture {
		put = s.fixtureSessionPut(kind)
	}
	s.mux.Handle("POST "+base, s.requireRole("admin", put))
	s.mux.Handle("DELETE "+base+"/{"+idParameter+"}", s.requireRole("admin", s.resourceDelete(kind, idParameter)))
}

func (s *Server) resourceList(kind string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values, err := s.store.Resources(r.Context(), kind)
		if err != nil {
			s.internalError(w, err)
			return
		}
		result := make([]any, 0, len(values))
		for _, value := range values {
			result = append(result, resourceRepresentation(value))
		}
		writeJSON(w, http.StatusOK, result)
	})
}

func (s *Server) resourceGet(kind, parameter string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value, err := s.findResource(r, kind, requestKey(r, parameter))
		if err != nil {
			writeError(w, http.StatusNotFound, "resource not found")
			return
		}
		writeJSON(w, http.StatusOK, resourceRepresentation(value))
	})
}

func (s *Server) resourcePut(kind, parameter string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, raw, ok := decodeCompatibilityBody(w, r)
		if !ok {
			return
		}
		key := requestKey(r, parameter)
		if key == "" {
			key = bodyKey(body)
		}
		if key == "" {
			key = newID(strings.TrimSuffix(kind, "s"))
		}
		name := bodyName(body)
		if name == "" {
			name = key
		}
		value, err := s.store.PutResource(r.Context(), domain.Resource{Kind: kind, Key: key, Name: name, Status: "ready", Spec: raw})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.audit(r.Context(), principal(r).Username, "CompatibilityResourceUpdated", kind+":"+key, "persistent metadata")
		writeJSON(w, http.StatusOK, resourceRepresentation(value))
	})
}

func (s *Server) fixtureSessionPut(kind string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, raw, ok := decodeCompatibilityBody(w, r)
		if !ok {
			return
		}
		key := bodyKey(body)
		if key == "" {
			key = newID("fixture")
		}
		name := bodyName(body)
		if name == "" {
			name = key
		}
		value, err := s.store.PutResource(r.Context(), domain.Resource{Kind: kind, Key: key, Name: name, Status: "fixture", Description: "configuration only; no connection or listener opened", Spec: raw})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.audit(r.Context(), principal(r).Username, "FixtureResourceCreated", kind+":"+key, "no network side effect")
		writeJSON(w, http.StatusOK, resourceRepresentation(value))
	})
}

func (s *Server) resourceDelete(kind, parameter string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := requestKey(r, parameter)
		if key == "" {
			writeError(w, http.StatusBadRequest, parameter+" is required")
			return
		}
		if err := s.store.DeleteResource(r.Context(), kind, key); err != nil {
			writeError(w, http.StatusNotFound, "resource not found")
			return
		}
		s.audit(r.Context(), principal(r).Username, "CompatibilityResourceDeleted", kind+":"+key, "metadata removed")
		writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
	})
}

func (s *Server) resourceClear(kind string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values, err := s.store.Resources(r.Context(), kind)
		if err != nil {
			s.internalError(w, err)
			return
		}
		for _, value := range values {
			if err := s.store.DeleteResource(r.Context(), kind, value.Key); err != nil {
				s.internalError(w, err)
				return
			}
		}
		s.audit(r.Context(), principal(r).Username, "CompatibilityResourcesCleared", kind, strconv.Itoa(len(values)))
		writeJSON(w, http.StatusOK, map[string]int{"deleted": len(values)})
	})
}

func (s *Server) findResource(r *http.Request, kind, key string) (domain.Resource, error) {
	values, err := s.store.Resources(r.Context(), kind)
	if err != nil {
		return domain.Resource{}, err
	}
	for _, value := range values {
		if value.Key == key {
			return value, nil
		}
	}
	return domain.Resource{}, store.ErrNotFound
}

func requestKey(r *http.Request, parameter string) string {
	if parameter == "" {
		return ""
	}
	if value := strings.TrimSpace(r.PathValue(parameter)); value != "" {
		return value
	}
	return strings.TrimSpace(r.URL.Query().Get(parameter))
}

func decodeCompatibilityBody(w http.ResponseWriter, r *http.Request) (any, json.RawMessage, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	var body any
	decoder := json.NewDecoder(r.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return nil, nil, false
	}
	body = sanitizeCompatibilityBody(body)
	raw, err := json.Marshal(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return nil, nil, false
	}
	return body, raw, true
}

func bodyKey(body any) string {
	value, ok := body.(map[string]any)
	if !ok {
		return ""
	}
	for _, field := range []string{"id", "identifier", "sessionId", "pipelineId", "credentialId", "serviceId", "userId", "fileId", "hostId", "build", "email", "emailAddress", "username", "name", "path", "lport", "exitCountry", "exitNode"} {
		if candidate := scalarString(value[field]); candidate != "" {
			return candidate
		}
	}
	return ""
}

func sanitizeCompatibilityBody(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, child := range typed {
			normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "_", ""), "-", ""))
			if strings.Contains(normalized, "password") || strings.Contains(normalized, "secret") ||
				strings.Contains(normalized, "privatekey") || normalized == "accesstoken" || normalized == "refreshtoken" {
				result[key] = "[redacted]"
				continue
			}
			result[key] = sanitizeCompatibilityBody(child)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, child := range typed {
			result[index] = sanitizeCompatibilityBody(child)
		}
		return result
	default:
		return typed
	}
}

func bodyName(body any) string {
	value, ok := body.(map[string]any)
	if !ok {
		return ""
	}
	for _, field := range []string{"name", "displayName", "emailAddress", "email", "username", "hostname", "path"} {
		if candidate := scalarString(value[field]); candidate != "" {
			return candidate
		}
	}
	return ""
}

func scalarString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case json.Number:
		return typed.String()
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return ""
	}
}

func resourceRepresentation(value domain.Resource) any {
	var body any
	if json.Unmarshal(value.Spec, &body) != nil {
		return map[string]any{"id": value.Key, "name": value.Name, "status": value.Status}
	}
	if object, ok := body.(map[string]any); ok {
		if _, exists := object["id"]; !exists {
			object["id"] = value.Key
		}
		if _, exists := object["status"]; !exists {
			object["status"] = value.Status
		}
		return object
	}
	return body
}

func (s *Server) blockedCapability(id, alternative string) http.Handler {
	definition, exists := capability.Get(id)
	if !exists {
		panic("unknown blocked capability: " + id)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		event, err := s.store.AddEvent(r.Context(), domain.Event{
			Identifier: newID("blocked"), Type: "CapabilityBlocked", Source: principal(r).Username,
			Outcome: "Blocked", System: "clean-room-boundary", Message: definition.Title,
			Details: "intentionally not reproduced; " + alternative,
		})
		if err == nil {
			s.hub.Publish(event)
		}
		writeJSON(w, http.StatusNotImplemented, map[string]string{
			"code": "CAPABILITY_NOT_IMPLEMENTED", "capabilityId": definition.ID, "state": string(definition.State),
			"error": definition.Title + " is currently unavailable", "safeAlternative": alternative,
		})
	})
}

func (s *Server) acmeChallenge(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "no fixture ACME challenge is active")
}

func (s *Server) acmeDNSFixture(w http.ResponseWriter, r *http.Request) {
	body, _, ok := decodeCompatibilityBody(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"fixture": true, "authorization": body, "externalRequest": false})
}

func (s *Server) acmeValidateFixture(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := decodeCompatibilityBody(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true, "fixture": true, "certificateIssued": false})
}

func (s *Server) embeddingProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{{"name": "postgres-trigram", "available": true, "external": false}})
}

func (s *Server) allEvents(w http.ResponseWriter, r *http.Request) {
	events, err := s.store.LatestEvents(r.Context(), nil, 1000)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) defaultSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []domain.Setting{{Key: "events.replayLimit", Value: "1000"}, {Key: "fixtures.enabled", Value: strconv.FormatBool(s.cfg.FixturesEnabled)}})
}

func (s *Server) emailProviders(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []string{"fixture", "smtp"})
}

func (s *Server) phishingTargets(w http.ResponseWriter, r *http.Request) {
	events, err := s.store.LatestEvents(r.Context(), nil, 200)
	if err != nil {
		s.internalError(w, err)
		return
	}
	result := make([]domain.Event, 0)
	for _, event := range events {
		if strings.Contains(strings.ToLower(event.Type), "phish") {
			result = append(result, event)
		}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) deleteWebDAVFile(w http.ResponseWriter, r *http.Request) {
	body, _, ok := decodeCompatibilityBody(w, r)
	if !ok {
		return
	}
	key, ok := body.(string)
	if !ok || strings.TrimSpace(key) == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}
	if err := s.store.DeleteResource(r.Context(), "webdav-files", key); err != nil {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) startTorFixture(w http.ResponseWriter, r *http.Request) {
	body, raw, ok := decodeCompatibilityBody(w, r)
	if !ok {
		return
	}
	name := bodyName(body)
	if name == "" {
		name = "Tor fixture"
	}
	value, err := s.store.PutResource(r.Context(), domain.Resource{Kind: "tor-runtime", Key: "singleton", Name: name, Status: "fixture-running", Description: "no Tor process started", Spec: raw})
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, resourceRepresentation(value))
}

func (s *Server) stopTorFixture(w http.ResponseWriter, r *http.Request) {
	err := s.store.DeleteResource(r.Context(), "tor-runtime", "singleton")
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "stopped", "fixture": true})
}

func (s *Server) torStatus(w http.ResponseWriter, r *http.Request) {
	value, err := s.findResource(r, "tor-runtime", "singleton")
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"status": "stopped", "fixture": true})
		return
	}
	writeJSON(w, http.StatusOK, resourceRepresentation(value))
}

func (s *Server) torExitNodes(w http.ResponseWriter, r *http.Request) {
	country := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("countryCode")))
	writeJSON(w, http.StatusOK, []map[string]any{{"fingerprint": "FIXTURE0000000000000000000000000000000000", "countryCode": country, "fixture": true}})
}

func (s *Server) rotateTorFixture(w http.ResponseWriter, r *http.Request) {
	s.audit(r.Context(), principal(r).Username, "TorFixtureRotated", "singleton", "no Tor process or external network used")
	writeJSON(w, http.StatusOK, map[string]any{"rotated": true, "fixture": true})
}

func (s *Server) notebookFiles(w http.ResponseWriter, r *http.Request) {
	kind := "notebook-files:" + r.PathValue("notebookId")
	s.resourceList(kind).ServeHTTP(w, r)
}

func (s *Server) notebookFile(w http.ResponseWriter, r *http.Request) {
	kind := "notebook-files:" + r.PathValue("notebookId")
	s.resourceGet(kind, "filename").ServeHTTP(w, r)
}

func (s *Server) deleteNotebookFile(w http.ResponseWriter, r *http.Request) {
	kind := "notebook-files:" + r.PathValue("notebookId")
	s.resourceDelete(kind, "filename").ServeHTTP(w, r)
}

func (s *Server) testPipelineCompatibility(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Definition json.RawMessage `json:"definition"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if len(request.Definition) == 0 {
		request.Definition = json.RawMessage(`{}`)
	}
	writeJSON(w, http.StatusOK, map[string]any{"safeTransformsOnly": true, "externalExecution": false, "definitionBytes": len(request.Definition)})
}

func (s *Server) pipelineDocumentation(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []map[string]any{
		{"name": "input", "description": "bounded input value"},
		{"name": "literal", "description": "static text"},
		{"name": "concat", "description": "concatenate values"},
		{"name": "base64", "description": "encode value"},
		{"name": "base64decode", "description": "decode value"},
		{"name": "sha256", "description": "hash value"},
		{"name": "upper", "description": "uppercase value"},
		{"name": "lower", "description": "lowercase value"},
	})
}

func (s *Server) clearPipelineCache(w http.ResponseWriter, r *http.Request) {
	s.audit(r.Context(), principal(r).Username, "PipelineCacheCleared", "safe-dag-worker", "worker is one-shot; no executable cache exists")
	writeJSON(w, http.StatusOK, map[string]any{"cleared": true, "entries": 0})
}
