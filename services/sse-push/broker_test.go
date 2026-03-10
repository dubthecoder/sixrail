package main

import (
	"testing"
	"time"
)

func TestBrokerSubscribeBroadcast(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	evt := SSEEvent{Name: "alerts", Data: []byte(`{"id":1}`)}
	b.Broadcast(evt)

	select {
	case got := <-ch:
		if got.Name != evt.Name {
			t.Errorf("got name %q, want %q", got.Name, evt.Name)
		}
		if string(got.Data) != string(evt.Data) {
			t.Errorf("got data %q, want %q", got.Data, evt.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestBrokerUnsubscribe(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	b.Unsubscribe(ch)

	// Broadcasting after unsubscribe should not panic.
	b.Broadcast(SSEEvent{Name: "alerts", Data: []byte(`{}`)})

	// Channel should be closed.
	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed")
	}
}

func TestBrokerSlowClient(t *testing.T) {
	b := NewBroker()
	ch := b.Subscribe()
	defer b.Unsubscribe(ch)

	// Fill the buffer (capacity 64).
	for i := 0; i < 64; i++ {
		b.Broadcast(SSEEvent{Name: "fill", Data: []byte(`{}`)})
	}

	// This broadcast should not block even though the buffer is full.
	done := make(chan struct{})
	go func() {
		b.Broadcast(SSEEvent{Name: "overflow", Data: []byte(`{}`)})
		close(done)
	}()

	select {
	case <-done:
		// success — broadcast did not block
	case <-time.After(time.Second):
		t.Fatal("Broadcast blocked on slow client")
	}
}

func TestBrokerClientCount(t *testing.T) {
	b := NewBroker()

	if got := b.ClientCount(); got != 0 {
		t.Fatalf("expected 0 clients, got %d", got)
	}

	ch1 := b.Subscribe()
	ch2 := b.Subscribe()

	if got := b.ClientCount(); got != 2 {
		t.Fatalf("expected 2 clients, got %d", got)
	}

	b.Unsubscribe(ch1)

	if got := b.ClientCount(); got != 1 {
		t.Fatalf("expected 1 client, got %d", got)
	}

	b.Unsubscribe(ch2)

	if got := b.ClientCount(); got != 0 {
		t.Fatalf("expected 0 clients, got %d", got)
	}
}
