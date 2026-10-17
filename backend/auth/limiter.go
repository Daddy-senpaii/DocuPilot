package auth

import (
	"sync"
	"time"
)

type ipRecord struct {
	timestamps []time.Time
}

// IPRateLimiter provides sliding-window rate limiting per client IP to mitigate
// brute force attacks and credential stuffing.
type IPRateLimiter struct {
	mu      sync.Mutex
	records map[string]*ipRecord
	stopCh  chan struct{}
}

// NewIPRateLimiter creates a rate limiter and starts a background cleanup ticker.
func NewIPRateLimiter(cleanupInterval time.Duration) *IPRateLimiter {
	if cleanupInterval <= 0 {
		cleanupInterval = 5 * time.Minute
	}

	limiter := &IPRateLimiter{
		records: make(map[string]*ipRecord),
		stopCh:  make(chan struct{}),
	}

	go limiter.cleanupLoop(cleanupInterval)
	return limiter
}

// Allow returns true if the given IP has not exceeded maxRequests within window.
func (r *IPRateLimiter) Allow(ip string, maxRequests int, window time.Duration) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-window)

	rec, exists := r.records[ip]
	if !exists {
		rec = &ipRecord{timestamps: make([]time.Time, 0, maxRequests+1)}
		r.records[ip] = rec
	}

	// Filter out expired timestamps
	validIdx := 0
	for i, ts := range rec.timestamps {
		if ts.After(cutoff) {
			validIdx = i
			break
		}
		if i == len(rec.timestamps)-1 {
			validIdx = len(rec.timestamps)
		}
	}
	rec.timestamps = rec.timestamps[validIdx:]

	if len(rec.timestamps) >= maxRequests {
		return false
	}

	rec.timestamps = append(rec.timestamps, now)
	return true
}

func (r *IPRateLimiter) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.purgeStale(15 * time.Minute)
		case <-r.stopCh:
			return
		}
	}
}

func (r *IPRateLimiter) purgeStale(maxAge time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	for ip, rec := range r.records {
		if len(rec.timestamps) == 0 || rec.timestamps[len(rec.timestamps)-1].Before(cutoff) {
			delete(r.records, ip)
		}
	}
}

// Close stops the background cleanup loop.
func (r *IPRateLimiter) Close() {
	select {
	case <-r.stopCh:
		// already closed
	default:
		close(r.stopCh)
	}
}
