package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Connect creates a Redis client and verifies connectivity with a ping.
func Connect(addr, password string) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis ping: %w", err)
	}
	return client, nil
}

// SetJSON marshals v to JSON and stores it under key with the given TTL.
func SetJSON(ctx context.Context, client *redis.Client, key string, v any, ttl time.Duration) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	return client.Set(ctx, key, data, ttl).Err()
}

// GetJSON retrieves the value at key and unmarshals it into dest.
func GetJSON(ctx context.Context, client *redis.Client, key string, dest any) error {
	data, err := client.Get(ctx, key).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// SetHashJSON stores each item in items as a JSON-encoded hash field, then sets
// an expiration on the key. Uses a pipeline for atomicity.
func SetHashJSON[V any](ctx context.Context, client *redis.Client, key string, items map[string]V, ttl time.Duration) error {
	pipe := client.Pipeline()
	for field, val := range items {
		data, err := json.Marshal(val)
		if err != nil {
			return fmt.Errorf("marshal field %q: %w", field, err)
		}
		pipe.HSet(ctx, key, field, data)
	}
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// GetHashFieldJSON retrieves a single hash field and unmarshals it into dest.
func GetHashFieldJSON(ctx context.Context, client *redis.Client, key, field string, dest any) error {
	data, err := client.HGet(ctx, key, field).Bytes()
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

// GetHashAllJSON retrieves all fields from a hash and unmarshals each value.
func GetHashAllJSON[V any](ctx context.Context, client *redis.Client, key string) (map[string]V, error) {
	raw, err := client.HGetAll(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	result := make(map[string]V, len(raw))
	for field, data := range raw {
		var v V
		if err := json.Unmarshal([]byte(data), &v); err != nil {
			return nil, fmt.Errorf("unmarshal field %q: %w", field, err)
		}
		result[field] = v
	}
	return result, nil
}

// SetMembers replaces the set at key with the given members and sets a TTL.
func SetMembers(ctx context.Context, client *redis.Client, key string, members []string, ttl time.Duration) error {
	pipe := client.Pipeline()
	pipe.Del(ctx, key)
	if len(members) > 0 {
		vals := make([]any, len(members))
		for i, m := range members {
			vals[i] = m
		}
		pipe.SAdd(ctx, key, vals...)
	}
	pipe.Expire(ctx, key, ttl)
	_, err := pipe.Exec(ctx)
	return err
}

// IsMember checks whether member belongs to the set stored at key.
func IsMember(ctx context.Context, client *redis.Client, key, member string) (bool, error) {
	return client.SIsMember(ctx, key, member).Result()
}

// SetTimestamp stores the current Unix timestamp (seconds) at key with a TTL.
func SetTimestamp(ctx context.Context, client *redis.Client, key string, ttl time.Duration) error {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	return client.Set(ctx, key, ts, ttl).Err()
}

// GetAge retrieves the Unix timestamp stored at key and returns the elapsed
// duration since that timestamp. Returns redis.Nil if the key does not exist.
func GetAge(ctx context.Context, client *redis.Client, key string) (time.Duration, error) {
	val, err := client.Get(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	ts, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse timestamp: %w", err)
	}
	return time.Since(time.Unix(ts, 0)), nil
}
