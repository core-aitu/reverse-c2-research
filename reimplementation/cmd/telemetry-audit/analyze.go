package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"time"
)

const (
	maxInput  = 4 << 20
	maxEvents = 20000
)

type event struct {
	ID      string    `json:"id"`
	Session string    `json:"session"`
	At      time.Time `json:"at"`
	Kind    string    `json:"kind"`
}

type eventFile struct {
	Schema string  `json:"schema"`
	Events []event `json:"events"`
}

type issue struct {
	EventID string `json:"eventId"`
	Session string `json:"session"`
	Code    string `json:"code"`
}

type sessionSummary struct {
	Session           string    `json:"session"`
	State             string    `json:"state"`
	FirstEvent        time.Time `json:"firstEvent"`
	LastEvent         time.Time `json:"lastEvent"`
	Observations      int       `json:"observations"`
	ValidObservations int       `json:"validObservations"`
}

type auditReport struct {
	Schema     string           `json:"schema"`
	EventCount int              `json:"eventCount"`
	Sessions   []sessionSummary `json:"sessions"`
	Issues     []issue          `json:"issues"`
}

func validID(s string) bool {
	if len(s) == 0 || len(s) > 80 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func analyze(reader io.Reader) (auditReport, error) {
	var report auditReport
	data, err := io.ReadAll(io.LimitReader(reader, maxInput+1))
	if err != nil {
		return report, err
	}
	if len(data) > maxInput {
		return report, fmt.Errorf("input exceeds %d bytes", maxInput)
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var file eventFile
	if err := dec.Decode(&file); err != nil {
		return report, fmt.Errorf("invalid event file: %w", err)
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return report, fmt.Errorf("expected exactly one JSON document")
	}
	if file.Schema != "lab-telemetry/v1" || file.Events == nil {
		return report, fmt.Errorf("expected lab-telemetry/v1 with an events array")
	}
	if len(file.Events) > maxEvents {
		return report, fmt.Errorf("event count exceeds %d", maxEvents)
	}
	seen := make(map[string]bool, len(file.Events))
	for i, e := range file.Events {
		if !validID(e.ID) || !validID(e.Session) || e.At.IsZero() {
			return report, fmt.Errorf("event %d: invalid id, session or timestamp", i)
		}
		if seen[e.ID] {
			return report, fmt.Errorf("duplicate event id %q", e.ID)
		}
		seen[e.ID] = true
		switch e.Kind {
		case "opened", "observed", "closed":
		default:
			return report, fmt.Errorf("event %d: unsupported event kind", i)
		}
		file.Events[i].At = e.At.UTC()
	}
	// Input order is the tie-breaker for records with identical timestamps.
	sort.SliceStable(file.Events, func(i, j int) bool { return file.Events[i].At.Before(file.Events[j].At) })
	report = auditReport{Schema: "lab-telemetry-audit/v1", EventCount: len(file.Events), Sessions: []sessionSummary{}, Issues: []issue{}}
	bySession := make(map[string]*sessionSummary)
	for _, e := range file.Events {
		s := bySession[e.Session]
		if s == nil {
			s = &sessionSummary{Session: e.Session, State: "unknown", FirstEvent: e.At}
			bySession[e.Session] = s
		}
		s.LastEvent = e.At
		var problem string
		switch e.Kind {
		case "opened":
			if s.State != "unknown" {
				problem = "unexpected_open"
			} else {
				s.State = "open"
			}
		case "observed":
			s.Observations++
			if s.State != "open" {
				problem = "observation_outside_open_session"
			} else {
				s.ValidObservations++
			}
		case "closed":
			if s.State != "open" {
				problem = "unexpected_close"
			} else {
				s.State = "closed"
			}
		}
		if problem != "" {
			report.Issues = append(report.Issues, issue{EventID: e.ID, Session: e.Session, Code: problem})
		}
	}
	for _, s := range bySession {
		report.Sessions = append(report.Sessions, *s)
	}
	sort.Slice(report.Sessions, func(i, j int) bool { return report.Sessions[i].Session < report.Sessions[j].Session })
	return report, nil
}
