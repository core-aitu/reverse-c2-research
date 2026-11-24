package domain

import (
	"encoding/json"
	"time"
)

type Resource struct {
	Kind        string          `json:"kind"`
	Key         string          `json:"key"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Status      string          `json:"status"`
	Spec        json.RawMessage `json:"spec"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type Adapter struct {
	Name      string          `json:"name"`
	Category  string          `json:"category"`
	Mode      string          `json:"mode"`
	Enabled   bool            `json:"enabled"`
	Endpoint  string          `json:"endpoint"`
	Status    string          `json:"status"`
	LastError string          `json:"lastError"`
	CheckedAt *time.Time      `json:"checkedAt"`
	Config    json.RawMessage `json:"config"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type Job struct {
	ID              string          `json:"id"`
	Kind            string          `json:"kind"`
	Status          string          `json:"status"`
	Input           json.RawMessage `json:"input"`
	Output          json.RawMessage `json:"output,omitempty"`
	Error           string          `json:"error"`
	Attempt         int             `json:"attempt"`
	MaxAttempts     int             `json:"maxAttempts"`
	CancelRequested bool            `json:"cancelRequested"`
	RequestedBy     string          `json:"requestedBy"`
	CreatedAt       time.Time       `json:"createdAt"`
	StartedAt       *time.Time      `json:"startedAt"`
	CompletedAt     *time.Time      `json:"completedAt"`
}

type PipelineDefinition struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Definition  json.RawMessage `json:"definition"`
	Enabled     bool            `json:"enabled"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

type AgentSession struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Transport   string          `json:"transport"`
	Status      string          `json:"status"`
	Metadata    json.RawMessage `json:"metadata"`
	FirstSeen   time.Time       `json:"firstSeen"`
	LastCheckin time.Time       `json:"lastCheckin"`
}

type AgentTask struct {
	ID          string          `json:"id"`
	SessionID   string          `json:"sessionId"`
	Kind        string          `json:"kind"`
	Payload     json.RawMessage `json:"payload"`
	Status      string          `json:"status"`
	Result      json.RawMessage `json:"result,omitempty"`
	Error       string          `json:"error"`
	CreatedAt   time.Time       `json:"createdAt"`
	CompletedAt *time.Time      `json:"completedAt"`
}
