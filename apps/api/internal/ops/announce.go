package ops

import (
	"context"

	"github.com/google/uuid"

	"beta-go/api/internal/order"
)

type announcer interface {
	Send(ctx context.Context, title, body, href string, users []uuid.UUID, vendorID uuid.UUID, roles []string)
}

// SetAnnouncer receives live notes. Tests leave this unset.
func (s *Service) SetAnnouncer(notes announcer) {
	s.notes = notes
}

func (s *Service) announce(ctx context.Context, title, body, href string, users []uuid.UUID, vendorID uuid.UUID, roles []string) {
	if s.notes == nil {
		return
	}
	s.notes.Send(ctx, title, body, href, users, vendorID, roles)
}

func (s *Service) tellCustomer(ctx context.Context, placed order.Order, title, body string) {
	s.announce(ctx, title, body, "/c/orders/"+placed.ID.String(), []uuid.UUID{placed.CustomerID}, uuid.Nil, nil)
}

func statusLine(status string) string {
	switch status {
	case order.StatusPreparing:
		return "The kitchen is cooking."
	case order.StatusReady:
		return "Your order is ready."
	case order.StatusFulfilled:
		return "The kitchen marked this handed over."
	default:
		return "Your order moved forward."
	}
}
