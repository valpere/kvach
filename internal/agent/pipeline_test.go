package agent

import (
	"context"
	"testing"
)

func TestPipelineFirstHandlerWins(t *testing.T) {
	h1 := func(_ context.Context, s string) (string, bool) {
		if s == "hello" {
			return "h1", true
		}
		return "", false
	}
	h2 := func(_ context.Context, _ string) (string, bool) {
		return "h2", true
	}

	p := NewPipeline(h1, h2)

	// h1 handles "hello".
	result, ok := p.Run(context.Background(), "hello")
	if !ok || result != "h1" {
		t.Fatalf("expected h1, got %q (ok=%v)", result, ok)
	}

	// h1 passes, h2 handles everything else.
	result, ok = p.Run(context.Background(), "other")
	if !ok || result != "h2" {
		t.Fatalf("expected h2, got %q (ok=%v)", result, ok)
	}
}

func TestPipelineNoHandler(t *testing.T) {
	h := func(_ context.Context, _ int) (string, bool) {
		return "", false
	}

	p := NewPipeline(h)
	_, ok := p.Run(context.Background(), 42)
	if ok {
		t.Fatal("expected no handler to match")
	}
}

func TestPipelineEmpty(t *testing.T) {
	p := NewPipeline[string, string]()
	_, ok := p.Run(context.Background(), "test")
	if ok {
		t.Fatal("expected false from empty pipeline")
	}
}

func TestPipelineAppendPrepend(t *testing.T) {
	var order []int

	make_handler := func(id int, handle bool) Handler[string, int] {
		return func(_ context.Context, _ string) (int, bool) {
			order = append(order, id)
			return id, handle
		}
	}

	p := NewPipeline(make_handler(2, false))
	p.Prepend(make_handler(1, false))
	p.Append(make_handler(3, true))

	result, ok := p.Run(context.Background(), "x")
	if !ok || result != 3 {
		t.Fatalf("expected handler 3, got %d (ok=%v)", result, ok)
	}
	if len(order) != 3 || order[0] != 1 || order[1] != 2 || order[2] != 3 {
		t.Fatalf("expected execution order [1,2,3], got %v", order)
	}
}

func TestPipelineLen(t *testing.T) {
	p := NewPipeline[int, int]()
	if p.Len() != 0 {
		t.Fatalf("expected 0, got %d", p.Len())
	}
	p.Append(func(_ context.Context, _ int) (int, bool) { return 0, false })
	if p.Len() != 1 {
		t.Fatalf("expected 1, got %d", p.Len())
	}
}
