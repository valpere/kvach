// Package bus implements an in-process publish/subscribe event bus.
//
// The bus decouples producers (the agent loop, tool dispatcher, permission
// system) from consumers (TUI renderer, HTTP SSE broadcaster, test
// assertions). Each subscriber receives its own channel and unsubscribes by
// calling the returned cancel function.
//
// All methods are safe for concurrent use.
package bus

import "sync"

// Event is the unit carried by the bus. The Payload field's concrete type
// depends on Type; see the EventType constants for documentation.
type Event struct {
	Type    string
	Payload any
}

// Bus is a fanout publish/subscribe hub.
type Bus struct {
	mu   sync.RWMutex
	subs map[uint64]subscriber
	next uint64
}

type subscriber struct {
	filter func(Event) bool
	ch     chan Event
}

// New returns an empty Bus.
func New() *Bus {
	return &Bus{subs: make(map[uint64]subscriber)}
}

// Publish sends e to every subscriber whose filter returns true.
// Subscribers that are not ready to receive are skipped (non-blocking send).
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for _, s := range b.subs {
		if s.filter == nil || s.filter(e) {
			select {
			case s.ch <- e:
			default:
			}
		}
	}
}

// Subscribe registers a new subscriber. filter may be nil to receive all
// events. Returns a channel and a cancel function; the caller must call cancel
// when it is done to release resources. The channel is buffered (64 events).
func (b *Bus) Subscribe(filter func(Event) bool) (<-chan Event, func()) {
	b.mu.Lock()
	id := b.next
	b.next++
	ch := make(chan Event, 64)
	b.subs[id] = subscriber{filter: filter, ch: ch}
	b.mu.Unlock()

	cancel := func() {
		b.mu.Lock()
		delete(b.subs, id)
		b.mu.Unlock()
		close(ch)
	}
	return ch, cancel
}
