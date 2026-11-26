package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestFixture(t *testing.T) {
	f, err := os.Open("../../tests/fixtures/lab-telemetry.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := analyze(f)
	if err != nil {
		t.Fatal(err)
	}
	if r.EventCount != 5 || len(r.Sessions) != 2 || len(r.Issues) != 2 {
		t.Fatalf("unexpected report: %+v", r)
	}
	a, b := r.Sessions[0], r.Sessions[1]
	if a.Session != "lab-a" || a.State != "closed" || a.Observations != 2 || a.ValidObservations != 1 || b.State != "unknown" {
		t.Fatalf("unexpected summaries: %+v", r.Sessions)
	}
	if r.Issues[0].EventID != "e3" || r.Issues[1].EventID != "e5" {
		t.Fatalf("unexpected issues: %+v", r.Issues)
	}
}

func TestEmpty(t *testing.T) {
	r, err := analyze(strings.NewReader(`{"schema":"lab-telemetry/v1","events":[]}`))
	if err != nil || r.EventCount != 0 || r.Sessions == nil || r.Issues == nil {
		t.Fatalf("empty report: %+v, %v", r, err)
	}
}

func TestRejectInvalidInput(t *testing.T) {
	const sample = `{"id":"e1","session":"a","at":"2026-01-01T00:00:00Z","kind":"opened"}`
	for name, input := range map[string]string{
		"duplicate":         `{"schema":"lab-telemetry/v1","events":[` + sample + `,` + sample + `]}`,
		"unknown field":     `{"schema":"lab-telemetry/v1","events":[],"extra":1}`,
		"unknown kind":      `{"schema":"lab-telemetry/v1","events":[` + strings.Replace(sample, "opened", "other", 1) + `]}`,
		"trailing":          `{"schema":"lab-telemetry/v1","events":[]} {}`,
		"null events":       `{"schema":"lab-telemetry/v1","events":null}`,
		"schema":            `{"schema":"other","events":[]}`,
		"invalid timestamp": `{"schema":"lab-telemetry/v1","events":[` + strings.Replace(sample, "2026-01-01T00:00:00Z", "bad", 1) + `]}`,
		"invalid id":        `{"schema":"lab-telemetry/v1","events":[` + strings.Replace(sample, "e1", "../x", 1) + `]}`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := analyze(strings.NewReader(input)); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func TestBounds(t *testing.T) {
	if _, err := analyze(strings.NewReader(strings.Repeat(" ", maxInput+1))); err == nil {
		t.Fatal("expected input limit")
	}
	input := `{"schema":"lab-telemetry/v1","events":[` + strings.Repeat(`{},`, maxEvents) + `{}]}`
	if _, err := analyze(strings.NewReader(input)); err == nil || !strings.Contains(err.Error(), "event count") {
		t.Fatalf("expected event count limit: %v", err)
	}
}

func TestCLI(t *testing.T) {
	var out, stderr bytes.Buffer
	if err := run(nil, strings.NewReader(`{"schema":"lab-telemetry/v1","events":[]}`), &out, &stderr); err != nil || !strings.Contains(out.String(), `"eventCount": 0`) {
		t.Fatalf("CLI: %v %s", err, out.String())
	}
	out.Reset()
	if err := run(nil, strings.NewReader(`not JSON`), &out, &stderr); err == nil || out.Len() != 0 {
		t.Fatal("bad input must not emit a report")
	}
}

func TestTransitionsAndTieOrder(t *testing.T) {
	input := `{"schema":"lab-telemetry/v1","events":[
	{"id":"1","session":"a","at":"2026-01-01T00:00:00Z","kind":"closed"},
	{"id":"2","session":"a","at":"2026-01-01T00:00:00Z","kind":"opened"},
	{"id":"3","session":"a","at":"2026-01-01T00:00:00Z","kind":"opened"},
	{"id":"4","session":"a","at":"2026-01-01T00:00:00Z","kind":"observed"},
	{"id":"5","session":"a","at":"2026-01-01T00:00:00Z","kind":"closed"},
	{"id":"6","session":"a","at":"2026-01-01T00:00:00Z","kind":"opened"}]}`
	r, err := analyze(strings.NewReader(input))
	if err != nil || len(r.Issues) != 3 || r.Sessions[0].State != "closed" || r.Sessions[0].ValidObservations != 1 {
		t.Fatalf("unexpected transition result: %+v %v", r, err)
	}
	if r.Issues[0].Code != "unexpected_close" || r.Issues[1].Code != "unexpected_open" || r.Issues[2].Code != "unexpected_open" {
		t.Fatalf("unexpected issue codes: %+v", r.Issues)
	}
}

func TestCleanFixture(t *testing.T) {
	f, err := os.Open("../../tests/fixtures/lab-telemetry-clean.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := analyze(f)
	if err != nil {
		t.Fatal(err)
	}
	if r.EventCount != 5 || len(r.Sessions) != 2 || len(r.Issues) != 0 {
		t.Fatalf("unexpected clean report: %+v", r)
	}
	if r.Sessions[0].State != "closed" || r.Sessions[1].State != "open" {
		t.Fatalf("unexpected states: %+v", r.Sessions)
	}
}
