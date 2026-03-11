package bus

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPubSub(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	nc, err := Connect("nats://localhost:4222")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer nc.Close()

	type msg struct {
		Hello string `json:"hello"`
	}

	received := make(chan msg, 1)

	_, err = Subscribe(nc, "test.subject", func(data []byte) {
		var m msg
		if err := json.Unmarshal(data, &m); err != nil {
			t.Errorf("unmarshal: %v", err)
			return
		}
		received <- m
	})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	sent := msg{Hello: "world"}
	if err := Publish(nc, "test.subject", sent); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case got := <-received:
		if got != sent {
			t.Fatalf("got %+v, want %+v", got, sent)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}
