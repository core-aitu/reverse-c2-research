package events

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"

	"github.com/research/specter-reimplementation/internal/domain"
)

type ticket struct {
	username string
	expires  time.Time
}

type Hub struct {
	mu      sync.RWMutex
	clients map[chan domain.EventEnvelope]struct{}
	tickets map[string]ticket
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[chan domain.EventEnvelope]struct{}),
		tickets: make(map[string]ticket),
	}
}

func (h *Hub) Ticket(username string) (string, time.Time, error) {
	buffer := make([]byte, 24)
	if _, err := rand.Read(buffer); err != nil {
		return "", time.Time{}, err
	}
	value := base64.RawURLEncoding.EncodeToString(buffer)
	expires := time.Now().UTC().Add(time.Minute)
	h.mu.Lock()
	h.tickets[value] = ticket{username: username, expires: expires}
	h.mu.Unlock()
	return value, expires, nil
}

func (h *Hub) ConsumeTicket(value string) (string, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	ticket, ok := h.tickets[value]
	delete(h.tickets, value)
	if !ok || time.Now().UTC().After(ticket.expires) {
		return "", false
	}
	return ticket.username, true
}

func (h *Hub) Subscribe() (<-chan domain.EventEnvelope, func()) {
	channel := make(chan domain.EventEnvelope, 64)
	h.mu.Lock()
	h.clients[channel] = struct{}{}
	h.mu.Unlock()
	return channel, func() {
		h.mu.Lock()
		if _, ok := h.clients[channel]; ok {
			delete(h.clients, channel)
			close(channel)
		}
		h.mu.Unlock()
	}
}

func (h *Hub) Publish(event domain.Event) {
	envelope := domain.Envelope(event)
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		select {
		case client <- envelope:
		default:
		}
	}
}
