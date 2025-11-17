package service

import (
	"context"
	"sync"
	"time"

	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
)

// optimizedLogbookCacheEntry combines the cache entry and LRU node into a single structure
// This reduces memory overhead and pointer indirection
type optimizedLogbookCacheEntry struct {
	key       int64
	value     types.Logbook
	expiresAt time.Time
	prev      *optimizedLogbookCacheEntry
	next      *optimizedLogbookCacheEntry
}

type optimizedInMemoryLogbookCache struct {
	mu         sync.RWMutex
	entries    map[int64]*optimizedLogbookCacheEntry
	maxEntries int
	head       *optimizedLogbookCacheEntry // most recently used
	tail       *optimizedLogbookCacheEntry // least recently used
}

func newOptimizedInMemoryLogbookCache() *optimizedInMemoryLogbookCache {
	return &optimizedInMemoryLogbookCache{
		// Pre-allocate map with expected capacity to reduce allocations
		entries:    make(map[int64]*optimizedLogbookCacheEntry, defaultLogbookCacheMaxEntries),
		maxEntries: defaultLogbookCacheMaxEntries,
		head:       nil,
		tail:       nil,
	}
}

func (c *optimizedInMemoryLogbookCache) Get(id int64) (types.Logbook, bool) {
	var empty types.Logbook
	if c == nil {
		return empty, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.entries[id]
	if !ok {
		return empty, false
	}

	// Check expiration
	if time.Now().After(entry.expiresAt) {
		c.removeLocked(entry)
		return empty, false
	}

	// Fast path: already at head, no list manipulation needed
	if entry == c.head {
		return entry.value, true
	}

	// Move to front
	c.moveToFrontLocked(entry)

	return entry.value, true
}

func (c *optimizedInMemoryLogbookCache) Set(id int64, lb types.Logbook, ttl time.Duration) {
	if c == nil {
		return
	}
	if ttl <= 0 {
		ttl = defaultLogbookCacheTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.entries == nil {
		c.entries = make(map[int64]*optimizedLogbookCacheEntry, defaultLogbookCacheMaxEntries)
	}

	// Update existing entry
	if entry, exists := c.entries[id]; exists {
		entry.value = lb
		entry.expiresAt = time.Now().Add(ttl)
		// Fast path: already at head
		if entry != c.head {
			c.moveToFrontLocked(entry)
		}
		return
	}

	// Evict LRU entry if at capacity
	if c.maxEntries > 0 && len(c.entries) >= c.maxEntries {
		if c.tail != nil {
			c.removeLocked(c.tail)
		}
	}

	// Create new entry (single allocation instead of two)
	entry := &optimizedLogbookCacheEntry{
		key:       id,
		value:     lb,
		expiresAt: time.Now().Add(ttl),
	}

	c.entries[id] = entry
	c.addToFrontLocked(entry)
}

func (c *optimizedInMemoryLogbookCache) Invalidate(id int64) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.entries[id]; ok {
		c.removeLocked(entry)
	}
}

// removeLocked removes an entry from both the map and LRU list
// Must be called with lock held
func (c *optimizedInMemoryLogbookCache) removeLocked(entry *optimizedLogbookCacheEntry) {
	if entry == nil {
		return
	}

	// Remove from LRU list
	if entry.prev != nil {
		entry.prev.next = entry.next
	} else {
		c.head = entry.next
	}

	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		c.tail = entry.prev
	}

	// Remove from map
	delete(c.entries, entry.key)
}

// addToFrontLocked adds an entry to the front (most recently used position)
// Must be called with lock held
func (c *optimizedInMemoryLogbookCache) addToFrontLocked(entry *optimizedLogbookCacheEntry) {
	if entry == nil {
		return
	}

	entry.next = c.head
	entry.prev = nil

	if c.head != nil {
		c.head.prev = entry
	}
	c.head = entry

	if c.tail == nil {
		c.tail = entry
	}
}

// moveToFrontLocked moves an entry to the front of the LRU list
// Must be called with lock held
func (c *optimizedInMemoryLogbookCache) moveToFrontLocked(entry *optimizedLogbookCacheEntry) {
	if entry == nil || entry == c.head {
		return // already at front
	}

	// Remove from current position
	if entry.prev != nil {
		entry.prev.next = entry.next
	}

	if entry.next != nil {
		entry.next.prev = entry.prev
	} else {
		// Was at tail
		c.tail = entry.prev
	}

	// Add to front
	entry.next = c.head
	entry.prev = nil

	if c.head != nil {
		c.head.prev = entry
	}
	c.head = entry
}

// fetchLogbookWithOptimizedCache uses the optimized cache implementation
func (s *Service) fetchLogbookWithOptimizedCache(ctx context.Context, logbookID int64) (types.Logbook, error) {
	const op errors.Op = "server.Service.fetchLogbookWithOptimizedCache"
	var emptyRetVal types.Logbook

	if s == nil {
		return emptyRetVal, errors.New(op).Msg(errMsgNilService)
	}
	if ctx == nil {
		return emptyRetVal, errors.New(op).Msg(errMsgNilContext)
	}
	if logbookID == 0 {
		return emptyRetVal, errors.New(op).Msg("logbookID is zero")
	}

	// Try cache first (optimized implementation)
	if cache, ok := s.logbookCache.(*optimizedInMemoryLogbookCache); ok {
		if lb, found := cache.Get(logbookID); found {
			return lb, nil
		}
	}

	// Fallback to database
	logbook, err := s.db.FetchLogbookByIDContext(ctx, logbookID)
	if err != nil {
		return emptyRetVal, errors.New(op).Err(err)
	}

	// Store in optimized cache
	if cache, ok := s.logbookCache.(*optimizedInMemoryLogbookCache); ok {
		cache.Set(logbookID, logbook, defaultLogbookCacheTTL)
	}

	return logbook, nil
}
