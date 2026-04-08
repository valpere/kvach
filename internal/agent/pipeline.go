package agent

import "context"

// Handler processes an input and returns either a result (when it handled
// the input) or signals that it did not handle it by returning false.
//
// This is used for chain-of-responsibility patterns: user input
// preprocessing (slash commands, @-mentions), output post-processing (safety
// filters), and event routing.
type Handler[I, O any] func(ctx context.Context, input I) (O, bool)

// Pipeline executes an ordered list of handlers. The first handler that
// returns (result, true) wins — subsequent handlers are skipped.
type Pipeline[I, O any] struct {
	handlers []Handler[I, O]
}

// NewPipeline returns a Pipeline with the given handlers in execution order.
// Order is load-bearing: earlier handlers take priority.
func NewPipeline[I, O any](handlers ...Handler[I, O]) *Pipeline[I, O] {
	return &Pipeline[I, O]{handlers: handlers}
}

// Run passes input through each handler in order. Returns (result, true) from
// the first handler that handles the input, or (zero, false) if no handler
// matched.
func (p *Pipeline[I, O]) Run(ctx context.Context, input I) (O, bool) {
	for _, h := range p.handlers {
		if result, handled := h(ctx, input); handled {
			return result, true
		}
	}
	var zero O
	return zero, false
}

// Append adds handlers to the end of the pipeline (lowest priority).
func (p *Pipeline[I, O]) Append(handlers ...Handler[I, O]) {
	p.handlers = append(p.handlers, handlers...)
}

// Prepend adds handlers to the front of the pipeline (highest priority).
func (p *Pipeline[I, O]) Prepend(handlers ...Handler[I, O]) {
	p.handlers = append(handlers, p.handlers...)
}

// Len returns the number of handlers in the pipeline.
func (p *Pipeline[I, O]) Len() int {
	return len(p.handlers)
}
