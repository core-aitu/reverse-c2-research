package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/research/specter-reimplementation/internal/researchscenario"
)

func (s *Server) researchScenarioRoutes() {
	s.mux.Handle("GET /api/research-scenarios", s.requireRole("admin", http.HandlerFunc(s.researchScenarioCatalog)))
	s.mux.Handle("GET /api/research-scenarios/runs", s.requireRole("admin", http.HandlerFunc(s.researchScenarioHistory)))
	s.mux.Handle("POST /api/research-scenarios/runs", s.requireRole("admin", http.HandlerFunc(s.runResearchScenario)))
}

func (s *Server) researchScenarioCatalog(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, researchscenario.Catalog())
}

func (s *Server) researchScenarioHistory(w http.ResponseWriter, r *http.Request) {
	runs, err := s.store.ResearchScenarioRuns(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, runs)
}

func (s *Server) runResearchScenario(w http.ResponseWriter, r *http.Request) {
	var input struct {
		ScenarioID string `json:"scenarioId"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid scenario request")
		return
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		writeError(w, http.StatusBadRequest, "expected exactly one JSON object")
		return
	}
	report, err := researchscenario.Run(input.ScenarioID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "unknown research scenario")
		return
	}
	actor := principal(r).Username
	run, err := s.store.SaveResearchScenarioRun(r.Context(), newID("research-run"), actor, report)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, run)
}
