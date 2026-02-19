package cache

import (
	"sync"
	"time"
)

type entry[T any] struct {
	value     T
	expiresAt time.Time
}

type Cache[T any] struct {
	entries map[string]*entry[T]
	ttl     time.Duration
	mu      sync.RWMutex
	hits    int64
	misses  int64
}

func New[T any](ttl time.Duration) *Cache[T] {
	c := &Cache[T]{
		entries: make(map[string]*entry[T]),
		ttl:     ttl,
	}
	go c.cleanup()
	return c
}

func (c *Cache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var zero T
	e, exists := c.entries[key]
	if !exists {
		c.misses++
		return zero, false
	}

	if time.Now().After(e.expiresAt) {
		c.misses++
		return zero, false
	}

	c.hits++
	return e.value, true
}

func (c *Cache[T]) Set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[key] = &entry[T]{
		value:     value,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *Cache[T]) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, key)
}

func (c *Cache[T]) DeleteByPrefix(prefix string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for key := range c.entries {
		if len(key) >= len(prefix) && key[:len(prefix)] == prefix {
			delete(c.entries, key)
		}
	}
}

func (c *Cache[T]) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries = make(map[string]*entry[T])
}

func (c *Cache[T]) Stats() (hits, misses int64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses
}

func (c *Cache[T]) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

func (c *Cache[T]) cleanup() {
	ticker := time.NewTicker(time.Minute)
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, e := range c.entries {
			if now.After(e.expiresAt) {
				delete(c.entries, key)
			}
		}
		c.mu.Unlock()
	}
}

type Service struct {
	User         *Cache[interface{}]
	School       *Cache[interface{}]
	Theme        *Cache[interface{}]
	Subjects     *Cache[interface{}]
	SchoolSearch *Cache[interface{}]
}

func NewService(userTTL, schoolTTL, themeTTL, subjectsTTL, searchTTL time.Duration) *Service {
	return &Service{
		User:         New[interface{}](userTTL),
		School:       New[interface{}](schoolTTL),
		Theme:        New[interface{}](themeTTL),
		Subjects:     New[interface{}](subjectsTTL),
		SchoolSearch: New[interface{}](searchTTL),
	}
}
