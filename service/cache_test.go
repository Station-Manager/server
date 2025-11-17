package service

import (
	"testing"
	"time"

	"github.com/Station-Manager/types"
)

func TestNewInMemoryLogbookCache(t *testing.T) {
	cache := newInMemoryLogbookCache()
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
	if cache.entries == nil {
		t.Error("expected initialized entries map")
	}
	if cache.maxEntries != defaultLogbookCacheMaxEntries {
		t.Errorf("expected maxEntries=%d, got %d", defaultLogbookCacheMaxEntries, cache.maxEntries)
	}
	if cache.head != nil {
		t.Error("expected nil head")
	}
	if cache.tail != nil {
		t.Error("expected nil tail")
	}
}

func TestInMemoryLogbookCache_SetAndGet(t *testing.T) {
	cache := newInMemoryLogbookCache()

	lb := types.Logbook{
		ID:       1,
		Callsign: "W1AW",
		UserID:   100,
	}

	// Set a logbook
	cache.Set(1, lb, 5*time.Minute)

	// Get it back
	retrieved, ok := cache.Get(1)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if retrieved.ID != lb.ID {
		t.Errorf("expected ID=%d, got %d", lb.ID, retrieved.ID)
	}
	if retrieved.Callsign != lb.Callsign {
		t.Errorf("expected Callsign=%s, got %s", lb.Callsign, retrieved.Callsign)
	}
}

func TestInMemoryLogbookCache_GetNonExistent(t *testing.T) {
	cache := newInMemoryLogbookCache()

	_, ok := cache.Get(999)
	if ok {
		t.Error("expected cache miss for non-existent key")
	}
}

func TestInMemoryLogbookCache_GetExpired(t *testing.T) {
	cache := newInMemoryLogbookCache()

	lb := types.Logbook{ID: 1, Callsign: "W1AW"}
	cache.Set(1, lb, 1*time.Millisecond)

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	_, ok := cache.Get(1)
	if ok {
		t.Error("expected cache miss for expired entry")
	}

	// Verify entry was removed
	if _, exists := cache.entries[1]; exists {
		t.Error("expected expired entry to be removed from cache")
	}
}

func TestInMemoryLogbookCache_UpdateExisting(t *testing.T) {
	cache := newInMemoryLogbookCache()

	lb1 := types.Logbook{ID: 1, Callsign: "W1AW"}
	cache.Set(1, lb1, 5*time.Minute)

	lb2 := types.Logbook{ID: 1, Callsign: "W2XYZ"}
	cache.Set(1, lb2, 5*time.Minute)

	retrieved, ok := cache.Get(1)
	if !ok {
		t.Fatal("expected cache hit")
	}
	if retrieved.Callsign != "W2XYZ" {
		t.Errorf("expected updated Callsign=W2XYZ, got %s", retrieved.Callsign)
	}

	// Verify only one entry exists
	if len(cache.entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(cache.entries))
	}
}

func TestInMemoryLogbookCache_LRUEviction(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 3,
	}

	// Fill cache to capacity
	cache.Set(1, types.Logbook{ID: 1, Callsign: "W1AW"}, 5*time.Minute)
	cache.Set(2, types.Logbook{ID: 2, Callsign: "W2AW"}, 5*time.Minute)
	cache.Set(3, types.Logbook{ID: 3, Callsign: "W3AW"}, 5*time.Minute)

	if len(cache.entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(cache.entries))
	}

	// Access entry 1 to make it recently used
	_, ok := cache.Get(1)
	if !ok {
		t.Fatal("expected cache hit for entry 1")
	}

	// Add a 4th entry, should evict entry 2 (oldest unused)
	cache.Set(4, types.Logbook{ID: 4, Callsign: "W4AW"}, 5*time.Minute)

	if len(cache.entries) != 3 {
		t.Errorf("expected 3 entries after eviction, got %d", len(cache.entries))
	}

	// Entry 2 should be evicted
	_, ok = cache.Get(2)
	if ok {
		t.Error("expected entry 2 to be evicted")
	}

	// Entries 1, 3, 4 should still exist
	for _, id := range []int64{1, 3, 4} {
		if _, ok := cache.Get(id); !ok {
			t.Errorf("expected entry %d to still exist", id)
		}
	}
}

func TestInMemoryLogbookCache_LRUOrderAfterAccess(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 3,
	}

	// Add 3 entries
	cache.Set(1, types.Logbook{ID: 1, Callsign: "W1AW"}, 5*time.Minute)
	cache.Set(2, types.Logbook{ID: 2, Callsign: "W2AW"}, 5*time.Minute)
	cache.Set(3, types.Logbook{ID: 3, Callsign: "W3AW"}, 5*time.Minute)

	// Access entries in order: 1, 2, 3
	cache.Get(1)
	cache.Get(2)
	cache.Get(3)

	// Now 3 is most recent, 1 is least recent
	// Adding entry 4 should evict entry 1
	cache.Set(4, types.Logbook{ID: 4, Callsign: "W4AW"}, 5*time.Minute)

	_, ok := cache.Get(1)
	if ok {
		t.Error("expected entry 1 (LRU) to be evicted")
	}

	for _, id := range []int64{2, 3, 4} {
		if _, ok := cache.Get(id); !ok {
			t.Errorf("expected entry %d to still exist", id)
		}
	}
}

func TestInMemoryLogbookCache_Invalidate(t *testing.T) {
	cache := newInMemoryLogbookCache()

	lb := types.Logbook{ID: 1, Callsign: "W1AW"}
	cache.Set(1, lb, 5*time.Minute)

	// Verify it exists
	_, ok := cache.Get(1)
	if !ok {
		t.Fatal("expected cache hit before invalidation")
	}

	// Invalidate
	cache.Invalidate(1)

	// Verify it's gone
	_, ok = cache.Get(1)
	if ok {
		t.Error("expected cache miss after invalidation")
	}

	if _, exists := cache.entries[1]; exists {
		t.Error("expected entry to be removed from map")
	}
}

func TestInMemoryLogbookCache_InvalidateNonExistent(t *testing.T) {
	cache := newInMemoryLogbookCache()

	// Should not panic
	cache.Invalidate(999)
}

func TestInMemoryLogbookCache_SetWithDefaultTTL(t *testing.T) {
	cache := newInMemoryLogbookCache()

	lb := types.Logbook{ID: 1, Callsign: "W1AW"}
	cache.Set(1, lb, 0) // 0 or negative should use default

	entry, ok := cache.entries[1]
	if !ok {
		t.Fatal("expected entry to exist")
	}

	expectedExpiry := time.Now().Add(defaultLogbookCacheTTL)
	diff := entry.expiresAt.Sub(expectedExpiry).Abs()
	if diff > time.Second {
		t.Errorf("expected expiry close to default TTL, diff=%v", diff)
	}
}

func TestInMemoryLogbookCache_NilReceiver(t *testing.T) {
	var cache *inMemoryLogbookCache

	// All methods should handle nil receiver gracefully
	_, ok := cache.Get(1)
	if ok {
		t.Error("expected false from Get on nil cache")
	}

	cache.Set(1, types.Logbook{}, 5*time.Minute) // Should not panic
	cache.Invalidate(1)                          // Should not panic
}

func TestInMemoryLogbookCache_LRUListIntegrity(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 5,
	}

	// Add several entries
	for i := int64(1); i <= 5; i++ {
		cache.Set(i, types.Logbook{ID: i, Callsign: "TEST"}, 5*time.Minute)
	}

	// Verify list integrity: head should be most recent (5), tail should be oldest (1)
	if cache.head == nil || cache.head.key != 5 {
		t.Errorf("expected head.key=5, got %v", cache.head)
	}
	if cache.tail == nil || cache.tail.key != 1 {
		t.Errorf("expected tail.key=1, got %v", cache.tail)
	}

	// Walk from head to tail
	node := cache.head
	count := 0
	for node != nil {
		count++
		node = node.next
	}
	if count != 5 {
		t.Errorf("expected 5 nodes walking head->tail, got %d", count)
	}

	// Walk from tail to head
	node = cache.tail
	count = 0
	for node != nil {
		count++
		node = node.prev
	}
	if count != 5 {
		t.Errorf("expected 5 nodes walking tail->head, got %d", count)
	}
}

func TestInMemoryLogbookCache_AccessPromotesToFront(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 5,
	}

	// Add 3 entries
	cache.Set(1, types.Logbook{ID: 1}, 5*time.Minute)
	cache.Set(2, types.Logbook{ID: 2}, 5*time.Minute)
	cache.Set(3, types.Logbook{ID: 3}, 5*time.Minute)

	// Head should be 3 (most recent set)
	if cache.head.key != 3 {
		t.Errorf("expected head.key=3, got %d", cache.head.key)
	}

	// Access entry 1 (currently at tail)
	cache.Get(1)

	// Now head should be 1
	if cache.head.key != 1 {
		t.Errorf("expected head.key=1 after access, got %d", cache.head.key)
	}
}

func TestInMemoryLogbookCache_RemoveMiddleNode(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 5,
	}

	// Add 3 entries
	cache.Set(1, types.Logbook{ID: 1}, 5*time.Minute)
	cache.Set(2, types.Logbook{ID: 2}, 5*time.Minute)
	cache.Set(3, types.Logbook{ID: 3}, 5*time.Minute)

	// Remove middle entry
	cache.Invalidate(2)

	// Verify list integrity
	if cache.head.key != 3 {
		t.Errorf("expected head.key=3, got %d", cache.head.key)
	}
	if cache.tail.key != 1 {
		t.Errorf("expected tail.key=1, got %d", cache.tail.key)
	}

	// Walk the list
	node := cache.head
	count := 0
	for node != nil {
		count++
		if node.key == 2 {
			t.Error("entry 2 should not be in list")
		}
		node = node.next
	}
	if count != 2 {
		t.Errorf("expected 2 nodes after removal, got %d", count)
	}
}

func TestInMemoryLogbookCache_RemoveHeadNode(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 5,
	}

	cache.Set(1, types.Logbook{ID: 1}, 5*time.Minute)
	cache.Set(2, types.Logbook{ID: 2}, 5*time.Minute)

	// Remove head
	cache.Invalidate(2)

	if cache.head.key != 1 {
		t.Errorf("expected head.key=1, got %d", cache.head.key)
	}
	if cache.tail.key != 1 {
		t.Errorf("expected tail.key=1, got %d", cache.tail.key)
	}
}

func TestInMemoryLogbookCache_RemoveTailNode(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 5,
	}

	cache.Set(1, types.Logbook{ID: 1}, 5*time.Minute)
	cache.Set(2, types.Logbook{ID: 2}, 5*time.Minute)

	// Remove tail
	cache.Invalidate(1)

	if cache.head.key != 2 {
		t.Errorf("expected head.key=2, got %d", cache.head.key)
	}
	if cache.tail.key != 2 {
		t.Errorf("expected tail.key=2, got %d", cache.tail.key)
	}
}

func TestInMemoryLogbookCache_RemoveOnlyNode(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 5,
	}

	cache.Set(1, types.Logbook{ID: 1}, 5*time.Minute)
	cache.Invalidate(1)

	if cache.head != nil {
		t.Error("expected nil head after removing only node")
	}
	if cache.tail != nil {
		t.Error("expected nil tail after removing only node")
	}
	if len(cache.entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(cache.entries))
	}
}

func TestInMemoryLogbookCache_ConcurrentAccess(t *testing.T) {
	cache := newInMemoryLogbookCache()

	// Add some initial data
	for i := int64(1); i <= 10; i++ {
		cache.Set(i, types.Logbook{ID: i, Callsign: "TEST"}, 5*time.Minute)
	}

	// Spawn multiple goroutines doing reads and writes
	done := make(chan bool)
	for g := 0; g < 10; g++ {
		go func(id int64) {
			for i := 0; i < 100; i++ {
				cache.Set(id, types.Logbook{ID: id}, 5*time.Minute)
				cache.Get(id)
				if i%10 == 0 {
					cache.Invalidate(id)
				}
			}
			done <- true
		}(int64(g + 1))
	}

	// Wait for all goroutines
	for g := 0; g < 10; g++ {
		<-done
	}
}

func TestInMemoryLogbookCache_SetWithNegativeTTL(t *testing.T) {
	cache := newInMemoryLogbookCache()

	lb := types.Logbook{ID: 1, Callsign: "W1AW"}
	cache.Set(1, lb, -5*time.Minute) // Negative should use default

	entry, ok := cache.entries[1]
	if !ok {
		t.Fatal("expected entry to exist")
	}

	expectedExpiry := time.Now().Add(defaultLogbookCacheTTL)
	diff := entry.expiresAt.Sub(expectedExpiry).Abs()
	if diff > time.Second {
		t.Errorf("expected expiry close to default TTL, diff=%v", diff)
	}
}

func TestInMemoryLogbookCache_AccessAlreadyAtFront(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 5,
	}

	cache.Set(1, types.Logbook{ID: 1}, 5*time.Minute)
	cache.Set(2, types.Logbook{ID: 2}, 5*time.Minute)

	// 2 is already at front (head)
	if cache.head.key != 2 {
		t.Fatalf("expected head.key=2, got %d", cache.head.key)
	}

	// Access it again - should still be at front
	cache.Get(2)

	if cache.head.key != 2 {
		t.Errorf("expected head.key=2 after accessing front node, got %d", cache.head.key)
	}

	// Verify list integrity
	if cache.tail.key != 1 {
		t.Errorf("expected tail.key=1, got %d", cache.tail.key)
	}
}

func TestInMemoryLogbookCache_EvictWhenEmpty(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 0, // No limit
	}

	// Should not panic or evict when maxEntries is 0
	for i := int64(1); i <= 10; i++ {
		cache.Set(i, types.Logbook{ID: i}, 5*time.Minute)
	}

	if len(cache.entries) != 10 {
		t.Errorf("expected 10 entries, got %d", len(cache.entries))
	}
}

func TestInMemoryLogbookCache_MultipleEvictions(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 2,
	}

	// Fill and overflow multiple times
	for i := int64(1); i <= 10; i++ {
		cache.Set(i, types.Logbook{ID: i, Callsign: "TEST"}, 5*time.Minute)

		// Should never exceed maxEntries
		if len(cache.entries) > cache.maxEntries {
			t.Errorf("cache size %d exceeds maxEntries %d", len(cache.entries), cache.maxEntries)
		}
	}

	// Should only have last 2 entries
	if len(cache.entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(cache.entries))
	}

	// Should have entries 9 and 10
	for _, id := range []int64{9, 10} {
		if _, ok := cache.Get(id); !ok {
			t.Errorf("expected entry %d to exist", id)
		}
	}
}

func TestInMemoryLogbookCache_UpdateExistingPromotesToFront(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 5,
	}

	cache.Set(1, types.Logbook{ID: 1, Callsign: "OLD"}, 5*time.Minute)
	cache.Set(2, types.Logbook{ID: 2, Callsign: "TEST"}, 5*time.Minute)
	cache.Set(3, types.Logbook{ID: 3, Callsign: "TEST"}, 5*time.Minute)

	// Update entry 1 (currently at tail)
	cache.Set(1, types.Logbook{ID: 1, Callsign: "NEW"}, 5*time.Minute)

	// Should now be at front
	if cache.head.key != 1 {
		t.Errorf("expected head.key=1 after update, got %d", cache.head.key)
	}

	// Verify value was updated
	lb, ok := cache.Get(1)
	if !ok {
		t.Fatal("expected to find entry 1")
	}
	if lb.Callsign != "NEW" {
		t.Errorf("expected Callsign=NEW, got %s", lb.Callsign)
	}
}

func TestInMemoryLogbookCache_SetWithNilEntries(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    nil, // nil map
		maxEntries: 5,
	}

	// Should initialize the map and not panic
	cache.Set(1, types.Logbook{ID: 1}, 5*time.Minute)

	if cache.entries == nil {
		t.Fatal("expected entries to be initialized")
	}

	if len(cache.entries) != 1 {
		t.Errorf("expected 1 entry, got %d", len(cache.entries))
	}
}

func TestInMemoryLogbookCache_HelperMethodsWithNil(t *testing.T) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 5,
	}

	// Test helper methods with nil parameters - should not panic
	cache.addToFrontLocked(nil)
	cache.removeNodeLocked(nil)
	cache.moveToFrontLocked(nil)

	// Cache should remain empty
	if len(cache.entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(cache.entries))
	}
}
