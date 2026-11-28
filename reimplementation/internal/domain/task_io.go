package domain

import (
	"encoding/json"
	"time"
)

type TaskOutputChunk struct {
	ID        int64     `json:"id"`
	TaskID    string    `json:"taskId"`
	Stream    string    `json:"stream"`
	Index     int       `json:"index"`
	Content   []byte    `json:"-"`
	Text      string    `json:"text"`
	Encoding  string    `json:"encoding"`
	EOF       bool      `json:"eof"`
	Truncated bool      `json:"truncated"`
	CreatedAt time.Time `json:"createdAt"`
}

type TaskOutputObject struct {
	Stream    string     `json:"stream"`
	Object    ObjectInfo `json:"object"`
	ByteCount int64      `json:"byteCount"`
	Truncated bool       `json:"truncated"`
}

type GatewayTaskDetail struct {
	GatewayTaskOperationalView
	Payload json.RawMessage    `json:"payload"`
	Result  json.RawMessage    `json:"result,omitempty"`
	Outputs []TaskOutputObject `json:"outputs"`
}

type AgentFileTransfer struct {
	ID                 string          `json:"id"`
	TaskID             string          `json:"taskId"`
	AgentID            string          `json:"agentId"`
	Name               string          `json:"name"`
	SourcePath         string          `json:"sourcePath"`
	MediaType          string          `json:"mediaType"`
	ExpectedSize       int64           `json:"expectedSize"`
	ExpectedSHA256     string          `json:"expectedSha256"`
	ReceivedSize       int64           `json:"receivedSize"`
	NextChunk          int             `json:"nextChunk"`
	Status             string          `json:"status"`
	Object             *ObjectInfo     `json:"object,omitempty"`
	MultipartUploadID  string          `json:"-"`
	MultipartObjectKey string          `json:"-"`
	MultipartParts     []MultipartPart `json:"-"`
	CreatedAt          time.Time       `json:"createdAt"`
	CompletedAt        *time.Time      `json:"completedAt,omitempty"`
}

type MultipartPart struct {
	Number int    `json:"number"`
	ETag   string `json:"etag"`
	Size   int64  `json:"size"`
}
