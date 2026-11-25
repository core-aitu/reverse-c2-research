package domain

import (
	"encoding/json"
	"time"
)

type AiAgentDefinition struct {
	ID                    int64    `json:"id"`
	AgentID               string   `json:"agentId"`
	Name                  string   `json:"name"`
	Description           string   `json:"description"`
	Prompt                string   `json:"prompt"`
	IncludeSpecterScripts bool     `json:"includeSpecterScripts"`
	AccessibleFunctions   []string `json:"accessibleFunctions"`
}

type AiFunctionDefinition struct {
	FunctionID    string `json:"functionId"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Documentation string `json:"documentation"`
	ApprovalLevel string `json:"approvalLevel"`
	Definition    string `json:"definition"`
}

type RunAiFunctionResult struct {
	Succeeded bool   `json:"succeeded"`
	Output    string `json:"output"`
}

type AiSessionInfo struct {
	Name               string              `json:"name"`
	Agents             []AiAgentDefinition `json:"agents"`
	ModelType          string              `json:"modelType"`
	ModelName          string              `json:"modelName"`
	ToolApprovalPolicy string              `json:"toolApprovalPolicy"`
	Plan               json.RawMessage     `json:"plan"`
	Status             string              `json:"status"`
}

type AiSessionInfoWithSessionID struct {
	SessionID string `json:"sessionId"`
	AiSessionInfo
}

type AiModelList struct {
	Kind   string   `json:"kind"`
	Models []string `json:"models"`
}

type AiMessage struct {
	SessionID      string     `json:"sessionId"`
	MessageID      string     `json:"messageId"`
	TimeCreated    time.Time  `json:"timeCreated"`
	TimeCompleted  *time.Time `json:"timeCompleted"`
	SenderName     string     `json:"senderName"`
	SenderType     string     `json:"senderType"`
	ReceiverName   string     `json:"receiverName"`
	Message        string     `json:"message"`
	SummarizedText string     `json:"summarizedText"`
	Status         string     `json:"status"`
}

type AiFunctionCallResult struct {
	SessionID     string    `json:"sessionId"`
	ToolCallID    string    `json:"toolCallId"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	ArgumentsJSON string    `json:"argumentsJson"`
	Output        string    `json:"output"`
	Error         string    `json:"error"`
	ExecutedUTC   time.Time `json:"executedUtc"`
	Status        string    `json:"status"`
}

type AiStatusUpdateMessage struct {
	SessionID  string `json:"sessionId"`
	SourceName string `json:"sourceName"`
	Message    string `json:"message"`
}
