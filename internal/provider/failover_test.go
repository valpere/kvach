package provider

import (
	"context"
	"errors"
	"testing"
	"time"
)

// stubProvider is a minimal Provider for testing failover logic.
type stubProvider struct {
	id       string
	name     string
	streamFn func(ctx context.Context, req *StreamRequest) (<-chan StreamEvent, error)
}

func (s *stubProvider) ID() string   { return s.id }
func (s *stubProvider) Name() string { return s.name }
func (s *stubProvider) Models(_ context.Context) ([]Model, error) {
	return nil, nil
}
func (s *stubProvider) Stream(ctx context.Context, req *StreamRequest) (<-chan StreamEvent, error) {
	return s.streamFn(ctx, req)
}

func okStream() (<-chan StreamEvent, error) {
	ch := make(chan StreamEvent, 1)
	ch <- StreamEvent{Type: StreamEventMessageEnd, FinishReason: "stop"}
	close(ch)
	return ch, nil
}

func failStream() (<-chan StreamEvent, error) {
	return nil, errors.New("provider error")
}

func TestFailoverUsesPrimaryFirst(t *testing.T) {
	primary := &stubProvider{id: "primary", name: "Primary", streamFn: func(_ context.Context, _ *StreamRequest) (<-chan StreamEvent, error) {
		return okStream()
	}}
	failover := &stubProvider{id: "failover", name: "Failover", streamFn: func(_ context.Context, _ *StreamRequest) (<-chan StreamEvent, error) {
		t.Fatal("failover should not be called")
		return nil, nil
	}}

	fp := NewFailoverProvider(primary, failover, time.Second)
	ch, err := fp.Stream(context.Background(), &StreamRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Drain.
	for range ch {
	}
}

func TestFailoverSwitchesOnError(t *testing.T) {
	primary := &stubProvider{id: "primary", name: "Primary", streamFn: func(_ context.Context, _ *StreamRequest) (<-chan StreamEvent, error) {
		return failStream()
	}}
	failoverCalled := false
	failover := &stubProvider{id: "failover", name: "Failover", streamFn: func(_ context.Context, _ *StreamRequest) (<-chan StreamEvent, error) {
		failoverCalled = true
		return okStream()
	}}

	fp := NewFailoverProvider(primary, failover, time.Second)
	ch, err := fp.Stream(context.Background(), &StreamRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for range ch {
	}
	if !failoverCalled {
		t.Fatal("expected failover to be called")
	}
}

func TestFailoverBothFail(t *testing.T) {
	primary := &stubProvider{id: "primary", name: "Primary", streamFn: func(_ context.Context, _ *StreamRequest) (<-chan StreamEvent, error) {
		return failStream()
	}}
	failover := &stubProvider{id: "failover", name: "Failover", streamFn: func(_ context.Context, _ *StreamRequest) (<-chan StreamEvent, error) {
		return failStream()
	}}

	fp := NewFailoverProvider(primary, failover, time.Second)
	_, err := fp.Stream(context.Background(), &StreamRequest{})
	if err == nil {
		t.Fatal("expected error when both providers fail")
	}
}

func TestFailoverFailbackAfterCooldown(t *testing.T) {
	calls := []string{}

	primary := &stubProvider{id: "primary", name: "Primary", streamFn: func(_ context.Context, _ *StreamRequest) (<-chan StreamEvent, error) {
		calls = append(calls, "primary")
		// First call fails, then succeeds.
		if len(calls) == 1 {
			return failStream()
		}
		return okStream()
	}}
	failover := &stubProvider{id: "failover", name: "Failover", streamFn: func(_ context.Context, _ *StreamRequest) (<-chan StreamEvent, error) {
		calls = append(calls, "failover")
		return okStream()
	}}

	fp := NewFailoverProvider(primary, failover, 10*time.Millisecond)

	// First call: primary fails, failover succeeds.
	ch, err := fp.Stream(context.Background(), &StreamRequest{})
	if err != nil {
		t.Fatalf("call 1 error: %v", err)
	}
	for range ch {
	}

	// Second call immediately: should skip primary (cooldown not elapsed).
	ch, err = fp.Stream(context.Background(), &StreamRequest{})
	if err != nil {
		t.Fatalf("call 2 error: %v", err)
	}
	for range ch {
	}

	// Wait for cooldown.
	time.Sleep(20 * time.Millisecond)

	// Third call: should retry primary.
	ch, err = fp.Stream(context.Background(), &StreamRequest{})
	if err != nil {
		t.Fatalf("call 3 error: %v", err)
	}
	for range ch {
	}

	// Verify call pattern: primary, failover, failover, primary.
	expected := []string{"primary", "failover", "failover", "primary"}
	if len(calls) != len(expected) {
		t.Fatalf("expected calls %v, got %v", expected, calls)
	}
	for i, e := range expected {
		if calls[i] != e {
			t.Fatalf("call %d: expected %q, got %q", i, e, calls[i])
		}
	}
}

func TestFailoverResetFailure(t *testing.T) {
	primaryCalls := 0
	primary := &stubProvider{id: "primary", name: "Primary", streamFn: func(_ context.Context, _ *StreamRequest) (<-chan StreamEvent, error) {
		primaryCalls++
		if primaryCalls == 1 {
			return failStream()
		}
		return okStream()
	}}
	failover := &stubProvider{id: "failover", name: "Failover", streamFn: func(_ context.Context, _ *StreamRequest) (<-chan StreamEvent, error) {
		return okStream()
	}}

	fp := NewFailoverProvider(primary, failover, time.Hour) // very long cooldown

	// Trigger failure.
	ch, _ := fp.Stream(context.Background(), &StreamRequest{})
	for range ch {
	}

	// Reset.
	fp.ResetFailure()

	// Should try primary again immediately.
	ch, _ = fp.Stream(context.Background(), &StreamRequest{})
	for range ch {
	}

	if primaryCalls != 2 {
		t.Fatalf("expected 2 primary calls after reset, got %d", primaryCalls)
	}
}
