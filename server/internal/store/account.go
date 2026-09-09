package store

import (
	"sync"
)

// Account represents a registered user account.
type Account struct {
	Username     string
	Permissions  []string
	CreatedAt    int64
}

// AccountStore manages user accounts.
type AccountStore struct {
	mu       sync.RWMutex
	accounts map[string]*Account
}

// NewAccountStore creates a new account store.
func NewAccountStore() *AccountStore {
	return &AccountStore{
		accounts: make(map[string]*Account),
	}
}

// Create creates a new account.
func (as *AccountStore) Create(username string, permissions []string) *Account {
	as.mu.Lock()
	defer as.mu.Unlock()

	acc := &Account{
		Username:    username,
		Permissions: permissions,
		CreatedAt:   0,
	}
	as.accounts[username] = acc
	return acc
}

// Get retrieves an account by username.
func (as *AccountStore) Get(username string) (*Account, bool) {
	as.mu.RLock()
	defer as.mu.RUnlock()
	acc, ok := as.accounts[username]
	return acc, ok
}

// List returns all accounts.
func (as *AccountStore) List() []*Account {
	as.mu.RLock()
	defer as.mu.RUnlock()
	accounts := make([]*Account, 0, len(as.accounts))
	for _, acc := range as.accounts {
		accounts = append(accounts, acc)
	}
	return accounts
}

// Delete removes an account.
func (as *AccountStore) Delete(username string) bool {
	as.mu.Lock()
	defer as.mu.Unlock()
	if _, ok := as.accounts[username]; ok {
		delete(as.accounts, username)
		return true
	}
	return false
}
