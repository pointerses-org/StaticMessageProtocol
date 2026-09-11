// Package auth provides token management with username binding.
package auth

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

// TokenInfo holds token metadata.
type TokenInfo struct {
	Token    string
	Username string
	Tail     string
}

// TokenStore manages authentication tokens bound to usernames.
type TokenStore struct {
	mu     sync.Mutex
	tokens map[string]*TokenInfo // tail → token info
	tails  []string
}

// NewTokenStore creates a new token store.
func NewTokenStore() *TokenStore {
	return &TokenStore{tokens: make(map[string]*TokenInfo)}
}

// Generate creates a new token bound to a username. Returns the full token string.
func (ts *TokenStore) Generate(username string) string {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	var hex [16]byte
	r.Read(hex[:])
	full := fmt.Sprintf("smpt128-%x", hex[:])
	// Tail is the last 4 bytes = 8 hex chars. This must match TOKEN_TAIL_LEN
	// (core/src/types.rs) exactly, or the client's extracted tail never matches
	// this key and every Lookup misses. fmt repeats a missing argument for extra
	// specifiers, so "%02x" x8 with 4 args silently produced a 16-char tail.
	tail := fmt.Sprintf("%x", hex[12:])

	ts.mu.Lock()
	defer ts.mu.Unlock()
	ts.tokens[tail] = &TokenInfo{Token: full, Username: username, Tail: tail}
	ts.tails = append(ts.tails, tail)
	return full
}

// Lookup finds a token by its tail. Returns the full token, username, and whether it was found.
func (ts *TokenStore) Lookup(tail string) (string, string, bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	info, ok := ts.tokens[tail]
	if !ok {
		return "", "", false
	}
	return info.Token, info.Username, true
}

// LookupUser finds the username for a token tail.
func (ts *TokenStore) LookupUser(tail string) (string, bool) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	info, ok := ts.tokens[tail]
	if !ok {
		return "", false
	}
	return info.Username, true
}

// List returns all token tails.
func (ts *TokenStore) List() []string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	result := make([]string, len(ts.tails))
	copy(result, ts.tails)
	return result
}

// ListAll returns all token infos.
func (ts *TokenStore) ListAll() []*TokenInfo {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	result := make([]*TokenInfo, 0, len(ts.tails))
	for _, tail := range ts.tails {
		if info, ok := ts.tokens[tail]; ok {
			result = append(result, info)
		}
	}
	return result
}

// Revoke removes a token by its tail. Returns true if found and revoked.
func (ts *TokenStore) Revoke(tail string) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	if _, ok := ts.tokens[tail]; ok {
		delete(ts.tokens, tail)
		for i, t := range ts.tails {
			if t == tail {
				ts.tails = append(ts.tails[:i], ts.tails[i+1:]...)
				break
			}
		}
		return true
	}
	return false
}
