package service

import (
	"context"
	"sync"
	"time"

	"github.com/Station-Manager/errors"
	"github.com/Station-Manager/types"
)

type logbookCache interface {
	Get(id int64) (types.Logbook, bool)
	Set(id int64, lb types.Logbook, ttl time.Duration)
	Invalidate(id int64)
}

type logbookCacheEntry struct {
	value     types.Logbook
	expiresAt time.Time
	prev      *lruNode
	next      *lruNode
}

type lruNode struct {
	key  int64
	prev *lruNode
	next *lruNode
}

type inMemoryLogbookCache struct {
	mu         sync.RWMutex
	entries    map[int64]*logbookCacheEntry
	maxEntries int
	// LRU doubly-linked list
	head *lruNode // most recently used
	tail *lruNode // least recently used
}

const (
	defaultLogbookCacheTTL        = 5 * time.Minute //TODO: make configurable
	defaultLogbookCacheMaxEntries = 1024            //TODO: make configurable
)

func newInMemoryLogbookCache() *inMemoryLogbookCache {
	return &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: defaultLogbookCacheMaxEntries,
		head:       nil,
		tail:       nil,
	}
}

func (c *inMemoryLogbookCache) Get(id int64) (types.Logbook, bool) {
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

	if time.Now().After(entry.expiresAt) {
		// expired; treat as miss and remove
		c.removeLocked(id)
		return empty, false
	}

	// Move to front (most recently used)
	c.moveToFrontLocked(entry)

	return entry.value, true
}

func (c *inMemoryLogbookCache) Set(id int64, lb types.Logbook, ttl time.Duration) {
	if c == nil {
		return
	}
	if ttl <= 0 {
		ttl = defaultLogbookCacheTTL
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.entries == nil {
		c.entries = make(map[int64]*logbookCacheEntry)
	}

	// Update existing entry
	if entry, exists := c.entries[id]; exists {
		entry.value = lb
		entry.expiresAt = time.Now().Add(ttl)
		c.moveToFrontLocked(entry)
		return
	}

	// Evict LRU entry if at capacity
	if c.maxEntries > 0 && len(c.entries) >= c.maxEntries {
		if c.tail != nil {
			c.removeLocked(c.tail.key)
		}
	}

	// Create new entry and node
	node := &lruNode{key: id}
	entry := &logbookCacheEntry{
		value:     lb,
		expiresAt: time.Now().Add(ttl),
		prev:      node,
		next:      node,
	}

	c.entries[id] = entry
	c.addToFrontLocked(node)
}

func (c *inMemoryLogbookCache) Invalidate(id int64) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.removeLocked(id)
}

// removeLocked removes an entry from the cache. Must be called with lock held.
func (c *inMemoryLogbookCache) removeLocked(id int64) {
	entry, ok := c.entries[id]
	if !ok {
		return
	}

	// Remove from LRU list
	node := entry.prev
	if node != nil {
		c.removeNodeLocked(node)
	}

	delete(c.entries, id)
}

// addToFrontLocked adds a node to the front (most recently used position). Must be called with lock held.
func (c *inMemoryLogbookCache) addToFrontLocked(node *lruNode) {
	if node == nil {
		return
	}

	node.next = c.head
	node.prev = nil

	if c.head != nil {
		c.head.prev = node
	}
	c.head = node

	if c.tail == nil {
		c.tail = node
	}
}

// removeNodeLocked removes a node from the LRU list. Must be called with lock held.
func (c *inMemoryLogbookCache) removeNodeLocked(node *lruNode) {
	if node == nil {
		return
	}

	if node.prev != nil {
		node.prev.next = node.next
	} else {
		c.head = node.next
	}

	if node.next != nil {
		node.next.prev = node.prev
	} else {
		c.tail = node.prev
	}
}

// moveToFrontLocked moves an entry to the front of the LRU list. Must be called with lock held.
func (c *inMemoryLogbookCache) moveToFrontLocked(entry *logbookCacheEntry) {
	if entry == nil {
		return
	}

	node := entry.prev
	if node == nil || node == c.head {
		return // already at front or not in list
	}

	c.removeNodeLocked(node)
	c.addToFrontLocked(node)
}

// fetchLogbookWithCache retrieves a logbook by ID using an in-memory cache backed by the database service.
// It assumes that the provided Service has a non-nil db and logbookCache.
func (s *Service) fetchLogbookWithCache(ctx context.Context, logbookID int64) (types.Logbook, error) {
	const op errors.Op = "server.Service.fetchLogbookWithCache"
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

	// 1. Try cache first.
	if s.logbookCache != nil {
		if lb, ok := s.logbookCache.Get(logbookID); ok {
			return lb, nil
		}
	}

	// 2. Fallback to database.
	logbook, err := s.db.FetchLogbookByIDContext(ctx, logbookID)
	if err != nil {
		return emptyRetVal, errors.New(op).Err(err)
	}
	if s.logbookCache != nil {
		s.logbookCache.Set(logbookID, logbook, defaultLogbookCacheTTL)
	}

	return logbook, nil
}
