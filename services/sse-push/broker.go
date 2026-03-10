package main

import "sync"

// SSEEvent represents a server-sent event with a named type and JSON payload.
type SSEEvent struct {
	Name string
	Data []byte
}

// Broker fans out SSEEvents to all connected SSE clients.
type Broker struct {
	mu      sync.RWMutex
	clients map[chan SSEEvent]struct{}
}

// NewBroker creates a new Broker ready for use.
func NewBroker() *Broker {
	return &Broker{
		clients: make(map[chan SSEEvent]struct{}),
	}
}

// Subscribe creates a buffered channel and registers it for broadcasts.
func (b *Broker) Subscribe() chan SSEEvent {
	ch := make(chan SSEEvent, 64)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a client channel from the broker and closes it.
func (b *Broker) Unsubscribe(ch chan SSEEvent) {
	b.mu.Lock()
	delete(b.clients, ch)
	close(ch)
	b.mu.Unlock()
}

// Broadcast sends an event to all registered clients.
// Slow clients whose buffers are full will have the message dropped.
func (b *Broker) Broadcast(event SSEEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.clients {
		select {
		case ch <- event:
		default:
			// drop message for slow client
		}
	}
}

// ClientCount returns the number of currently connected clients.
func (b *Broker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
