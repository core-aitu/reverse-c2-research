package httpapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/gateway"
	"github.com/research/specter-reimplementation/internal/store"
)

func (s *Server) agentGatewayRoutes() {
	s.mux.HandleFunc("POST /api/agent/v1/enroll", s.agentEnroll)
	s.mux.HandleFunc("POST /api/agent/v1/checkin", s.agentCheckinV1)
	s.mux.HandleFunc("POST /api/agent/v1/ack", s.agentAckV1)
	s.mux.HandleFunc("POST /api/agent/v1/results", s.agentResultV1)
	s.mux.HandleFunc("POST /api/agent/v1/rotate", s.agentRotateV1)
	s.mux.HandleFunc("POST /api/agent/v1/task-status", s.agentTaskStatusV1)
	s.mux.HandleFunc("POST /api/agent/v1/task-output", s.agentTaskOutputV1)
	s.mux.HandleFunc("POST /api/agent/v1/files/begin", s.agentFileBeginV1)
	s.mux.HandleFunc("PUT /api/agent/v1/files/{id}/chunks/{index}", s.agentFileChunkV1)
	s.mux.HandleFunc("POST /api/agent/v1/files/{id}/complete", s.agentFileCompleteV1)
	s.mux.Handle("POST /api/gateway/enrollment-tokens", s.requireRole("admin", http.HandlerFunc(s.createEnrollmentToken)))
	s.mux.Handle("GET /api/gateway/enrollment-tokens", s.requireRole("admin", http.HandlerFunc(s.enrollmentTokens)))
	s.mux.Handle("GET /api/gateway/agents", s.protected(http.HandlerFunc(s.gatewayAgents)))
	s.mux.Handle("GET /api/gateway/listener", s.protected(http.HandlerFunc(s.gatewayListener)))
	s.mux.Handle("GET /api/gateway/agents/{id}/tasks", s.protected(http.HandlerFunc(s.gatewayAgentTasks)))
	s.mux.Handle("GET /api/gateway/agents/{id}/tasks/{taskID}", s.requireRole("admin", http.HandlerFunc(s.gatewayTaskDetail)))
	s.mux.Handle("POST /api/gateway/agents/{id}/tasks", s.requireRole("admin", http.HandlerFunc(s.createGatewayTask)))
	s.mux.Handle("POST /api/gateway/agents/{id}/tasks/{taskID}/cancel", s.requireRole("admin", http.HandlerFunc(s.cancelGatewayTask)))
	s.mux.Handle("POST /api/gateway/agents/{id}/tasks/{taskID}/output-ticket", s.requireRole("admin", http.HandlerFunc(s.createTaskOutputTicket)))
	s.mux.HandleFunc("GET /api/gateway/task-output", s.taskOutputSocket)
	s.mux.Handle("POST /api/gateway/agents/{id}/{action}", s.requireRole("admin", http.HandlerFunc(s.manageAgentIdentity)))
}

func (s *Server) gatewayListener(w http.ResponseWriter, _ *http.Request) {
	tlsConfigured := s.cfg.AgentTLSCert != "" && s.cfg.AgentTLSKey != ""
	status := "offline"
	if tlsConfigured && s.cfg.AgentPublicURL != "" {
		status = "online"
	} else if tlsConfigured || s.cfg.AgentPublicURL != "" {
		status = "degraded"
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"endpoint": s.cfg.AgentPublicURL, "address": s.cfg.AgentAddr,
		"tlsConfigured": tlsConfigured, "ready": status == "online", "status": status,
	})
}

func (s *Server) agentTaskOutputV1(w http.ResponseWriter, r *http.Request) {
	frame, ok := s.protocolRequest(w, r)
	if !ok {
		return
	}
	agent, ok := s.agentPrincipal(w, r)
	if !ok {
		return
	}
	output, err := gateway.DecodeTaskOutputChunk(frame)
	if err != nil || !s.verifyAgentRequest(w, r, agent.ID, frame, output.Sequence) {
		if err != nil {
			writeError(w, http.StatusBadRequest, "malformed task output frame")
		}
		return
	}
	value := domain.TaskOutputChunk{
		TaskID: output.TaskID, Stream: output.Stream, Index: int(output.Index), Content: output.Data, EOF: output.EOF, Truncated: output.Truncated,
	}
	if err = s.store.AdvanceAgentSequence(r.Context(), agent.ID, output.Sequence, false); err != nil {
		_, matched, matchErr := s.store.MatchTaskOutputChunk(r.Context(), agent.ID, value)
		if matchErr != nil || !matched {
			writeError(w, http.StatusConflict, "replayed task output sequence")
			return
		}
		writeFrame(w, http.StatusOK, mustAck(gateway.Ack{Sequence: output.Sequence}))
		return
	}
	stored, duplicate, err := s.store.StoreTaskOutputChunk(r.Context(), agent.ID, value)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	if !duplicate {
		s.publishTaskOutput(stored)
	}
	writeFrame(w, http.StatusOK, mustAck(gateway.Ack{Sequence: output.Sequence}))
}

func (s *Server) protocolRequest(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	if r.Header.Get("X-Specter-Protocol-Version") != strconv.Itoa(gateway.Version) {
		writeError(w, http.StatusUpgradeRequired, "unsupported agent protocol version")
		return nil, false
	}
	if media := requestMediaType(r); media != "application/vnd.specter.gateway+gzip" {
		writeError(w, http.StatusUnsupportedMediaType, "agent gateway requires gzip binary media type")
		return nil, false
	}
	r.Body = http.MaxBytesReader(w, r.Body, gateway.MaxFrameSize)
	content, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, "agent frame exceeds limit")
		return nil, false
	}
	return content, true
}

func (s *Server) agentEnroll(w http.ResponseWriter, r *http.Request) {
	frame, ok := s.protocolRequest(w, r)
	if !ok {
		return
	}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Enrollment ")
	if token == "" {
		writeError(w, http.StatusUnauthorized, "enrollment token required")
		return
	}
	registration, err := gateway.DecodeRegistration(frame)
	if err != nil {
		writeError(w, http.StatusBadRequest, "malformed registration frame")
		return
	}
	if registration.AgentID == "" {
		registration.AgentID = newID("agent")
	}
	if len(registration.AgentID) > 128 || len(registration.Build) > 128 {
		writeError(w, http.StatusBadRequest, "registration identifiers exceed policy")
		return
	}
	capabilities, err := parseAdvertisedCapabilities(r.Header.Get("X-Specter-Capabilities"))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	identity, err := secureToken()
	if err != nil {
		s.internalError(w, err)
		return
	}
	metadata, _ := json.Marshal(map[string]any{"build": registration.Build, "protocolVersion": gateway.Version, "capabilities": capabilities, "architecture": registration.Architecture, "os": registration.OS, "intervalMs": registration.Interval.Milliseconds()})
	agent, err := s.store.EnrollAgent(r.Context(), token, registration.AgentID, registration.AgentID, identity, metadata)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "enrollment token invalid, expired, or used")
		return
	}
	s.audit(r.Context(), agent.ID, "AgentEnrolled", agent.ID, "protocol v1")
	w.Header().Set("X-Specter-Agent-ID", agent.ID)
	w.Header().Set("X-Specter-Identity", identity)
	writeFrame(w, http.StatusCreated, mustAck(gateway.Ack{}))
}

func parseAdvertisedCapabilities(raw string) ([]string, error) {
	allowed := map[string]bool{
		"echo": true, "system-info": true, "sleep": true, "exec": true, "file-download": true,
		"shell:sh": true, "shell:cmd": true, "shell:powershell": true,
	}
	seen := map[string]bool{}
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if !allowed[value] {
			return nil, errors.New("agent advertised an unsupported capability")
		}
		seen[value] = true
	}
	for _, required := range []string{"echo", "system-info", "sleep"} {
		if !seen[required] {
			return nil, errors.New("agent omitted a required v1 capability")
		}
	}
	values := make([]string, 0, len(seen))
	for value := range seen {
		values = append(values, value)
	}
	sort.Strings(values)
	return values, nil
}

func (s *Server) agentCheckinV1(w http.ResponseWriter, r *http.Request) {
	frame, ok := s.protocolRequest(w, r)
	if !ok {
		return
	}
	agent, ok := s.agentPrincipal(w, r)
	if !ok {
		return
	}
	checkin, err := gateway.DecodeCheckin(frame)
	clockDelta := time.Since(checkin.Timestamp)
	if clockDelta < 0 {
		clockDelta = -clockDelta
	}
	if err != nil || checkin.AgentID != agent.ID || clockDelta > gateway.ClockSkewTolerance {
		writeError(w, http.StatusBadRequest, "malformed or mismatched check-in")
		return
	}
	if !s.verifyAgentRequest(w, r, agent.ID, frame, checkin.Sequence) {
		return
	}
	if !s.acquireAgent(agent.ID) {
		writeError(w, http.StatusTooManyRequests, "concurrent or overly frequent check-in rejected")
		return
	}
	defer s.releaseAgent(agent.ID)
	if err = s.store.AdvanceAgentCheckin(r.Context(), agent.ID, checkin.Sequence, checkin.Cursor); err != nil {
		writeError(w, http.StatusConflict, "replayed check-in sequence")
		return
	}
	tasks, err := s.store.LeaseGatewayTasks(r.Context(), agent.ID, checkin.Cursor, checkin.Limit, 30*time.Second)
	if err != nil {
		s.internalError(w, err)
		return
	}
	batch := gateway.TaskBatch{Sequence: checkin.Sequence, Cursor: checkin.Cursor, Tasks: make([]gateway.Task, 0, len(tasks))}
	for _, task := range tasks {
		batch.Tasks = append(batch.Tasks, gateway.Task{Version: task.Version, ID: task.ID, IdempotencyKey: task.IdempotencyKey, Cursor: task.Cursor, Kind: task.Kind, Payload: task.Payload, Deadline: task.Deadline, Timeout: time.Duration(task.TimeoutMS) * time.Millisecond, CancelRequested: task.CancelRequestedAt != nil})
		if task.Cursor > batch.Cursor {
			batch.Cursor = task.Cursor
		}
		s.audit(r.Context(), agent.ID, "AgentTaskDispatched", task.ID, agent.ID+"; dispatch "+strconv.Itoa(task.DispatchCount))
	}
	encoded, err := gateway.EncodeTaskBatch(batch)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r.Context(), agent.ID, "AgentCheckin", agent.ID, strconv.Itoa(len(tasks))+" tasks")
	writeFrame(w, http.StatusOK, encoded)
}

func (s *Server) agentAckV1(w http.ResponseWriter, r *http.Request) {
	frame, ok := s.protocolRequest(w, r)
	if !ok {
		return
	}
	agent, ok := s.agentPrincipal(w, r)
	if !ok {
		return
	}
	receipt, err := gateway.DecodeReceipt(frame)
	if err != nil {
		writeError(w, http.StatusBadRequest, "malformed receipt")
		return
	}
	if !s.verifyAgentRequest(w, r, agent.ID, frame, receipt.Sequence) {
		return
	}
	if err = s.store.AdvanceAgentSequence(r.Context(), agent.ID, receipt.Sequence, false); err != nil {
		writeError(w, http.StatusConflict, "replayed receipt sequence")
		return
	}
	if err = s.store.AckGatewayTask(r.Context(), agent.ID, receipt.TaskID, receipt.Cursor); err != nil {
		writeError(w, http.StatusNotFound, "leased task not found")
		return
	}
	s.audit(r.Context(), agent.ID, "AgentTaskAcknowledged", receipt.TaskID, agent.ID)
	s.audit(r.Context(), agent.ID, "AgentTaskStarted", receipt.TaskID, agent.ID)
	writeFrame(w, http.StatusOK, mustAck(gateway.Ack{Sequence: receipt.Sequence, Cursor: receipt.Cursor}))
}

func (s *Server) agentResultV1(w http.ResponseWriter, r *http.Request) {
	frame, ok := s.protocolRequest(w, r)
	if !ok {
		return
	}
	agent, ok := s.agentPrincipal(w, r)
	if !ok {
		return
	}
	result, err := gateway.DecodeResult(frame)
	if err != nil {
		writeError(w, http.StatusBadRequest, "malformed result frame")
		return
	}
	if !s.verifyAgentRequest(w, r, agent.ID, frame, result.Sequence) {
		return
	}
	payload := json.RawMessage(result.Payload)
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if !json.Valid(payload) {
		writeError(w, http.StatusBadRequest, "result payload must be JSON")
		return
	}
	task, err := s.store.GatewayTask(r.Context(), agent.ID, result.TaskID)
	if err != nil {
		writeError(w, http.StatusNotFound, "leased task not found")
		return
	}
	if task.Kind == "exec" || task.Kind == "shell" {
		outputs, finalizeErr := s.store.FinalizeTaskOutput(r.Context(), task.ID)
		if finalizeErr != nil {
			writeError(w, http.StatusServiceUnavailable, "task output archival failed")
			return
		}
		if len(outputs) > 0 {
			var value map[string]any
			if json.Unmarshal(payload, &value) == nil {
				value["outputs"] = outputs
				payload, _ = json.Marshal(value)
			}
		}
	}
	code, message, retryable := "", "", false
	if result.Error != nil {
		code, message, retryable = result.Error.Code, result.Error.Message, result.Error.Retryable
	}
	idempotent, err := s.store.CompleteGatewayTask(r.Context(), agent.ID, result.TaskID, result.SubmissionID, result.Sequence, result.Success, payload, code, message, retryable)
	if err != nil {
		writeError(w, http.StatusNotFound, "leased task not found or sequence replayed")
		return
	}
	detail := "accepted"
	if idempotent {
		detail = "idempotent replay"
	}
	s.audit(r.Context(), agent.ID, "AgentTaskResult", result.TaskID, agent.ID+"; "+detail)
	terminalEvent := "AgentTaskFailed"
	if result.Success {
		terminalEvent = "AgentTaskCompleted"
	} else if code == "cancelled" {
		terminalEvent = "AgentTaskCancelled"
	}
	s.audit(r.Context(), agent.ID, terminalEvent, result.TaskID, agent.ID+"; "+detail)
	writeFrame(w, http.StatusOK, mustAck(gateway.Ack{Sequence: result.Sequence}))
}

func (s *Server) agentRotateV1(w http.ResponseWriter, r *http.Request) {
	frame, ok := s.protocolRequest(w, r)
	if !ok {
		return
	}
	agent, ok := s.agentPrincipal(w, r)
	if !ok {
		return
	}
	request, err := gateway.DecodeAck(frame)
	if err != nil || !s.verifyAgentRequest(w, r, agent.ID, frame, request.Sequence) {
		return
	}
	if err = s.store.AdvanceAgentSequence(r.Context(), agent.ID, request.Sequence, false); err != nil {
		writeError(w, http.StatusConflict, "replayed rotation sequence")
		return
	}
	identity, err := secureToken()
	if err != nil {
		s.internalError(w, err)
		return
	}
	if err = s.store.RotateAgentIdentity(r.Context(), agent.ID, identity); err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r.Context(), agent.ID, "AgentIdentityRotated", agent.ID, "self-service rotation with five-minute overlap")
	w.Header().Set("X-Specter-Identity", identity)
	writeFrame(w, http.StatusOK, mustAck(gateway.Ack{Sequence: request.Sequence, Cursor: request.Cursor}))
}

func (s *Server) agentTaskStatusV1(w http.ResponseWriter, r *http.Request) {
	frame, ok := s.protocolRequest(w, r)
	if !ok {
		return
	}
	agent, ok := s.agentPrincipal(w, r)
	if !ok {
		return
	}
	receipt, err := gateway.DecodeReceipt(frame)
	if err != nil || !s.verifyAgentRequest(w, r, agent.ID, frame, receipt.Sequence) {
		return
	}
	if err = s.store.AdvanceAgentSequence(r.Context(), agent.ID, receipt.Sequence, false); err != nil {
		writeError(w, http.StatusConflict, "replayed task-status sequence")
		return
	}
	cancelRequested, err := s.store.GatewayTaskCancelRequested(r.Context(), agent.ID, receipt.TaskID)
	if err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	w.Header().Set("X-Specter-Cancel-Requested", strconv.FormatBool(cancelRequested))
	writeFrame(w, http.StatusOK, mustAck(gateway.Ack{Sequence: receipt.Sequence, Cursor: receipt.Cursor}))
}

func (s *Server) verifyAgentRequest(w http.ResponseWriter, r *http.Request, agentID string, frame []byte, sequence uint64) bool {
	identity := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	timestamp, nonce, signature := r.Header.Get("X-Specter-Timestamp"), r.Header.Get("X-Specter-Nonce"), r.Header.Get("X-Specter-Signature")
	headerSequence, err := strconv.ParseUint(r.Header.Get("X-Specter-Sequence"), 10, 64)
	if err != nil || headerSequence != sequence || !gateway.VerifyRequestSignature(identity, r.Method, r.URL.Path, timestamp, nonce, sequence, frame, signature, time.Now().UTC()) {
		writeError(w, http.StatusUnauthorized, "invalid agent request signature")
		return false
	}
	if err = s.store.UseAgentNonce(r.Context(), agentID, nonce, sequence, time.Now().UTC().Add(-2*gateway.ClockSkewTolerance)); err != nil {
		writeError(w, http.StatusConflict, "replayed agent request nonce")
		return false
	}
	return true
}

func (s *Server) agentPrincipal(w http.ResponseWriter, r *http.Request) (domain.AgentIdentity, bool) {
	identity := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if identity == "" {
		writeError(w, http.StatusUnauthorized, "agent identity required")
		return domain.AgentIdentity{}, false
	}
	agent, err := s.store.AuthenticateAgent(r.Context(), identity)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "agent identity invalid")
		return domain.AgentIdentity{}, false
	}
	return agent, true
}

func (s *Server) acquireAgent(id string) bool {
	s.agentMu.Lock()
	defer s.agentMu.Unlock()
	now := time.Now()
	if s.agentActive[id] || now.Sub(s.agentLast[id]) < 100*time.Millisecond {
		return false
	}
	s.agentActive[id] = true
	s.agentLast[id] = now
	return true
}
func (s *Server) releaseAgent(id string) {
	s.agentMu.Lock()
	delete(s.agentActive, id)
	s.agentMu.Unlock()
}

func (s *Server) createEnrollmentToken(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TTLSeconds int64 `json:"ttlSeconds"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if request.TTLSeconds < 60 || request.TTLSeconds > 86400 {
		writeError(w, http.StatusBadRequest, "TTL must be between 60 and 86400 seconds")
		return
	}
	token, err := secureToken()
	if err != nil {
		s.internalError(w, err)
		return
	}
	id := newID("enroll")
	expires := time.Now().UTC().Add(time.Duration(request.TTLSeconds) * time.Second)
	if err = s.store.CreateAgentEnrollment(r.Context(), id, token, principal(r).Username, expires); err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r.Context(), principal(r).Username, "AgentEnrollmentTokenCreated", id, expires.Format(time.RFC3339))
	writeJSON(w, http.StatusCreated, map[string]any{"id": id, "token": token, "expiresAt": expires})
}

func (s *Server) enrollmentTokens(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.AgentEnrollmentTokens(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) gatewayAgents(w http.ResponseWriter, r *http.Request) {
	identities, err := s.store.AgentIdentities(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	now := time.Now().UTC()
	values := make([]domain.AgentOperationalView, 0, len(identities))
	for _, identity := range identities {
		metadata := struct {
			Build           string   `json:"build"`
			ProtocolVersion int      `json:"protocolVersion"`
			Capabilities    []string `json:"capabilities"`
			Architecture    string   `json:"architecture"`
			OS              string   `json:"os"`
			IntervalMS      int64    `json:"intervalMs"`
		}{ProtocolVersion: gateway.Version, Capabilities: []string{"echo", "system-info", "sleep"}, IntervalMS: 5000}
		_ = json.Unmarshal(identity.Metadata, &metadata)
		if metadata.IntervalMS < 100 {
			metadata.IntervalMS = 100
		}
		interval := time.Duration(metadata.IntervalMS) * time.Millisecond
		next := identity.LastCheckin.Add(interval)
		grace := 2 * interval
		if grace < 5*time.Second {
			grace = 5 * time.Second
		}
		health := "online"
		if !identity.Enabled || identity.RevokedAt != nil || now.After(next.Add(3*grace)) {
			health = "offline"
		} else if now.After(next.Add(grace)) {
			health = "degraded"
		}
		values = append(values, domain.AgentOperationalView{ID: identity.ID, Name: identity.Name, Source: "real", ProtocolVersion: metadata.ProtocolVersion, BuildVersion: metadata.Build, Capabilities: metadata.Capabilities, Architecture: metadata.Architecture, OS: metadata.OS, Generation: identity.Generation, Enabled: identity.Enabled, RevokedAt: identity.RevokedAt, FirstSeen: identity.FirstSeen, LastCheckin: identity.LastCheckin, NextExpectedCheckin: next, Health: health, IntervalMS: metadata.IntervalMS, LastSequence: identity.LastSequence, LastCursor: identity.LastCursor})
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) gatewayAgentTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.store.GatewayTasks(r.Context(), r.PathValue("id"))
	if err != nil {
		s.internalError(w, err)
		return
	}
	values := make([]domain.GatewayTaskOperationalView, 0, len(tasks))
	for _, task := range tasks {
		values = append(values, operationalGatewayTask(task))
	}
	writeJSON(w, http.StatusOK, values)
}

func operationalGatewayTask(task domain.GatewayTask) domain.GatewayTaskOperationalView {
	view := domain.GatewayTaskOperationalView{ID: task.ID, AgentID: task.AgentID, CorrelationID: task.ID, Version: task.Version, IdempotencyKey: task.IdempotencyKey, Kind: task.Kind, Status: task.Status, LeaseUntil: task.LeaseUntil, Deadline: task.Deadline, TimeoutMS: task.TimeoutMS, CancelRequestedAt: task.CancelRequestedAt, StartedAt: task.StartedAt, DispatchCount: task.DispatchCount, CreatedAt: task.CreatedAt, CompletedAt: task.CompletedAt}
	if task.DispatchCount > 1 {
		view.Retries = task.DispatchCount - 1
	}
	if task.ErrorCode != "" {
		view.Error = &domain.GatewayTaskErrorView{Code: task.ErrorCode, Message: task.ErrorMessage, Retryable: task.ErrorRetryable}
	}
	return view
}

func (s *Server) createGatewayTask(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Version        uint16          `json:"version"`
		IdempotencyKey string          `json:"idempotencyKey"`
		Kind           string          `json:"kind"`
		Payload        json.RawMessage `json:"payload"`
		Deadline       time.Time       `json:"deadline"`
		TimeoutMS      int64           `json:"timeoutMs"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	identity, err := s.store.AgentIdentity(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	var metadata struct {
		Capabilities []string `json:"capabilities"`
	}
	if json.Unmarshal(identity.Metadata, &metadata) != nil || !supportsTask(metadata.Capabilities, request.Kind, request.Payload) {
		writeError(w, http.StatusConflict, "agent does not advertise the requested capability")
		return
	}
	task, err := s.store.CreateGatewayTask(r.Context(), domain.GatewayTask{ID: newID("task"), AgentID: r.PathValue("id"), Version: request.Version, IdempotencyKey: request.IdempotencyKey, Kind: request.Kind, Payload: request.Payload, Deadline: request.Deadline, TimeoutMS: request.TimeoutMS})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "AgentTaskQueued", task.ID, task.AgentID)
	writeJSON(w, http.StatusCreated, operationalGatewayTask(task))
}

func supportsTask(capabilities []string, kind string, payload json.RawMessage) bool {
	wanted := kind
	if kind == "shell" {
		var command struct {
			Interpreter string `json:"interpreter"`
		}
		if json.Unmarshal(payload, &command) != nil || command.Interpreter == "" {
			return false
		}
		wanted = "shell:" + command.Interpreter
	}
	for _, capability := range capabilities {
		if capability == wanted {
			return true
		}
	}
	return false
}

func (s *Server) cancelGatewayTask(w http.ResponseWriter, r *http.Request) {
	status, err := s.store.CancelGatewayTask(r.Context(), r.PathValue("id"), r.PathValue("taskID"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusConflict, "task is terminal or does not belong to agent")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r.Context(), principal(r).Username, "AgentTaskCancellationRequested", r.PathValue("taskID"), r.PathValue("id")+"; "+status)
	writeJSON(w, http.StatusAccepted, map[string]string{"id": r.PathValue("taskID"), "status": status})
}

func (s *Server) manageAgentIdentity(w http.ResponseWriter, r *http.Request) {
	id, action := r.PathValue("id"), r.PathValue("action")
	var err error
	var identity string
	switch action {
	case "disable":
		err = s.store.SetAgentEnabled(r.Context(), id, false)
	case "enable":
		err = s.store.SetAgentEnabled(r.Context(), id, true)
	case "revoke":
		err = s.store.RevokeAgent(r.Context(), id)
	case "rotate":
		identity, err = secureToken()
		if err == nil {
			err = s.store.RotateAgentIdentity(r.Context(), id, identity)
		}
	default:
		writeError(w, http.StatusNotFound, "unknown identity action")
		return
	}
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "agent identity not found")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r.Context(), principal(r).Username, "AgentIdentityChanged", id, action)
	response := map[string]any{"id": id, "action": action}
	if identity != "" {
		response["identity"] = identity
	}
	writeJSON(w, http.StatusOK, response)
}

func secureToken() (string, error) {
	buffer := make([]byte, 32)
	defer clear(buffer)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}
func mustAck(value gateway.Ack) []byte {
	frame, err := gateway.EncodeAck(value)
	if err != nil {
		panic(err)
	}
	return frame
}
func writeFrame(w http.ResponseWriter, status int, frame []byte) {
	w.Header().Set("Content-Type", "application/vnd.specter.gateway+gzip")
	w.Header().Set("X-Specter-Protocol-Version", strconv.Itoa(gateway.Version))
	w.WriteHeader(status)
	_, _ = w.Write(frame)
}
