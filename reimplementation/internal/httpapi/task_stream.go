package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/research/specter-reimplementation/internal/domain"
	"github.com/research/specter-reimplementation/internal/store"
)

type taskOutputTicket struct {
	TaskID  string
	Expires time.Time
}

type taskOutputEnvelope struct {
	ID         int64                  `json:"id"`
	Type       string                 `json:"type"`
	OccurredAt time.Time              `json:"occurredAt"`
	Version    int                    `json:"version"`
	Payload    domain.TaskOutputChunk `json:"payload"`
}

func (s *Server) gatewayTaskDetail(w http.ResponseWriter, r *http.Request) {
	task, err := s.store.GatewayTask(r.Context(), r.PathValue("id"), r.PathValue("taskID"))
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	if err != nil {
		s.internalError(w, err)
		return
	}
	outputs, err := s.store.TaskOutputObjects(r.Context(), task.ID)
	if err != nil {
		s.internalError(w, err)
		return
	}
	result := task.Result
	if task.Kind == "file-download" && len(result) > 0 {
		var value map[string]any
		if json.Unmarshal(result, &value) == nil {
			if objectID, ok := value["objectId"].(string); ok && objectID != "" {
				if object, objectErr := s.store.ObjectInfo(r.Context(), objectID); objectErr == nil {
					value["object"] = domain.ObjectInfo{ID: object.ID, Kind: object.Kind, Key: object.Key, Name: object.OriginalName, SHA256: object.SHA256, MediaType: object.MediaType, Size: object.Size, CreatedAt: object.CreatedAt}
					result, _ = json.Marshal(value)
				}
			}
		}
	}
	detail := domain.GatewayTaskDetail{GatewayTaskOperationalView: operationalGatewayTask(task), Payload: task.Payload, Result: result, Outputs: outputs}
	writeJSON(w, http.StatusOK, detail)
}

func (s *Server) createTaskOutputTicket(w http.ResponseWriter, r *http.Request) {
	if _, err := s.store.GatewayTask(r.Context(), r.PathValue("id"), r.PathValue("taskID")); err != nil {
		writeError(w, http.StatusNotFound, "task not found")
		return
	}
	ticket, err := secureToken()
	if err != nil {
		s.internalError(w, err)
		return
	}
	expires := time.Now().UTC().Add(time.Minute)
	s.outputMu.Lock()
	for key, value := range s.outputTickets {
		if time.Now().UTC().After(value.Expires) {
			delete(s.outputTickets, key)
		}
	}
	s.outputTickets[ticket] = taskOutputTicket{TaskID: r.PathValue("taskID"), Expires: expires}
	s.outputMu.Unlock()
	writeJSON(w, http.StatusCreated, map[string]any{"ticket": ticket, "expiresAt": expires})
}

func (s *Server) consumeTaskOutputTicket(value, taskID string) bool {
	s.outputMu.Lock()
	defer s.outputMu.Unlock()
	ticket, ok := s.outputTickets[value]
	delete(s.outputTickets, value)
	return ok && ticket.TaskID == taskID && time.Now().UTC().Before(ticket.Expires)
}

func (s *Server) taskOutputSocket(w http.ResponseWriter, r *http.Request) {
	if !s.validOrigin(r) {
		writeError(w, http.StatusForbidden, "origin rejected")
		return
	}
	taskID := r.URL.Query().Get("taskId")
	if taskID == "" || !s.consumeTaskOutputTicket(r.URL.Query().Get("ticket"), taskID) {
		writeError(w, http.StatusUnauthorized, "invalid task output ticket")
		return
	}
	connection, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: []string{originPattern(s.cfg.AllowedOrigin)}})
	if err != nil {
		return
	}
	defer connection.CloseNow()
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	channel, unsubscribe := s.subscribeTaskOutput(taskID)
	defer unsubscribe()
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	replay, err := s.store.TaskOutputChunks(ctx, taskID, after)
	if err != nil {
		_ = connection.Close(websocket.StatusInternalError, "output replay failed")
		return
	}
	last := after
	for _, chunk := range replay {
		if err := writeTaskOutput(ctx, connection, chunk); err != nil {
			return
		}
		last = chunk.ID
	}
	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-channel:
			if !ok {
				return
			}
			if chunk.ID <= last {
				continue
			}
			if err := writeTaskOutput(ctx, connection, chunk); err != nil {
				return
			}
			last = chunk.ID
		}
	}
}

func writeTaskOutput(ctx context.Context, connection *websocket.Conn, chunk domain.TaskOutputChunk) error {
	envelope := taskOutputEnvelope{ID: chunk.ID, Type: "AgentTaskOutput", OccurredAt: chunk.CreatedAt, Version: 1, Payload: chunk}
	writeContext, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return wsjson.Write(writeContext, connection, envelope)
}

func (s *Server) subscribeTaskOutput(taskID string) (<-chan domain.TaskOutputChunk, func()) {
	channel := make(chan domain.TaskOutputChunk, 128)
	s.outputMu.Lock()
	if s.outputSubs[taskID] == nil {
		s.outputSubs[taskID] = map[chan domain.TaskOutputChunk]struct{}{}
	}
	s.outputSubs[taskID][channel] = struct{}{}
	s.outputMu.Unlock()
	return channel, func() {
		s.outputMu.Lock()
		if _, ok := s.outputSubs[taskID][channel]; ok {
			delete(s.outputSubs[taskID], channel)
			close(channel)
		}
		if len(s.outputSubs[taskID]) == 0 {
			delete(s.outputSubs, taskID)
		}
		s.outputMu.Unlock()
	}
}

func (s *Server) publishTaskOutput(chunk domain.TaskOutputChunk) {
	s.outputMu.RLock()
	defer s.outputMu.RUnlock()
	for subscriber := range s.outputSubs[chunk.TaskID] {
		select {
		case subscriber <- chunk:
		default:
		}
	}
}

func taskOutputJSON(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}
