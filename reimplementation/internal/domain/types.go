package domain

import "time"

type Account struct {
	ID       int64    `json:"-"`
	Username string   `json:"username"`
	Enabled  bool     `json:"enabled"`
	Roles    []string `json:"roles"`
}

type DocumentationEntry struct {
	Name     string               `json:"name"`
	Contents string               `json:"contents"`
	Children []DocumentationEntry `json:"children"`
}

type DocumentationSearchResult struct {
	Path    string  `json:"path"`
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float32 `json:"score"`
}

type Event struct {
	ID         int64     `json:"id"`
	Identifier string    `json:"identifier"`
	Timestamp  time.Time `json:"timestamp"`
	Type       string    `json:"type"`
	Source     string    `json:"source"`
	Outcome    string    `json:"outcome"`
	System     string    `json:"system"`
	Message    string    `json:"message"`
	Details    string    `json:"details"`
}

type EventEnvelope struct {
	ID         int64     `json:"id"`
	Type       string    `json:"type"`
	OccurredAt time.Time `json:"occurredAt"`
	Version    int       `json:"version"`
	Payload    Event     `json:"payload"`
}

func Envelope(event Event) EventEnvelope {
	return EventEnvelope{
		ID: event.ID, Type: "EventNotification", OccurredAt: event.Timestamp,
		Version: 1, Payload: event,
	}
}

type Setting struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
