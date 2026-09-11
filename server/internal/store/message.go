// Package store provides message and CFM storage.
package store

import (
	"sort"
	"sync"
	"time"
)

// ContextPair represents a single context reference.
type ContextPair struct {
	RefID     uint64
	Timestamp uint32
}

// StoredMessage is a message in the store.
type StoredMessage struct {
	ID        uint64
	Route     string
	UserData  []byte
	Context   []ContextPair
	TokenTail string
	Timestamp time.Time
}

// MessageStore is an in-memory message store with retention.
type MessageStore struct {
	mu        sync.RWMutex
	messages  map[string][]*StoredMessage
	retention time.Duration
}

// NewMessageStore creates a store with the given retention in minutes.
func NewMessageStore(mins int) *MessageStore {
	if mins <= 0 {
		mins = 10
	}
	return &MessageStore{
		messages:  make(map[string][]*StoredMessage),
		retention: time.Duration(mins) * time.Minute,
	}
}

// Store adds a message.
func (ms *MessageStore) Store(msg *StoredMessage) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.messages[msg.Route] = append(ms.messages[msg.Route], msg)
}

// Query returns messages matching the filter.
func (ms *MessageStore) Query(route string, limit, offset int, after, before uint32, msgID uint64) []*StoredMessage {
	ms.mu.RLock()
	defer ms.mu.RUnlock()

	var results []*StoredMessage
	for _, msg := range ms.messages[route] {
		if after > 0 && msg.Timestamp.Unix() < int64(after) {
			continue
		}
		if before > 0 && msg.Timestamp.Unix() > int64(before) {
			continue
		}
		if msgID != 0 && msg.ID != msgID {
			continue
		}
		results = append(results, msg)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.Before(results[j].Timestamp)
	})

	if offset >= len(results) {
		return nil
	}
	results = results[offset:]
	if limit > 0 && limit < len(results) {
		results = results[:limit]
	}
	return results
}

// GetByID finds a message by ID across all routes.
func (ms *MessageStore) GetByID(msgID uint64) *StoredMessage {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	for _, msgs := range ms.messages {
		for _, msg := range msgs {
			if msg.ID == msgID {
				return msg
			}
		}
	}
	return nil
}

// LatestID returns the ID of the newest message in route, or 0 when the inbox is
// empty. Newest means the most recent Timestamp; ties keep the earliest write.
func (ms *MessageStore) LatestID(route string) uint64 {
	ms.mu.RLock()
	defer ms.mu.RUnlock()
	var newest *StoredMessage
	for _, msg := range ms.messages[route] {
		if newest == nil || msg.Timestamp.After(newest.Timestamp) {
			newest = msg
		}
	}
	if newest == nil {
		return 0
	}
	return newest.ID
}

// Cleanup removes expired messages and returns the count removed.
func (ms *MessageStore) Cleanup() int {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	cutoff := time.Now().Add(-ms.retention)
	removed := 0
	for route, msgs := range ms.messages {
		filtered := msgs[:0]
		for _, msg := range msgs {
			if msg.Timestamp.After(cutoff) {
				filtered = append(filtered, msg)
			} else {
				removed++
			}
		}
		if len(filtered) == 0 {
			delete(ms.messages, route)
		} else {
			ms.messages[route] = filtered
		}
	}
	return removed
}
