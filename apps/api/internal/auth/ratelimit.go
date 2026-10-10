package auth

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"beta-go/api/internal/platform/httpx"
)

// Limiter allows a fixed number of hits per key inside a window, on this process only.
type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	hits   map[string][]time.Time
}

func NewLimiter(limit int, window time.Duration) *Limiter {
	return &Limiter{
		limit:  limit,
		window: window,
		hits:   map[string][]time.Time{},
	}
}

// Allow reports whether this key may proceed, and records the hit when it may.
func (l *Limiter) Allow(key string) bool {
	now := time.Now()
	cutoff := now.Add(-l.window)

	l.mu.Lock()
	defer l.mu.Unlock()

	existing := l.hits[key]
	recent := make([]time.Time, 0, len(existing)+1)
	for _, hit := range existing {
		if hit.After(cutoff) {
			recent = append(recent, hit)
		}
	}
	if len(recent) >= l.limit {
		l.hits[key] = recent
		return false
	}
	l.hits[key] = append(recent, now)
	if len(l.hits) > 10000 {
		l.hits = map[string][]time.Time{key: l.hits[key]}
	}
	return true
}

// Limit wraps a route and rejects extra attempts.
func (l *Limiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.Allow(c.ClientIP()) {
			httpx.Error(c, http.StatusTooManyRequests, "rate_limited", "Too many sign-in attempts. Wait a minute and try again.")
			c.Abort()
			return
		}
		c.Next()
	}
}
