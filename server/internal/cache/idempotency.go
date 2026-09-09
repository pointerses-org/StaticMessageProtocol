// Package cache provides the idempotency cache.
package cache

import (
	"fmt"
	"sync"
)

// IdempotencyCache is an LRU cache of (msgID, token) → response.
type IdempotencyCache struct {
	mu      sync.Mutex
	entries map[string][]byte
	order   []string
	maxSize int
}

// NewIdempotencyCache creates a cache with the given max size.
func NewIdempotencyCache(maxSize int) *IdempotencyCache {
	if maxSize <= 0 {
		maxSize = 10000
	}
	return &IdempotencyCache{
		entries: make(map[string][]byte, maxSize),
		order:   make([]string, 0, maxSize),
		maxSize: maxSize,
	}
}

// Key builds the cache key from message ID and token tail.
func (ic *IdempotencyCache) Key(msgID uint64, tokenTail []byte) string {
	return fmt.Sprintf("%x:%s", msgID, tokenTail)
}

// Get retrieves a cached value. Returns nil and false if not found.
func (ic *IdempotencyCache) Get(key string) ([]byte, bool) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	val, ok := ic.entries[key]
	if ok {
		ic.moveToFront(key)
	}
	return val, ok
}

// Put stores a value in the cache.
func (ic *IdempotencyCache) Put(key string, value []byte) {
	ic.mu.Lock()
	defer ic.mu.Unlock()
	if _, ok := ic.entries[key]; !ok {
		if len(ic.order) >= ic.maxSize {
			evictKey := ic.order[len(ic.order)-1]
			delete(ic.entries, evictKey)
			ic.order = ic.order[:len(ic.order)-1]
		}
		ic.order = append(ic.order, key)
	} else {
		ic.moveToFront(key)
	}
	ic.entries[key] = value
}

func (ic *IdempotencyCache) moveToFront(key string) {
	for i, k := range ic.order {
		if k == key {
			ic.order = append(ic.order[:i], ic.order[i+1:]...)
			break
		}
	}
	ic.order = append([]string{key}, ic.order...)
}
