package auth

import (
	"testing"
	"time"
)

func TestLimiterBlocksAfterTheBudget(t *testing.T) {
	limiter := NewLimiter(2, time.Minute)
	if !limiter.Allow("1.2.3.4") || !limiter.Allow("1.2.3.4") {
		t.Fatal("expected the first two attempts to pass")
	}
	if limiter.Allow("1.2.3.4") {
		t.Fatal("expected the third attempt to be blocked")
	}
	if !limiter.Allow("5.6.7.8") {
		t.Fatal("expected a different address to have its own budget")
	}
}
