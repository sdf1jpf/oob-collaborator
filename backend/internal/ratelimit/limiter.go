package ratelimit

import (
	"sync"
	"time"
)

// IPLimiter enforces a per-key request cap within a sliding window.
type IPLimiter struct {
	mu      sync.Mutex
	max     int
	window  time.Duration
	entries map[string]*windowEntry
}

type windowEntry struct {
	count       int
	windowStart time.Time
}

func NewIPLimiter(max int, window time.Duration) *IPLimiter {
	return &IPLimiter{
		max:     max,
		window:  window,
		entries: make(map[string]*windowEntry),
	}
}

func (l *IPLimiter) Allow(key string) bool {
	if key == "" {
		key = "unknown"
	}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[key]
	if !ok || now.Sub(e.windowStart) >= l.window {
		l.entries[key] = &windowEntry{count: 1, windowStart: now}
		return true
	}
	if e.count >= l.max {
		return false
	}
	e.count++
	return true
}

// LockoutLimiter tracks failed attempts and enforces a temporary lockout.
type LockoutLimiter struct {
	mu           sync.Mutex
	maxFailures  int
	failureWindow time.Duration
	lockout      time.Duration
	entries      map[string]*lockoutEntry
}

type lockoutEntry struct {
	failures    int
	windowStart time.Time
	lockedUntil time.Time
}

func NewLockoutLimiter(maxFailures int, failureWindow, lockout time.Duration) *LockoutLimiter {
	return &LockoutLimiter{
		maxFailures:   maxFailures,
		failureWindow: failureWindow,
		lockout:       lockout,
		entries:       make(map[string]*lockoutEntry),
	}
}

func (l *LockoutLimiter) IsLocked(key string) bool {
	if key == "" {
		key = "unknown"
	}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[key]
	if !ok {
		return false
	}
	if now.Before(e.lockedUntil) {
		return true
	}
	if now.Sub(e.windowStart) >= l.failureWindow {
		delete(l.entries, key)
	}
	return false
}

func (l *LockoutLimiter) RecordFailure(key string) {
	if key == "" {
		key = "unknown"
	}
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.entries[key]
	if !ok || now.Sub(e.windowStart) >= l.failureWindow {
		l.entries[key] = &lockoutEntry{failures: 1, windowStart: now}
		return
	}
	e.failures++
	if e.failures >= l.maxFailures {
		e.lockedUntil = now.Add(l.lockout)
	}
}

func (l *LockoutLimiter) RecordSuccess(key string) {
	if key == "" {
		key = "unknown"
	}
	l.mu.Lock()
	delete(l.entries, key)
	l.mu.Unlock()
}
