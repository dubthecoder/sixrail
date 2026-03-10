package cache

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisClient returns a connected Redis client for integration tests.
// It reads REDIS_ADDR (default localhost:6379) and REDIS_PASSWORD from env.
func redisClient(t *testing.T) *redis.Client {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	client, err := Connect(addr, os.Getenv("REDIS_PASSWORD"))
	if err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestSetGetJSON(t *testing.T) {
	client := redisClient(t)
	ctx := context.Background()

	type sample struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	key := "test:cache:json:" + t.Name()
	t.Cleanup(func() { client.Del(ctx, key) })

	in := sample{Name: "hello", Value: 42}
	if err := SetJSON(ctx, client, key, in, 10*time.Second); err != nil {
		t.Fatalf("SetJSON: %v", err)
	}

	var out sample
	if err := GetJSON(ctx, client, key, &out); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if out != in {
		t.Errorf("got %+v, want %+v", out, in)
	}
}

func TestSetGetJSON_NotFound(t *testing.T) {
	client := redisClient(t)
	ctx := context.Background()

	var out string
	err := GetJSON(ctx, client, "test:cache:nonexistent", &out)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if err != redis.Nil {
		t.Errorf("expected redis.Nil, got %v", err)
	}
}

func TestSetGetHash(t *testing.T) {
	client := redisClient(t)
	ctx := context.Background()

	type item struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	key := "test:cache:hash:" + t.Name()
	t.Cleanup(func() { client.Del(ctx, key) })

	items := map[string]item{
		"a": {ID: 1, Name: "alpha"},
		"b": {ID: 2, Name: "beta"},
	}

	if err := SetHashJSON(ctx, client, key, items, 10*time.Second); err != nil {
		t.Fatalf("SetHashJSON: %v", err)
	}

	// GetHashFieldJSON
	var single item
	if err := GetHashFieldJSON(ctx, client, key, "a", &single); err != nil {
		t.Fatalf("GetHashFieldJSON: %v", err)
	}
	if single != items["a"] {
		t.Errorf("field 'a': got %+v, want %+v", single, items["a"])
	}

	// GetHashAllJSON
	all, err := GetHashAllJSON[item](ctx, client, key)
	if err != nil {
		t.Fatalf("GetHashAllJSON: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("got %d fields, want 2", len(all))
	}
	for k, want := range items {
		got, ok := all[k]
		if !ok {
			t.Errorf("missing field %q", k)
			continue
		}
		if got != want {
			t.Errorf("field %q: got %+v, want %+v", k, got, want)
		}
	}
}

func TestSetMembers_IsMember(t *testing.T) {
	client := redisClient(t)
	ctx := context.Background()

	key := "test:cache:set:" + t.Name()
	t.Cleanup(func() { client.Del(ctx, key) })

	if err := SetMembers(ctx, client, key, []string{"x", "y", "z"}, 10*time.Second); err != nil {
		t.Fatalf("SetMembers: %v", err)
	}

	ok, err := IsMember(ctx, client, key, "y")
	if err != nil {
		t.Fatalf("IsMember: %v", err)
	}
	if !ok {
		t.Error("expected 'y' to be a member")
	}

	ok, err = IsMember(ctx, client, key, "w")
	if err != nil {
		t.Fatalf("IsMember: %v", err)
	}
	if ok {
		t.Error("expected 'w' to not be a member")
	}
}

func TestSetTimestamp_GetAge(t *testing.T) {
	client := redisClient(t)
	ctx := context.Background()

	key := "test:cache:ts:" + t.Name()
	t.Cleanup(func() { client.Del(ctx, key) })

	if err := SetTimestamp(ctx, client, key, 10*time.Second); err != nil {
		t.Fatalf("SetTimestamp: %v", err)
	}

	age, err := GetAge(ctx, client, key)
	if err != nil {
		t.Fatalf("GetAge: %v", err)
	}
	if age < 0 || age > 2*time.Second {
		t.Errorf("age %v out of expected range [0, 2s]", age)
	}
}
