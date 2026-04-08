package provider

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// FailoverProvider wraps a primary and failover Provider, automatically
// switching to the failover when the primary fails and attempting to fail
// back after a cooldown period.
//
// From the caller's perspective this is a single Provider — the failover
// logic is transparent.
type FailoverProvider struct {
	primary  Provider
	failover Provider

	// FailbackAfter is the cooldown before the primary is retried after a
	// failure. Default: 60 seconds.
	FailbackAfter time.Duration

	mu       sync.Mutex
	lastFail time.Time
}

// NewFailoverProvider returns a FailoverProvider that uses primary first and
// falls back to failover on error. If failbackAfter is zero it defaults to
// 60 seconds.
func NewFailoverProvider(primary, failover Provider, failbackAfter time.Duration) *FailoverProvider {
	if failbackAfter <= 0 {
		failbackAfter = 60 * time.Second
	}
	return &FailoverProvider{
		primary:       primary,
		failover:      failover,
		FailbackAfter: failbackAfter,
	}
}

// ID returns the primary provider's ID.
func (f *FailoverProvider) ID() string { return f.primary.ID() }

// Name returns a composite name indicating the failover relationship.
func (f *FailoverProvider) Name() string {
	return fmt.Sprintf("%s (failover: %s)", f.primary.Name(), f.failover.Name())
}

// Models returns the primary provider's model list.
func (f *FailoverProvider) Models(ctx context.Context) ([]Model, error) {
	return f.primary.Models(ctx)
}

// Stream attempts to stream from the primary provider. On error it records
// the failure time and retries with the failover provider. After
// FailbackAfter has elapsed since the last failure, the primary is tried
// again.
func (f *FailoverProvider) Stream(ctx context.Context, req *StreamRequest) (<-chan StreamEvent, error) {
	if f.shouldUsePrimary() {
		ch, err := f.primary.Stream(ctx, req)
		if err == nil {
			return ch, nil
		}
		f.recordFailure()
		// Fall through to failover.
	}

	ch, err := f.failover.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("both primary (%s) and failover (%s) failed: %w",
			f.primary.ID(), f.failover.ID(), err)
	}
	return ch, nil
}

func (f *FailoverProvider) shouldUsePrimary() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.lastFail.IsZero() {
		return true
	}
	return time.Since(f.lastFail) >= f.FailbackAfter
}

func (f *FailoverProvider) recordFailure() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastFail = time.Now()
}

// ResetFailure clears the failure state, causing the next request to try
// the primary provider immediately.
func (f *FailoverProvider) ResetFailure() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lastFail = time.Time{}
}
