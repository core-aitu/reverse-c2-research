package domain

import (
	"encoding/json"
	"time"
)

type AgentIdentity struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Generation   int             `json:"generation"`
	Enabled      bool            `json:"enabled"`
	RevokedAt    *time.Time      `json:"revokedAt,omitempty"`
	Metadata     json.RawMessage `json:"metadata"`
	LastSequence uint64          `json:"lastSequence"`
	LastCursor   uint64          `json:"lastCursor"`
	FirstSeen    time.Time       `json:"firstSeen"`
	LastCheckin  time.Time       `json:"lastCheckin"`
}

type GatewayTask struct {
	ID                string          `json:"id"`
	AgentID           string          `json:"agentId"`
	Cursor            uint64          `json:"cursor"`
	Version           uint16          `json:"version"`
	IdempotencyKey    string          `json:"idempotencyKey"`
	Kind              string          `json:"kind"`
	Payload           json.RawMessage `json:"payload"`
	Status            string          `json:"status"`
	LeaseUntil        *time.Time      `json:"leaseUntil,omitempty"`
	Deadline          time.Time       `json:"deadline"`
	TimeoutMS         int64           `json:"timeoutMs"`
	CancelRequestedAt *time.Time      `json:"cancelRequestedAt,omitempty"`
	StartedAt         *time.Time      `json:"startedAt,omitempty"`
	DispatchCount     int             `json:"dispatchCount"`
	Result            json.RawMessage `json:"result,omitempty"`
	ErrorCode         string          `json:"errorCode,omitempty"`
	ErrorMessage      string          `json:"errorMessage,omitempty"`
	ErrorRetryable    bool            `json:"errorRetryable"`
	CreatedAt         time.Time       `json:"createdAt"`
	CompletedAt       *time.Time      `json:"completedAt,omitempty"`
}

type AgentOperationalView struct {
	ID                  string     `json:"id"`
	Name                string     `json:"name"`
	Source              string     `json:"source"`
	ProtocolVersion     int        `json:"protocolVersion"`
	BuildVersion        string     `json:"buildVersion"`
	Capabilities        []string   `json:"capabilities"`
	Architecture        string     `json:"architecture"`
	OS                  string     `json:"os"`
	Generation          int        `json:"generation"`
	Enabled             bool       `json:"enabled"`
	RevokedAt           *time.Time `json:"revokedAt,omitempty"`
	FirstSeen           time.Time  `json:"firstSeen"`
	LastCheckin         time.Time  `json:"lastCheckin"`
	NextExpectedCheckin time.Time  `json:"nextExpectedCheckin"`
	Health              string     `json:"health"`
	IntervalMS          int64      `json:"intervalMs"`
	LastSequence        uint64     `json:"lastSequence"`
	LastCursor          uint64     `json:"lastCursor"`
}

type GatewayTaskErrorView struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

type GatewayTaskOperationalView struct {
	ID                string                `json:"id"`
	AgentID           string                `json:"agentId"`
	CorrelationID     string                `json:"correlationId"`
	Version           uint16                `json:"version"`
	IdempotencyKey    string                `json:"idempotencyKey"`
	Kind              string                `json:"kind"`
	Status            string                `json:"status"`
	LeaseUntil        *time.Time            `json:"leaseUntil,omitempty"`
	Deadline          time.Time             `json:"deadline"`
	TimeoutMS         int64                 `json:"timeoutMs"`
	CancelRequestedAt *time.Time            `json:"cancelRequestedAt,omitempty"`
	StartedAt         *time.Time            `json:"startedAt,omitempty"`
	DispatchCount     int                   `json:"dispatchCount"`
	Retries           int                   `json:"retries"`
	Error             *GatewayTaskErrorView `json:"error,omitempty"`
	CreatedAt         time.Time             `json:"createdAt"`
	CompletedAt       *time.Time            `json:"completedAt,omitempty"`
}

type EnrollmentTokenInfo struct {
	ID        string     `json:"id"`
	ExpiresAt time.Time  `json:"expiresAt"`
	UsedAt    *time.Time `json:"usedAt,omitempty"`
	CreatedBy string     `json:"createdBy"`
	CreatedAt time.Time  `json:"createdAt"`
}
