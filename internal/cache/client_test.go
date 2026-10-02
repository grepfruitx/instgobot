package cache

import (
	"testing"
	"time"
)

func TestExpiredEntriesAreInvisible(t *testing.T) {
	c := New(t.Context())
	c.Set("k", "v", time.Millisecond)
	time.Sleep(5 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("expected expired key to miss")
	}
	if ttl := c.TTL("k"); ttl != 0 {
		t.Fatalf("expected zero ttl, got %v", ttl)
	}
	if ok, _ := c.SetNX("k", "v2", time.Minute); !ok {
		t.Fatal("expected SetNX to succeed over expired key")
	}
}

func TestSetNXKeepsExisting(t *testing.T) {
	c := New(t.Context())
	if ok, _ := c.SetNX("k", 1, time.Minute); !ok {
		t.Fatal("first SetNX should succeed")
	}
	ok, ttl := c.SetNX("k", 2, time.Minute)
	if ok || ttl <= 0 {
		t.Fatalf("second SetNX: ok=%v ttl=%v", ok, ttl)
	}
	if v, _ := c.Get("k"); v != 1 {
		t.Fatalf("value overwritten: %v", v)
	}
}

func TestIncrKeepsWindowFromFirstHit(t *testing.T) {
	c := New(t.Context())
	n, first := c.Incr("k", time.Minute)
	if n != 1 {
		t.Fatalf("n=%d", n)
	}
	time.Sleep(5 * time.Millisecond)
	n, second := c.Incr("k", time.Minute)
	if n != 2 || second >= first {
		t.Fatalf("n=%d first=%v second=%v", n, first, second)
	}
}

func TestSweepRemovesExpired(t *testing.T) {
	c := &Cache{m: map[string]entry{}}
	go c.sweep(t.Context(), time.Millisecond)
	c.Set("k", "v", time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	c.mu.Lock()
	n := len(c.m)
	c.mu.Unlock()
	if n != 0 {
		t.Fatalf("expected sweep to empty map, got %d", n)
	}
}
