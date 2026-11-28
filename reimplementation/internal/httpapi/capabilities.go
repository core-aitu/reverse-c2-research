package httpapi

import (
	"net/http"

	"github.com/research/specter-reimplementation/internal/capability"
)

func (s *Server) capabilities(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, capability.All())
}
