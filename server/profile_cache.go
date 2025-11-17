//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"math/rand"
	"runtime"
	"time"

	"github.com/Station-Manager/types"
)

// Copied from cache.go for standalone profiling
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
	entries    map[int64]*logbookCacheEntry
	maxEntries int
	head       *lruNode
	tail       *lruNode
}

const (
	defaultLogbookCacheTTL        = 5 * time.Minute
	defaultLogbookCacheMaxEntries = 1024
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

	entry, ok := c.entries[id]
	if !ok {
		return empty, false
	}

	if time.Now().After(entry.expiresAt) {
		c.removeLocked(id)
		return empty, false
	}

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

	if c.entries == nil {
		c.entries = make(map[int64]*logbookCacheEntry)
	}

	if entry, exists := c.entries[id]; exists {
		entry.value = lb
		entry.expiresAt = time.Now().Add(ttl)
		c.moveToFrontLocked(entry)
		return
	}

	if c.maxEntries > 0 && len(c.entries) >= c.maxEntries {
		if c.tail != nil {
			c.removeLocked(c.tail.key)
		}
	}

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

func (c *inMemoryLogbookCache) removeLocked(id int64) {
	entry, ok := c.entries[id]
	if !ok {
		return
	}

	node := entry.prev
	if node != nil {
		c.removeNodeLocked(node)
	}

	delete(c.entries, id)
}

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

func (c *inMemoryLogbookCache) moveToFrontLocked(entry *logbookCacheEntry) {
	if entry == nil {
		return
	}

	node := entry.prev
	if node == nil || node == c.head {
		return
	}

	c.removeNodeLocked(node)
	c.addToFrontLocked(node)
}

func main() {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	// Benchmark scenarios
	fmt.Println("=== Cache Performance Profile ===\n")

	// 1. Sequential writes
	start := time.Now()
	for i := 0; i < 10000; i++ {
		cache.Set(int64(i), lb, defaultLogbookCacheTTL)
	}
	elapsed := time.Since(start)
	fmt.Printf("Sequential writes (10k):     %v (%.2f ns/op)\n", elapsed, float64(elapsed.Nanoseconds())/10000)

	// 2. Sequential reads (all hits)
	start = time.Now()
	hits := 0
	for i := 0; i < 10000; i++ {
		if _, ok := cache.Get(int64(i)); ok {
			hits++
		}
	}
	elapsed = time.Since(start)
	fmt.Printf("Sequential reads (10k hits): %v (%.2f ns/op, %d hits)\n", elapsed, float64(elapsed.Nanoseconds())/10000, hits)

	// 3. Random reads
	rng := rand.New(rand.NewSource(42))
	start = time.Now()
	hits = 0
	for i := 0; i < 10000; i++ {
		if _, ok := cache.Get(int64(rng.Intn(10000))); ok {
			hits++
		}
	}
	elapsed = time.Since(start)
	fmt.Printf("Random reads (10k):          %v (%.2f ns/op, %d hits)\n", elapsed, float64(elapsed.Nanoseconds())/10000, hits)

	// 4. Mixed workload (70% reads, 20% writes, 10% invalidations)
	cache2 := newInMemoryLogbookCache()
	for i := 0; i < 500; i++ {
		cache2.Set(int64(i), lb, defaultLogbookCacheTTL)
	}

	start = time.Now()
	for i := 0; i < 10000; i++ {
		op := i % 10
		id := int64(i % 1000)

		switch {
		case op < 7:
			cache2.Get(id)
		case op < 9:
			cache2.Set(id, lb, defaultLogbookCacheTTL)
		default:
			// Invalidate would require mutex, skip for now
		}
	}
	elapsed = time.Since(start)
	fmt.Printf("Mixed workload (10k ops):    %v (%.2f ns/op)\n", elapsed, float64(elapsed.Nanoseconds())/10000)

	// 5. Hot key access (best case)
	start = time.Now()
	for i := 0; i < 10000; i++ {
		cache.Get(1)
	}
	elapsed = time.Since(start)
	fmt.Printf("Hot key access (10k):        %v (%.2f ns/op)\n", elapsed, float64(elapsed.Nanoseconds())/10000)

	// 6. LRU eviction test
	cache3 := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 1000,
	}
	start = time.Now()
	for i := 0; i < 10000; i++ {
		cache3.Set(int64(i), lb, defaultLogbookCacheTTL)
	}
	elapsed = time.Since(start)
	fmt.Printf("LRU eviction (10k w/ 1k max): %v (%.2f ns/op)\n", elapsed, float64(elapsed.Nanoseconds())/10000)

	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("\n=== Memory Stats ===\n")
	fmt.Printf("Alloc:      %v MB\n", m.Alloc/1024/1024)
	fmt.Printf("TotalAlloc: %v MB\n", m.TotalAlloc/1024/1024)
	fmt.Printf("Sys:        %v MB\n", m.Sys/1024/1024)
	fmt.Printf("NumGC:      %v\n", m.NumGC)

	// Cache stats
	fmt.Printf("\n=== Cache Stats ===\n")
	fmt.Printf("Entries in cache3: %d\n", len(cache3.entries))
	fmt.Printf("Max entries: %d\n", cache3.maxEntries)
}
