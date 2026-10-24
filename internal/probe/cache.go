package probe

import (
	"sync"
	"time"
)

// cache keeps answers by address for a while.
type cache[T any] struct {
	mu    sync.Mutex
	items map[string]cached[T]
}

type cached[T any] struct {
	value T
	at    time.Time
}

func (c *cache[T]) get(key string) (T, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	item, ok := c.items[key]
	if !ok || time.Since(item.at) > cacheFor {
		var zero T
		return zero, false
	}
	return item.value, true
}

func (c *cache[T]) put(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		c.items = map[string]cached[T]{}
	}
	if len(c.items) >= cacheSize {
		for k, item := range c.items {
			if time.Since(item.at) > cacheFor || len(c.items) >= cacheSize {
				delete(c.items, k)
			}
		}
	}
	c.items[key] = cached[T]{value: value, at: time.Now()}
}
