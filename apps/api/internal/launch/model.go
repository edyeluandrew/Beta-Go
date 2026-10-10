package launch

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// Entry is one append-only audit row. Posted rows are never edited.
type Entry struct {
	ID        uuid.UUID
	ActorID   uuid.UUID
	Action    string
	EntityID  uuid.UUID
	Reason    string
	CreatedAt time.Time
}

// BackupCheck is an admin confirming that Neon recovery still works.
type BackupCheck struct {
	ID        uuid.UUID
	ActorID   uuid.UUID
	Note      string
	CreatedAt time.Time
}

var (
	ErrNotFound  = errors.New("not found")
	ErrForbidden = errors.New("forbidden")
	ErrBadInput  = errors.New("bad input")
)
