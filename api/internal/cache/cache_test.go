// api/internal/cache/cache_test.go
package cache_test

import (
	"testing"
	"time"

	"github.com/teclara/sixrail/api/internal/cache"
)

func TestCache_SetAndGet(t *testing.T) {
	c := cache.New()
	c.Set("key1", []byte(`{"data":"hello"}`), 5*time.Second)

	val, ok := c.Get("key1")
	if !ok {
		t.Fatal("expected key1 to exist")
	}
	if string(val) != `{"data":"hello"}` {
		t.Fatalf("expected hello, got %s", string(val))
	}
}

func TestCache_Expiry(t *testing.T) {
	c := cache.New()
	c.Set("key2", []byte(`expired`), 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get("key2")
	if ok {
		t.Fatal("expected key2 to be expired")
	}
}

func TestCache_GetStale(t *testing.T) {
	c := cache.New()
	c.Set("key3", []byte(`stale`), 1*time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	val, ok := c.GetStale("key3")
	if !ok {
		t.Fatal("expected key3 to exist as stale")
	}
	if string(val) != `stale` {
		t.Fatalf("expected stale, got %s", string(val))
	}
}

func TestCache_Miss(t *testing.T) {
	c := cache.New()
	_, ok := c.Get("nonexistent")
	if ok {
		t.Fatal("expected miss for nonexistent key")
	}
}
