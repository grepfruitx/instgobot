package cache

import (
	"context"
	"sync"
	"time"
)

type entry struct {
	val any
	exp time.Time
}

type Cache struct {
	mu sync.Mutex
	m  map[string]entry
}

func New(ctx context.Context) *Cache {
	c := &Cache{m: map[string]entry{}}
	go c.sweep(ctx, time.Minute)
	return c
}

func (c *Cache) sweep(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			c.mu.Lock()
			for k, e := range c.m {
				if !now.Before(e.exp) {
					delete(c.m, k)
				}
			}
			c.mu.Unlock()
		}
	}
}

func (c *Cache) live(key string, now time.Time) (entry, bool) {
	e, ok := c.m[key]
	if !ok || !now.Before(e.exp) {
		return entry{}, false
	}
	return e, true
}

func (c *Cache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.live(key, time.Now())
	return e.val, ok
}

func (c *Cache) Set(key string, val any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = entry{val: val, exp: time.Now().Add(ttl)}
}

func (c *Cache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, key)
}

func (c *Cache) TTL(key string) time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	e, ok := c.live(key, now)
	if !ok {
		return 0
	}
	return e.exp.Sub(now)
}

func (c *Cache) SetNX(key string, val any, ttl time.Duration) (bool, time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	if e, ok := c.live(key, now); ok {
		return false, e.exp.Sub(now)
	}
	c.m[key] = entry{val: val, exp: now.Add(ttl)}
	return true, ttl
}

func (c *Cache) Incr(key string, ttl time.Duration) (int, time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	e, ok := c.live(key, now)
	if !ok {
		e = entry{val: 0, exp: now.Add(ttl)}
	}
	n := e.val.(int) + 1
	c.m[key] = entry{val: n, exp: e.exp}
	return n, e.exp.Sub(now)
}
