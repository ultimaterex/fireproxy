package auth

import (
	"sync"
	"time"
)

// IPRateLimiter is a simple sliding-window limiter (~N requests per window per key).
type IPRateLimiter struct {
	mu      sync.Mutex
	hits    map[string][]time.Time
	limit   int
	window  time.Duration
	cleanup time.Time
}

// NewIPRateLimiter creates a limiter allowing limit events per window.
func NewIPRateLimiter(limit int, window time.Duration) *IPRateLimiter {
	if limit <= 0 {
		limit = 10
	}
	if window <= 0 {
		window = time.Minute
	}
	return &IPRateLimiter{
		hits:   map[string][]time.Time{},
		limit:  limit,
		window: window,
	}
}

// Allow reports whether key may proceed and records the attempt when allowed.
func (l *IPRateLimiter) Allow(key string) bool {
	if l == nil {
		return true
	}
	if key == "" {
		key = "unknown"
	}
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	if now.After(l.cleanup) {
		l.pruneLocked(now)
		l.cleanup = now.Add(l.window)
	}

	cutoff := now.Add(-l.window)
	times := l.hits[key]
	n := 0
	for _, t := range times {
		if t.After(cutoff) {
			times[n] = t
			n++
		}
	}
	times = times[:n]
	if len(times) >= l.limit {
		l.hits[key] = times
		return false
	}
	l.hits[key] = append(times, now)
	return true
}

func (l *IPRateLimiter) pruneLocked(now time.Time) {
	cutoff := now.Add(-l.window)
	for k, times := range l.hits {
		n := 0
		for _, t := range times {
			if t.After(cutoff) {
				times[n] = t
				n++
			}
		}
		if n == 0 {
			delete(l.hits, k)
		} else {
			l.hits[k] = times[:n]
		}
	}
}
