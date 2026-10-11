package notify

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTicketIsSingleUse(t *testing.T) {
	bus := New(nil, "http://localhost:5173")
	user := uuid.New()
	raw := bus.issue(user)
	got, ok := bus.take(raw)
	if !ok || got != user {
		t.Fatalf("ticket = %s ok=%v", got, ok)
	}
	if _, ok := bus.take(raw); ok {
		t.Fatal("ticket worked twice")
	}
}

func TestTicketExpires(t *testing.T) {
	bus := New(nil, "http://localhost:5173")
	raw := bus.issue(uuid.New())
	bus.mu.Lock()
	item := bus.tickets[raw]
	item.until = time.Now().Add(-time.Second)
	bus.tickets[raw] = item
	bus.mu.Unlock()
	if _, ok := bus.take(raw); ok {
		t.Fatal("expired ticket was accepted")
	}
}
