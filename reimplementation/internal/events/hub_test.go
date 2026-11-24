package events

import (
	"testing"
	"time"

	"github.com/research/specter-reimplementation/internal/domain"
)

func TestTicketIsSingleUse(t *testing.T) {
	hub := NewHub()
	ticket, _, err := hub.Ticket("admin")
	if err != nil {
		t.Fatal(err)
	}
	if username, ok := hub.ConsumeTicket(ticket); !ok || username != "admin" {
		t.Fatal("ticket was not accepted")
	}
	if _, ok := hub.ConsumeTicket(ticket); ok {
		t.Fatal("ticket was accepted twice")
	}
}

func TestPublishEnvelope(t *testing.T) {
	hub := NewHub()
	stream, unsubscribe := hub.Subscribe()
	defer unsubscribe()
	hub.Publish(domain.Event{ID: 9, Timestamp: time.Unix(10, 0).UTC(), Message: "fixture-live-window"})
	select {
	case envelope := <-stream:
		if envelope.ID != 9 || envelope.Version != 1 || envelope.Payload.Message != "fixture-live-window" {
			t.Fatalf("unexpected envelope: %#v", envelope)
		}
	case <-time.After(time.Second):
		t.Fatal("event was not published")
	}
}
