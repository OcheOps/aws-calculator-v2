package auth

import (
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type Limiter struct {
	mu       sync.Mutex
	burst    int
	interval time.Duration
	buckets  map[string]*rate.Limiter
	lastSeen map[string]time.Time
}

func NewLimiter(burst int, interval time.Duration) *Limiter {
	return &Limiter{
		burst:    burst,
		interval: interval,
		buckets:  map[string]*rate.Limiter{},
		lastSeen: map[string]time.Time{},
	}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	b, ok := l.buckets[key]
	if !ok {
		b = rate.NewLimiter(rate.Every(l.interval/time.Duration(l.burst)), l.burst)
		l.buckets[key] = b
	}
	l.lastSeen[key] = time.Now()
	return b.Allow()
}
