package chat

import (
	"sync"
	"time"
)

// breaker is a per-target circuit breaker keyed by "provider/model".
//
// It exists because a provider that keeps losing the first-chunk probe is not
// free: every request that reaches its slot in the rotation pays the full probe
// budget before falling through to the next one. With a local model that needs
// 15s to warm up, that is one visitor in three waiting 15s for nothing.
//
// After `threshold` consecutive failures a target is skipped for `cooldown`.
// Any success resets it, and requests cancelled by the caller are not counted —
// those say nothing about the provider's health.
type breaker struct {
	mu        sync.Mutex
	failures  map[string]int
	openUntil map[string]time.Time

	threshold int
	cooldown  time.Duration

	// now is swappable in tests.
	now func() time.Time
}

func newBreaker(threshold int, cooldown time.Duration) *breaker {
	return &breaker{
		failures:  map[string]int{},
		openUntil: map[string]time.Time{},
		threshold: threshold,
		cooldown:  cooldown,
		now:       time.Now,
	}
}

// enabled reports whether the breaker is configured to trip at all.
func (b *breaker) enabled() bool {
	return b != nil && b.threshold > 0 && b.cooldown > 0
}

// isOpen reports whether the target is currently in cooldown.
func (b *breaker) isOpen(id string) bool {
	if !b.enabled() {
		return false
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	until, ok := b.openUntil[id]
	if !ok {
		return false
	}
	if b.now().Before(until) {
		return true
	}
	// Cooldown elapsed: let the next request through as the trial call.
	delete(b.openUntil, id)
	b.failures[id] = 0
	return false
}

// fail records a failed attempt and returns the cooldown deadline if this
// failure tripped the breaker (zero time otherwise).
func (b *breaker) fail(id string) time.Time {
	if !b.enabled() {
		return time.Time{}
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	b.failures[id]++
	if b.failures[id] < b.threshold {
		return time.Time{}
	}
	until := b.now().Add(b.cooldown)
	b.openUntil[id] = until
	return until
}

// succeed clears the failure count for a target.
func (b *breaker) succeed(id string) {
	if !b.enabled() {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.failures, id)
	delete(b.openUntil, id)
}
