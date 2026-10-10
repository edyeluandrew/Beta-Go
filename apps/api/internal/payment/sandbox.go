package payment

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// Provider starts a payment and reports what the provider currently says.
// Query errors must leave the payment unchanged. A missing answer is not a failure or a success.
type Provider interface {
	Initiate(ctx context.Context, orderID string) (reference string, err error)
	Query(ctx context.Context, reference string) (status string, err error)
}

// Sandbox is the local stand-in. It does not move money and it is not MTN or Airtel.
type Sandbox struct {
	mu     sync.Mutex
	result map[string]string
	calls  int
}

func NewSandbox() *Sandbox {
	return &Sandbox{result: map[string]string{}}
}

func (s *Sandbox) Initiate(_ context.Context, _ string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	reference := "sb_" + uuid.NewString()
	s.result[reference] = ProviderPending
	return reference, nil
}

func (s *Sandbox) Query(_ context.Context, reference string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	status, ok := s.result[reference]
	if !ok {
		return "", ErrNotFound
	}
	return status, nil
}

// SetResult stores the next answer Query will give. Tests use this in place of a provider console.
func (s *Sandbox) SetResult(reference, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.result[reference] = status
}

// Calls counts how many payments were started.
func (s *Sandbox) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}
