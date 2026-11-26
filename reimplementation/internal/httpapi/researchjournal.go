package httpapi

import (
	"errors"
	"github.com/research/specter-reimplementation/internal/store"
	"net/http"
)

func (s *Server) researchJournalRoutes() {
	s.mux.Handle("GET /api/research-scenarios/runs/{id}/journal", s.requireRole("admin", http.HandlerFunc(s.researchJournal)))
}

func (s *Server) researchJournal(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if len(id) == 0 || len(id) > 128 {
		writeError(w, http.StatusBadRequest, "invalid run identifier")
		return
	}
	entries, err := s.store.ResearchJournal(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "research run not found")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}
