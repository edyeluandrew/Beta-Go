package launch

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryBooks stores audit and backup checks for tests.
type MemoryBooks struct {
	mu      sync.Mutex
	entries []Entry
	backups []BackupCheck
}

func NewMemoryBooks() *MemoryBooks {
	return &MemoryBooks{}
}

func (m *MemoryBooks) Write(_ context.Context, actorID, entityID uuid.UUID, action, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = append([]Entry{{
		ID: uuid.New(), ActorID: actorID, Action: action, EntityID: entityID, Reason: reason, CreatedAt: time.Now().UTC(),
	}}, m.entries...)
	return nil
}

func (m *MemoryBooks) List(_ context.Context, limit int) ([]Entry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 || limit > len(m.entries) {
		limit = len(m.entries)
	}
	out := make([]Entry, limit)
	copy(out, m.entries[:limit])
	return out, nil
}

func (m *MemoryBooks) SaveBackup(_ context.Context, check BackupCheck) (BackupCheck, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if check.ID == uuid.Nil {
		check.ID = uuid.New()
	}
	if check.CreatedAt.IsZero() {
		check.CreatedAt = time.Now().UTC()
	}
	m.backups = append([]BackupCheck{check}, m.backups...)
	return check, nil
}

func (m *MemoryBooks) ListBackups(_ context.Context, limit int) ([]BackupCheck, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if limit <= 0 || limit > len(m.backups) {
		limit = len(m.backups)
	}
	out := make([]BackupCheck, limit)
	copy(out, m.backups[:limit])
	return out, nil
}
