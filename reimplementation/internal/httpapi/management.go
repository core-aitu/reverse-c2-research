package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/research/specter-reimplementation/internal/adapters"
	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/pipeline"
	"github.com/research/specter-reimplementation/internal/store"
	"github.com/research/specter-reimplementation/internal/worker"
)

func (s *Server) managementRoutes() {
	s.mux.Handle("GET /api/workflows/{kind}", s.protected(http.HandlerFunc(s.resources)))
	s.mux.Handle("PUT /api/workflows/{kind}", s.requireRole("admin", http.HandlerFunc(s.putResource)))
	s.mux.Handle("DELETE /api/workflows/{kind}/{key}", s.requireRole("admin", http.HandlerFunc(s.deleteResource)))

	for path, kind := range map[string]string{
		"Payloads": "implants", "Sessions": "sessions",
		"WinRmSessions": "winrm", "Tunnels": "tunnels",
		"Artifacts": "files", "Updates": "updates", "License": "license",
	} {
		s.registerResourceAlias(path, kind)
	}
	s.catalogRoutes()
	s.aiRoutes()

	s.mux.Handle("GET /api/adapters", s.protected(http.HandlerFunc(s.adapters)))
	s.mux.Handle("PUT /api/adapters/{name}", s.requireRole("admin", http.HandlerFunc(s.putAdapter)))
	s.mux.Handle("POST /api/adapters/{name}/probe", s.requireRole("admin", http.HandlerFunc(s.probeAdapter)))
	s.mux.Handle("POST /api/adapters/{name}/fixture/{operation}", s.requireRole("admin", http.HandlerFunc(s.runAdapterFixture)))

	s.mux.Handle("GET /api/jobs", s.protected(http.HandlerFunc(s.jobs)))
	s.mux.Handle("POST /api/jobs/{id}/cancel", s.requireRole("admin", http.HandlerFunc(s.cancelJob)))

	s.mux.Handle("GET /api/pipelines", s.protected(http.HandlerFunc(s.pipelines)))
	s.mux.Handle("PUT /api/pipelines", s.requireRole("admin", http.HandlerFunc(s.putPipeline)))
	s.mux.Handle("DELETE /api/pipelines/{id}", s.requireRole("admin", http.HandlerFunc(s.deletePipeline)))
	s.mux.Handle("POST /api/pipelines/validate", s.protected(http.HandlerFunc(s.validatePipeline)))
	s.mux.Handle("POST /api/pipelines/{id}/run", s.requireRole("admin", http.HandlerFunc(s.runPipeline)))
	s.mux.Handle("GET /api/PayloadPipeline", s.protected(http.HandlerFunc(s.pipelines)))
	s.mux.Handle("PUT /api/PayloadPipeline", s.requireRole("admin", http.HandlerFunc(s.putPipeline)))
	s.mux.Handle("GET /api/PayloadPipeline/PayloadPipeline/{id}", s.protected(http.HandlerFunc(s.pipelineByID)))
	s.mux.Handle("DELETE /api/PayloadPipeline/PayloadPipeline/{id}", s.requireRole("admin", http.HandlerFunc(s.deletePipeline)))
	s.mux.Handle("POST /api/PayloadPipeline/PayloadPipeline/{id}/run", s.requireRole("admin", http.HandlerFunc(s.runPipeline)))

	s.mux.Handle("GET /api/gateway/sessions", s.protected(http.HandlerFunc(s.agentSessions)))
	s.mux.Handle("POST /api/gateway/simulate/register", s.requireRole("admin", http.HandlerFunc(s.registerAgent)))
	s.mux.Handle("GET /api/gateway/sessions/{id}/tasks", s.protected(http.HandlerFunc(s.agentTasks)))
	s.mux.Handle("POST /api/gateway/sessions/{id}/tasks", s.requireRole("admin", http.HandlerFunc(s.createAgentTask)))
	s.mux.Handle("POST /api/gateway/simulate/{id}/checkin", s.requireRole("admin", http.HandlerFunc(s.agentCheckin)))
	s.mux.Handle("POST /api/gateway/simulate/{id}/tasks/{task}/result", s.requireRole("admin", http.HandlerFunc(s.agentResult)))
}

func (s *Server) registerResourceAlias(path, kind string) {
	s.mux.Handle("GET /api/"+path, s.protected(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resources, err := s.store.Resources(r.Context(), kind)
		if err != nil {
			s.internalError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, resources)
	})))
	for _, method := range []string{"POST", "PUT"} {
		s.mux.Handle(method+" /api/"+path, s.requireRole("admin", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var value domain.Resource
			if !decodeJSON(w, r, &value) {
				return
			}
			value.Kind = kind
			s.saveResource(w, r, value)
		})))
	}
}

func (s *Server) resources(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.Resources(r.Context(), r.PathValue("kind"))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) putResource(w http.ResponseWriter, r *http.Request) {
	var value domain.Resource
	if !decodeJSON(w, r, &value) {
		return
	}
	value.Kind = r.PathValue("kind")
	s.saveResource(w, r, value)
}

func (s *Server) saveResource(w http.ResponseWriter, r *http.Request, value domain.Resource) {
	saved, err := s.store.PutResource(r.Context(), value)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "ResourceUpdated", saved.Kind, saved.Key)
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) deleteResource(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteResource(r.Context(), r.PathValue("kind"), r.PathValue("key")); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "resource not found")
		} else {
			s.internalError(w, err)
		}
		return
	}
	s.audit(r.Context(), principal(r).Username, "ResourceDeleted", r.PathValue("kind"), r.PathValue("key"))
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) adapters(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.Adapters(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) putAdapter(w http.ResponseWriter, r *http.Request) {
	var value domain.Adapter
	if !decodeJSON(w, r, &value) {
		return
	}
	value.Name = r.PathValue("name")
	saved, err := s.store.PutAdapter(r.Context(), value)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "AdapterConfigured", saved.Name, saved.Mode)
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) probeAdapter(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.Adapter(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusNotFound, "adapter not found")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
	defer cancel()
	result, probeErr := adapters.Probe(ctx, value, s.cfg.RealAdapters)
	status, detail := result.Status, result.Detail
	if probeErr != nil {
		status, detail = "unhealthy", probeErr.Error()
	}
	updated, err := s.store.SetAdapterStatus(r.Context(), value.Name, status, detail)
	if err != nil {
		s.internalError(w, err)
		return
	}
	s.audit(r.Context(), principal(r).Username, "AdapterProbed", value.Name, status)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) runAdapterFixture(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.Adapter(r.Context(), r.PathValue("name"))
	if err != nil {
		writeError(w, http.StatusNotFound, "adapter not found")
		return
	}
	if !s.cfg.FixturesEnabled || !value.Enabled || value.Mode != "fixture" {
		writeError(w, http.StatusConflict, "fixture adapter is not enabled")
		return
	}
	var input json.RawMessage
	if !decodeJSON(w, r, &input) {
		return
	}
	result, err := adapters.ExecuteFixture(value.Name, r.PathValue("operation"), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "AdapterFixtureExecuted", value.Name, result.Operation+"; no external side effect")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) jobs(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.Jobs(r.Context(), 100)
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) cancelJob(w http.ResponseWriter, r *http.Request) {
	if err := s.store.CancelJob(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, "active job not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "JobCancelled", r.PathValue("id"), "cancellation requested")
	writeJSON(w, http.StatusOK, map[string]bool{"cancelled": true})
}

func (s *Server) pipelines(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.Pipelines(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) pipelineByID(w http.ResponseWriter, r *http.Request) {
	value, err := s.store.Pipeline(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "pipeline not found")
		return
	}
	writeJSON(w, http.StatusOK, value)
}

func (s *Server) putPipeline(w http.ResponseWriter, r *http.Request) {
	var value domain.PipelineDefinition
	if !decodeJSON(w, r, &value) {
		return
	}
	definition, err := pipeline.Parse(value.Definition)
	if err == nil {
		_, err = pipeline.Validate(definition)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	saved, err := s.store.PutPipeline(r.Context(), value)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "PipelineUpdated", saved.ID, saved.Name)
	writeJSON(w, http.StatusOK, saved)
}

func (s *Server) deletePipeline(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeletePipeline(r.Context(), r.PathValue("id")); err != nil {
		writeError(w, http.StatusNotFound, "pipeline not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "PipelineDeleted", r.PathValue("id"), "definition removed")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) validatePipeline(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Definition json.RawMessage `json:"definition"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	definition, err := pipeline.Parse(request.Definition)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	order, err := pipeline.Validate(definition)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"valid": true, "order": order})
}

func (s *Server) runPipeline(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Input string `json:"input"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	definition, err := s.store.Pipeline(r.Context(), r.PathValue("id"))
	if err != nil || !definition.Enabled {
		writeError(w, http.StatusNotFound, "enabled pipeline not found")
		return
	}
	input, _ := json.Marshal(map[string]string{"pipelineId": definition.ID, "input": request.Input})
	job, err := s.store.CreateJob(r.Context(), domain.Job{
		ID: newID("job"), Kind: "pipeline", Input: input, MaxAttempts: 1, RequestedBy: principal(r).Username,
	})
	if err != nil {
		s.internalError(w, err)
		return
	}
	go s.executePipeline(job.ID, definition, request.Input)
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) executePipeline(jobID string, stored domain.PipelineDefinition, input string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := s.store.StartJob(ctx, jobID); err != nil {
		return
	}
	definition, err := pipeline.Parse(stored.Definition)
	var output json.RawMessage
	if err == nil {
		var result pipeline.Result
		result, err = worker.ExecuteProcess(ctx, s.cfg.WorkerPath, worker.Request{Version: 1, Definition: definition, Input: input})
		if err == nil {
			output, err = json.Marshal(result)
		}
	}
	_ = s.store.FinishJob(ctx, jobID, output, err)
	outcome := "Success"
	if err != nil {
		outcome = "Failure"
	}
	event, addErr := s.store.AddEvent(ctx, domain.Event{
		Identifier: jobID, Type: "PipelineJobCompleted", Source: "safe-dag-worker", Outcome: outcome,
		System: "reimplementation", Message: stored.Name, Details: "non-executable pipeline job completed",
	})
	if addErr == nil {
		s.hub.Publish(event)
	}
}

func (s *Server) agentSessions(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.AgentSessions(r.Context())
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) registerAgent(w http.ResponseWriter, r *http.Request) {
	var value domain.AgentSession
	if !decodeJSON(w, r, &value) {
		return
	}
	if value.ID == "" {
		value.ID = newID("sim")
	}
	value.Transport = "simulator"
	saved, err := s.store.RegisterAgent(r.Context(), value)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "AgentSimulatorRegistered", saved.ID, saved.Name)
	writeJSON(w, http.StatusCreated, saved)
}

func (s *Server) agentTasks(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.AgentTasks(r.Context(), r.PathValue("id"))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) createAgentTask(w http.ResponseWriter, r *http.Request) {
	var value domain.AgentTask
	if !decodeJSON(w, r, &value) {
		return
	}
	value.ID, value.SessionID = newID("task"), r.PathValue("id")
	saved, err := s.store.CreateAgentTask(r.Context(), value)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r.Context(), principal(r).Username, "SimulatorTaskQueued", saved.ID, saved.Kind)
	writeJSON(w, http.StatusCreated, saved)
}

func (s *Server) agentCheckin(w http.ResponseWriter, r *http.Request) {
	values, err := s.store.DispatchAgentTasks(r.Context(), r.PathValue("id"))
	if err != nil {
		s.internalError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, values)
}

func (s *Server) agentResult(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if !decodeJSON(w, r, &request) {
		return
	}
	if len(request.Result) == 0 {
		request.Result = json.RawMessage(`{}`)
	}
	if err := s.store.CompleteAgentTask(r.Context(), r.PathValue("id"), r.PathValue("task"), request.Result, request.Error); err != nil {
		writeError(w, http.StatusNotFound, "dispatched task not found")
		return
	}
	s.audit(r.Context(), principal(r).Username, "SimulatorTaskCompleted", r.PathValue("task"), r.PathValue("id"))
	writeJSON(w, http.StatusOK, map[string]bool{"accepted": true})
}

func (s *Server) audit(ctx context.Context, username, eventType, subject, detail string) {
	event, err := s.store.AddEvent(ctx, domain.Event{
		Identifier: newID("evt"), Type: eventType, Source: username, Outcome: "Success",
		System: "management-plane", Message: subject, Details: detail,
	})
	if err == nil {
		s.hub.Publish(event)
	}
}

func newID(prefix string) string {
	buffer := make([]byte, 12)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
	}
	return strings.ToLower(prefix) + "-" + hex.EncodeToString(buffer)
}
