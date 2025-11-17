package server

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
}

type inMemoryLogbookCache struct {
	mu         sync.RWMutex
	entries    map[int64]logbookCacheEntry
	maxEntries int
}

const (
	defaultLogbookCacheTTL        = 5 * time.Minute //TODO: make configurable
	defaultLogbookCacheMaxEntries = 1024            //TODO: make configurable
)

func newInMemoryLogbookCache() *inMemoryLogbookCache {
	return &inMemoryLogbookCache{
		entries:    make(map[int64]logbookCacheEntry),
		maxEntries: defaultLogbookCacheMaxEntries,
	}
}

func (c *inMemoryLogbookCache) Get(id int64) (types.Logbook, bool) {
	var empty types.Logbook
	if c == nil {
		return empty, false
	}

	c.mu.RLock()
	entry, ok := c.entries[id]
	c.mu.RUnlock()
	if !ok {
		return empty, false
	}

	if time.Now().After(entry.expiresAt) {
		// expired; treat as miss and remove lazily
		c.Invalidate(id)
		return empty, false
	}

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
		c.entries = make(map[int64]logbookCacheEntry)
	}

	if c.maxEntries > 0 && len(c.entries) >= c.maxEntries {
		// Simple eviction: remove one arbitrary entry.
		for k := range c.entries {
			delete(c.entries, k)
			break
		}
	}

	c.entries[id] = logbookCacheEntry{
		value:     lb,
		expiresAt: time.Now().Add(ttl),
	}
}

func (c *inMemoryLogbookCache) Invalidate(id int64) {
	if c == nil {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, id)
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
