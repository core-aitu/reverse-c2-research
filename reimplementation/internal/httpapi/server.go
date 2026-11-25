package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/research/specter-reimplementation/internal/auth"
	"github.com/research/specter-reimplementation/internal/config"
	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/events"
	"github.com/research/specter-reimplementation/internal/store"
)

type Server struct {
	cfg   config.Config
	store *store.Store
	auth  *auth.Service
	hub   *events.Hub
	log   *slog.Logger
	mux   *http.ServeMux
}

type principalKey struct{}

func New(cfg config.Config, store *store.Store, authService *auth.Service, hub *events.Hub, logger *slog.Logger) http.Handler {
	server := &Server{cfg: cfg, store: store, auth: authService, hub: hub, log: logger, mux: http.NewServeMux()}
	server.routes()
	return server.securityHeaders(server.logging(server.mux))
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /readyz", s.ready)
	s.mux.HandleFunc("POST /api/Accounts/authenticate", s.authenticate)
	s.mux.HandleFunc("POST /api/Accounts/refresh-token", s.refresh)
	s.mux.Handle("POST /api/Accounts/revoke-token", s.protected(http.HandlerFunc(s.revoke)))
	s.mux.Handle("GET /api/Identity", s.protected(http.HandlerFunc(s.identity)))
	s.mux.Handle("GET /api/Accounts", s.requireRole("admin", http.HandlerFunc(s.accounts)))
	s.mux.Handle("POST /api/Accounts", s.requireRole("admin", http.HandlerFunc(s.createAccount)))
	s.mux.Handle("GET /api/Accounts/{username}", s.requireRole("admin", http.HandlerFunc(s.account)))
	s.mux.Handle("PUT /api/Accounts/{name}", s.requireRole("admin", http.HandlerFunc(s.updateAccount)))
	s.mux.Handle("DELETE /api/Accounts/{username}", s.requireRole("admin", http.HandlerFunc(s.deleteAccount)))
	s.mux.Handle("GET /api/Documentation", s.protected(http.HandlerFunc(s.documentation)))
	s.mux.Handle("POST /api/Documentation/search", s.protected(http.HandlerFunc(s.searchDocumentation)))
	s.mux.Handle("GET /api/Documentation/index-status", s.protected(http.HandlerFunc(s.documentationIndexStatus)))
	s.mux.Handle("POST /api/Documentation/index", s.requireRole("admin", http.HandlerFunc(s.documentationIndex)))
	s.mux.Handle("GET /api/Events/Events/GetLatestEvents", s.protected(http.HandlerFunc(s.latestEvents)))
	s.mux.Handle("GET /api/events/latest", s.protected(http.HandlerFunc(s.latestEvents)))
	s.mux.Handle("POST /api/events/ticket", s.protected(http.HandlerFunc(s.eventTicket)))
	s.mux.HandleFunc("GET /api/events", s.eventSocket)
	s.mux.Handle("GET /api/Settings", s.protected(http.HandlerFunc(s.settings)))
	s.mux.Handle("PUT /api/Settings", s.requireRole("admin", http.HandlerFunc(s.putSettings)))
	s.managementRoutes()
	s.researchScenarioRoutes()
	s.researchJournalRoutes()
	if s.cfg.FixturesEnabled {
		s.mux.Handle("POST /api/fixtures/event", s.requireRole("admin", http.HandlerFunc(s.fixtureEvent)))
	}
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

type authenticateRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) {
	var request authenticateRequest
	if !decodeJSON(w, r, &request) || request.Username == "" || request.Password == "" {
		return
	}
	tokens, err := s.auth.Login(r.Context(), request.Username, request.Password, clientIP(r))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	s.setRefreshCookie(w, tokens.RefreshToken)
	tokens.RefreshToken = ""
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) refresh(w http.ResponseWriter, r *http.Request) {
	if !s.validOrigin(r) {
		writeError(w, http.StatusForbidden, "origin rejected")
		return
	}
	cookie, err := r.Cookie("specter_refresh")
	if err != nil {
		writeError(w, http.StatusUnauthorized, "refresh token missing")
		return
	}
	tokens, err := s.auth.Refresh(r.Context(), cookie.Value, clientIP(r))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "refresh token invalid")
		return
	}
	s.setRefreshCookie(w, tokens.RefreshToken)
	tokens.RefreshToken = ""
	writeJSON(w, http.StatusOK, tokens)
}

func (s *Server) revoke(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("specter_refresh"); err == nil {
		_ = s.store.RevokeRefreshToken(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "specter_refresh", Path: "/api/Accounts", MaxAge: -1, HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteStrictMode})
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

func (s *Server) identity(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, principal(r))
}

func (s *Server) accounts(w http.ResponseWriter, r *http.Request) {
	accounts, err := s.store.Accounts(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, accounts)
}

type createAccountRequest struct {
	Username string   `json:"username"`
	Password string   `json:"password"`
	Enabled  *bool    `json:"enabled"`
	Roles    []string `json:"roles"`
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	var request createAccountRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	enabled := true
	if request.Enabled != nil {
		enabled = *request.Enabled
	}
	account, err := s.store.CreateAccount(r.Context(), request.Username, request.Password, enabled, request.Roles)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "AccountCreated", account.Username, strings.Join(account.Roles, ","))
	writeJSON(w, http.StatusOK, account)
}

func (s *Server) account(w http.ResponseWriter, r *http.Request) {
	account, err := s.store.AccountByUsername(r.Context(), r.PathValue("username"))
	if err != nil {
		writeError(w, http.StatusNotFound, "account not found")
		return
	}
	writeJSON(w, http.StatusOK, account)
}

type updateAccountRequest struct {
	Password *string  `json:"password"`
	Enabled  *bool    `json:"enabled"`
	Roles    []string `json:"roles"`
}

func (s *Server) updateAccount(w http.ResponseWriter, r *http.Request) {
	var request updateAccountRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	account, err := s.store.UpdateAccount(r.Context(), r.PathValue("name"), request.Password, request.Enabled, request.Roles)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "AccountUpdated", account.Username, strings.Join(account.Roles, ","))
	writeJSON(w, http.StatusOK, account)
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")
	if username == principal(r).Username {
		writeError(w, http.StatusConflict, "cannot delete the current account")
		return
	}
	if err := s.store.DeleteAccount(r.Context(), username); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "AccountDeleted", username, "refresh sessions revoked by cascade")
	writeJSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

func (s *Server) documentation(w http.ResponseWriter, r *http.Request) {
	entries, err := s.store.Documentation(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

type documentationSearchRequest struct {
	Query    string `json:"query"`
	TopK     int    `json:"topK"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

func (s *Server) searchDocumentation(w http.ResponseWriter, r *http.Request) {
	var request documentationSearchRequest
	if !decodeJSON(w, r, &request) {
		return
	}
	if strings.TrimSpace(request.Query) == "" {
		writeJSON(w, http.StatusOK, []domain.DocumentationSearchResult{})
		return
	}
	results, err := s.store.SearchDocumentation(r.Context(), request.Query, request.TopK)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, results)
}

func (s *Server) documentationIndexStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"isIndexing": false, "isAvailable": true, "provider": "postgres-trigram"})
}

func (s *Server) documentationIndex(w http.ResponseWriter, r *http.Request) {
	if err := s.store.SeedDocumentation(r.Context()); err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "complete"})
}

func (s *Server) latestEvents(w http.ResponseWriter, r *http.Request) {
	var from *time.Time
	if value := r.URL.Query().Get("from"); value != "" {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid from timestamp")
			return
		}
		from = &parsed
	}
	events, err := s.store.LatestEvents(r.Context(), from, 200)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, events)
}

func (s *Server) eventTicket(w http.ResponseWriter, r *http.Request) {
	ticket, expires, err := s.hub.Ticket(principal(r).Username)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ticket": ticket, "expiresAt": expires})
}

func (s *Server) eventSocket(w http.ResponseWriter, r *http.Request) {
	if !s.validOrigin(r) {
		writeError(w, http.StatusForbidden, "origin rejected")
		return
	}
	if _, ok := s.hub.ConsumeTicket(r.URL.Query().Get("ticket")); !ok {
		writeError(w, http.StatusUnauthorized, "invalid event ticket")
		return
	}
	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{originPattern(s.cfg.AllowedOrigin)}})
	if err != nil {
		return
	}
	defer connection.CloseNow()
	connectionContext, cancelConnection := context.WithCancel(r.Context())
	defer cancelConnection()
	go func() {
		for {
			if _, _, err := connection.Read(connectionContext); err != nil {
				cancelConnection()
				return
			}
		}
	}()
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	replay, err := s.store.EventsAfter(connectionContext, after, 1000)
	if err != nil {
		_ = connection.Close(websocket.StatusInternalError, "replay failed")
		return
	}
	for _, event := range replay {
		if err := wsjson.Write(connectionContext, connection, domain.Envelope(event)); err != nil {
			return
		}
	}
	channel, unsubscribe := s.hub.Subscribe()
	defer unsubscribe()
	for {
		select {
		case <-connectionContext.Done():
			return
		case envelope, ok := <-channel:
			if !ok {
				return
			}
			ctx, cancel := context.WithTimeout(connectionContext, 10*time.Second)
			err := wsjson.Write(ctx, connection, envelope)
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func (s *Server) settings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.store.Settings(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (s *Server) putSettings(w http.ResponseWriter, r *http.Request) {
	var settings []domain.Setting
	if !decodeJSON(w, r, &settings) {
		return
	}
	if err := s.store.PutSettings(r.Context(), settings); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"saved": true})
}

func (s *Server) fixtureEvent(w http.ResponseWriter, r *http.Request) {
	event, err := s.store.AddEvent(r.Context(), domain.Event{
		Identifier: "fixture-live-window", Timestamp: time.Now().UTC(), Type: "ResearchFixture",
		Source: "fixture-api", Outcome: "Success", System: "reimplementation",
		Message: "fixture-live-window", Details: "PostgreSQL to WebSocket to browser verification event",
	})
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.hub.Publish(event)
	writeJSON(w, http.StatusCreated, event)
}

func (s *Server) protected(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			writeError(w, http.StatusUnauthorized, "bearer token required")
			return
		}
		claims, err := s.auth.Parse(strings.TrimPrefix(header, "Bearer "))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "bearer token invalid")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, claims)))
	})
}

func (s *Server) requireRole(role string, next http.Handler) http.Handler {
	return s.protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, candidate := range principal(r).Roles {
			if candidate == role {
				next.ServeHTTP(w, r)
				return
			}
		}
		writeError(w, http.StatusForbidden, "insufficient role")
	}))
}

func principal(r *http.Request) auth.Claims {
	claims, _ := r.Context().Value(principalKey{}).(auth.Claims)
	return claims
}

func (s *Server) setRefreshCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name: "specter_refresh", Value: token, Path: "/api/Accounts", MaxAge: int(s.cfg.RefreshTTL.Seconds()),
		HttpOnly: true, Secure: s.cfg.SecureCookies, SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) validOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return origin == "" || origin == s.cfg.AllowedOrigin
}

func originPattern(origin string) string {
	origin = strings.TrimPrefix(origin, "https://")
	return strings.TrimPrefix(origin, "http://")
}

func (s *Server) internalError(w http.ResponseWriter, err error) {
	s.log.Error("request failed", "error", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		next.ServeHTTP(w, r)
		s.log.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'none'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
