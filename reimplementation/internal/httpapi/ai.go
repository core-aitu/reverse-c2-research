package httpapi

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/research/specter-reimplementation/internal/aifixture"
	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/store"
)

func (s *Server) aiRoutes() {
	s.mux.Handle("GET /api/AiAgentDefinitions", s.protected(http.HandlerFunc(s.aiAgents)))
	s.mux.Handle("POST /api/AiAgentDefinitions", s.requireRole("admin", http.HandlerFunc(s.createAiAgent)))
	s.mux.Handle("PUT /api/AiAgentDefinitions/{agentId}", s.requireRole("admin", http.HandlerFunc(s.updateAiAgent)))
	s.mux.Handle("DELETE /api/AiAgentDefinitions/{agentId}", s.requireRole("admin", http.HandlerFunc(s.deleteAiAgent)))

	s.mux.Handle("GET /api/AiFunctionDefinitions", s.protected(http.HandlerFunc(s.aiFunctions)))
	s.mux.Handle("POST /api/AiFunctionDefinitions", s.requireRole("admin", http.HandlerFunc(s.createAiFunction)))
	s.mux.Handle("PUT /api/AiFunctionDefinitions/{funcId}", s.requireRole("admin", http.HandlerFunc(s.updateAiFunction)))
	s.mux.Handle("DELETE /api/AiFunctionDefinitions/{funcId}", s.requireRole("admin", http.HandlerFunc(s.deleteAiFunction)))
	s.mux.Handle("POST /api/AiFunctionDefinitions/{funcId}/run", s.requireRole("admin", http.HandlerFunc(s.runAiFunction)))

	s.mux.Handle("GET /api/AiSessions/AiModelsList", s.protected(http.HandlerFunc(s.aiModelsList)))
	s.mux.Handle("GET /api/AiSessions/AiModelsListAll", s.protected(http.HandlerFunc(s.aiModelsListAll)))
	s.mux.Handle("POST /api/AiSessions/NewAiSession", s.requireRole("admin", http.HandlerFunc(s.newAiSession)))
	s.mux.Handle("GET /api/AiSessions/GetAiSessions", s.protected(http.HandlerFunc(s.aiSessions)))
	s.mux.Handle("GET /api/AiSessions/GetAiSessionBySessionId", s.protected(http.HandlerFunc(s.aiSession)))
	s.mux.Handle("POST /api/AiSessions/DeleteAiSession", s.requireRole("admin", http.HandlerFunc(s.deleteAiSession)))
	s.mux.Handle("GET /api/AiSessions/GetAiMessagesBySessionId", s.protected(http.HandlerFunc(s.aiMessages)))
	s.mux.Handle("GET /api/AiSessions/NewUserMessage", s.requireRole("admin", http.HandlerFunc(s.newAiMessage)))
	s.mux.Handle("GET /api/AiSessions/ApproveOrDenyAiSessionTask", s.requireRole("admin", http.HandlerFunc(s.decideAiSessionTask)))
	s.mux.Handle("GET /api/AiSessions/GetAiFunctionCallResultsBySessionId", s.protected(http.HandlerFunc(s.aiFunctionResults)))
	s.mux.Handle("POST /api/AiSessions/PauseAiSession", s.requireRole("admin", http.HandlerFunc(s.pauseAiSession)))
	s.mux.Handle("POST /api/AiSessions/ResumeAiSession", s.requireRole("admin", http.HandlerFunc(s.resumeAiSession)))
	s.mux.Handle("GET /api/AiSessions/CancelAiSessionTask", s.requireRole("admin", http.HandlerFunc(s.cancelAiSession)))
	s.mux.Handle("GET /api/AiSessions/GetSessionStatus", s.protected(http.HandlerFunc(s.aiSessionStatus)))
}

func (s *Server) aiAgents(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.AiAgents(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) createAiAgent(w http.ResponseWriter, r *http.Request) {
	var value domain.AiAgentDefinition
	if !decodeJSON(w, r, &value) {
		return
	}
	if err := s.store.PutAiAgent(r.Context(), "", value); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "AiAgentDefinitionCreated", value.AgentID, value.Name)
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) updateAiAgent(w http.ResponseWriter, r *http.Request) {
	var value domain.AiAgentDefinition
	if !decodeJSON(w, r, &value) {
		return
	}
	value.AgentID = r.PathValue("agentId")
	if err := s.store.PutAiAgent(r.Context(), value.AgentID, value); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "AiAgentDefinitionUpdated", value.AgentID, value.Name)
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) deleteAiAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("agentId")
	if err := s.store.DeleteAiAgent(r.Context(), id); err != nil {
		writeCatalogError(w, err, "AI agent definition not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "AiAgentDefinitionDeleted", id, "definition removed")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) aiFunctions(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.AiFunctions(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) createAiFunction(w http.ResponseWriter, r *http.Request) {
	var value domain.AiFunctionDefinition
	if !decodeJSON(w, r, &value) {
		return
	}
	definition, err := aifixture.Parse(value.Definition)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.PutAiFunction(r.Context(), "", "fixture."+definition.Operation, value); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "AiFunctionDefinitionCreated", value.FunctionID, definition.Operation)
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) updateAiFunction(w http.ResponseWriter, r *http.Request) {
	var value domain.AiFunctionDefinition
	if !decodeJSON(w, r, &value) {
		return
	}
	value.FunctionID = r.PathValue("funcId")
	definition, err := aifixture.Parse(value.Definition)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.PutAiFunction(r.Context(), value.FunctionID, "fixture."+definition.Operation, value); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "AiFunctionDefinitionUpdated", value.FunctionID, definition.Operation)
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) deleteAiFunction(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("funcId")
	if err := s.store.DeleteAiFunction(r.Context(), id); err != nil {
		writeCatalogError(w, err, "AI function definition not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "AiFunctionDefinitionDeleted", id, "definition removed")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) runAiFunction(w http.ResponseWriter, r *http.Request) {
	var arguments map[string]string
	if !decodeJSON(w, r, &arguments) {
		return
	}
	value, err := s.store.AiFunction(r.Context(), r.PathValue("funcId"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "AI function definition not found")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	definition, err := aifixture.Parse(value.Definition)
	if err != nil {
		writeError(w, http.StatusConflict, "stored AI function is not an executable fixture")
		return
	}
	output, err := aifixture.Run(definition, arguments)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "AiFixtureFunctionExecuted", value.FunctionID, definition.Operation)
	writeJSON(w, http.StatusOK, domain.RunAiFunctionResult{Succeeded: true, Output: output})
}

func (s *Server) aiModelsList(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []string{"fixture-deterministic"})
}

func (s *Server) aiModelsListAll(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, []domain.AiModelList{{Kind: "fixture", Models: []string{"fixture-deterministic"}}})
}

func (s *Server) newAiSession(w http.ResponseWriter, r *http.Request) {
	var value domain.AiSessionInfo
	if !decodeJSON(w, r, &value) {
		return
	}
	// The management plane exposes lifecycle state but does not start a model.
	value.ModelType = "fixture"
	value.ModelName = "fixture-deterministic"
	value.Status = "Stopped"
	sessionID, err := s.store.CreateAiSession(r.Context(), value)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "AiSessionCreated", sessionID, "fixture provider; no model started")
	writeJSON(w, http.StatusOK, sessionID)
}

func (s *Server) aiSessions(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.AiSessions(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) aiSession(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.AiSession(r.Context(), r.URL.Query().Get("sessionId"))
	if err != nil {
		writeCatalogError(w, err, "AI session not found")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) deleteAiSession(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("sessionId")
	if err := s.store.DeleteAiSession(r.Context(), id); err != nil {
		writeCatalogError(w, err, "AI session not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "AiSessionDeleted", id, "session data removed")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) aiMessages(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.AiMessages(r.Context(), r.URL.Query().Get("sessionId"))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) newAiMessage(w http.ResponseWriter, r *http.Request) {
	var value domain.AiMessage
	if !decodeJSON(w, r, &value) {
		return
	}
	if err := s.store.AddAiMessage(r.Context(), value); err != nil {
		writeCatalogError(w, err, "AI session not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "AiFixtureMessageStored", value.SessionID, "message persisted; no provider called")
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) decideAiSessionTask(w http.ResponseWriter, r *http.Request) {
	approved, err := strconv.ParseBool(r.URL.Query().Get("approved"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "approved is required")
		return
	}
	toolCallID := r.URL.Query().Get("toolCallId")
	if err := s.store.DecideAiFunctionCall(r.Context(), toolCallID, approved); err != nil {
		writeCatalogError(w, err, "AI function call not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "AiFunctionCallDecision", toolCallID, strconv.FormatBool(approved))
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) aiFunctionResults(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.AiFunctionResults(r.Context(), r.URL.Query().Get("sessionId"))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) pauseAiSession(w http.ResponseWriter, r *http.Request) {
	s.setAiSessionStatus(w, r, "Paused")
}

func (s *Server) resumeAiSession(w http.ResponseWriter, r *http.Request) {
	// Resume restores the persistent state only; no provider process is started.
	s.setAiSessionStatus(w, r, "Running")
}

func (s *Server) cancelAiSession(w http.ResponseWriter, r *http.Request) {
	s.setAiSessionStatus(w, r, "Stopped")
}

func (s *Server) setAiSessionStatus(w http.ResponseWriter, r *http.Request, status string) {
	id := r.URL.Query().Get("sessionId")
	if err := s.store.SetAiSessionStatus(r.Context(), id, status); err != nil {
		writeCatalogError(w, err, "AI session not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "AiSessionStatusChanged", id, status+"; fixture state only")
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) aiSessionStatus(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.AiSession(r.Context(), r.URL.Query().Get("sessionId"))
	if err != nil {
		writeCatalogError(w, err, "AI session not found")
		return
	}
	writeJSON(w, http.StatusOK, domain.AiStatusUpdateMessage{SessionID: value.SessionID,
		SourceName: "fixture", Message: value.Status + ": persistent fixture state; no external model runtime"})
}
