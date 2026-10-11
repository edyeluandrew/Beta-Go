package order

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

func (s *Service) announce(ctx context.Context, title, body, href string, users []uuid.UUID, vendorID uuid.UUID, roles []string) {
	if s.notes == nil {
		return
	}
	s.notes.Send(ctx, title, body, href, users, vendorID, roles)
}
