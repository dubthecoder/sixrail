package bus

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go"
)

// Connect establishes a connection to a NATS server with automatic reconnection.
func Connect(url string) (*nats.Conn, error) {
	nc, err := nats.Connect(url,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
			slog.Warn("NATS disconnected", "error", err)
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			slog.Info("NATS reconnected", "url", nc.ConnectedUrl())
		}),
	)
	if err != nil {
		return nil, err
	}
	return nc, nil
}

// Publish marshals v as JSON and publishes it to the given subject.
func Publish(nc *nats.Conn, subject string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return nc.Publish(subject, data)
}

// Subscribe registers a handler that receives the raw message data for the given subject.
// Returns the subscription for cleanup if needed.
func Subscribe(nc *nats.Conn, subject string, handler func(data []byte)) (*nats.Subscription, error) {
	return nc.Subscribe(subject, func(msg *nats.Msg) {
		handler(msg.Data)
	})
}
