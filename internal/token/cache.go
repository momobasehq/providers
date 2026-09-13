package token

import (
	"context"
	"sync"
	"time"
)

type Fetch func(context.Context) (string, time.Duration, error)

type Cache struct {
	mu      sync.Mutex
	value   string
	expires time.Time
}

func (c *Cache) Reset() { c.mu.Lock(); c.value = ""; c.expires = time.Time{}; c.mu.Unlock() }

func (c *Cache) Get(ctx context.Context, fetch Fetch) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.value != "" && time.Until(c.expires) > 30*time.Second {
		return c.value, nil
	}
	v, ttl, err := fetch(ctx)
	if err != nil {
		return "", err
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	c.value, c.expires = v, time.Now().Add(ttl)
	return v, nil
}
