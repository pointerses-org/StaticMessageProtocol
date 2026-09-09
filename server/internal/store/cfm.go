package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CFMEntry is a Cloud File Message entry.
type CFMEntry struct {
	ID        uint64
	FilePath  string
	Uploader  string
	Public    bool
	ExpiresAt time.Time
	Size      int64
}

// CFMStore manages CFM file storage.
type CFMStore struct {
	mu      sync.Mutex
	entries map[uint64]*CFMEntry
	baseDir string
	maxSize int64
}

// NewCFMStore creates a CFM store with the given base directory and max size.
func NewCFMStore(baseDir string, maxMB int) *CFMStore {
	if maxMB <= 0 {
		maxMB = 100
	}
	os.MkdirAll(baseDir, 0755)
	return &CFMStore{
		entries: make(map[uint64]*CFMEntry),
		baseDir: baseDir,
		maxSize: int64(maxMB) * 1024 * 1024,
	}
}

// Save stores a file and returns the entry.
func (cs *CFMStore) Save(cfID uint64, data []byte, uploader string, public bool, expire time.Duration) (*CFMEntry, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	if int64(len(data)) > cs.maxSize {
		return nil, fmt.Errorf("file too large: %d > %d", len(data), cs.maxSize)
	}

	filePath := filepath.Join(cs.baseDir, fmt.Sprintf("%016x.bin", cfID))
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return nil, err
	}

	entry := &CFMEntry{
		ID:        cfID,
		FilePath:  filePath,
		Uploader:  uploader,
		Public:    public,
		ExpiresAt: time.Now().Add(expire),
		Size:      int64(len(data)),
	}
	cs.entries[cfID] = entry
	return entry, nil
}

// Load retrieves a file by CFM ID.
func (cs *CFMStore) Load(cfID uint64, requester string) ([]byte, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	entry, ok := cs.entries[cfID]
	if !ok {
		return nil, fmt.Errorf("CFM not found: %d", cfID)
	}
	if time.Now().After(entry.ExpiresAt) {
		os.Remove(entry.FilePath)
		delete(cs.entries, cfID)
		return nil, fmt.Errorf("CFM expired: %d", cfID)
	}
	if !entry.Public && entry.Uploader != requester {
		return nil, fmt.Errorf("no permission for CFM: %d", cfID)
	}
	return os.ReadFile(entry.FilePath)
}

// Cleanup removes expired entries and returns the count removed.
func (cs *CFMStore) Cleanup() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	removed := 0
	for cfID, entry := range cs.entries {
		if time.Now().After(entry.ExpiresAt) {
			os.Remove(entry.FilePath)
			delete(cs.entries, cfID)
			removed++
		}
	}
	return removed
}
