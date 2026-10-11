package payment

import (
	"context"

	"github.com/google/uuid"
)

type announcer interface {
	Send(ctx context.Context, title, body, href string, users []uuid.UUID, vendorID uuid.UUID, roles []string)
}

// SetAnnouncer receives live notes. Tests leave this unset.
func (s *Service) SetAnnouncer(notes announcer) {
	s.notes = notes
}

func (s *Service) notePaid(ctx context.Context, payment Payment) {
	if s.notes == nil {
		return
	}
	placed, err := s.orders.Get(ctx, payment.OrderID)
	if err != nil {
		return
	}
	id := placed.ID.String()
	s.notes.Send(ctx, "Payment received", "The kitchen can see your order now.", "/c/orders/"+id, []uuid.UUID{placed.CustomerID}, uuid.Nil, nil)
	s.notes.Send(ctx, "Paid order", "A customer paid. Accept it before the window closes.", "/v/orders/"+id, nil, placed.VendorID, nil)
	s.notes.Send(ctx, "Paid order waiting", "A kitchen has a paid order to accept.", "/a", nil, uuid.Nil, []string{"support", "super_admin"})
}
