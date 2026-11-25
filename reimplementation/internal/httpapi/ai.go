package httpapi

import (
	"errors"
	"net/http"

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
