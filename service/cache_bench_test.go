package service

import (
	"math/rand"
	"testing"
	"time"

	"github.com/Station-Manager/types"
)

// Benchmark cache operations for performance profiling

func BenchmarkCache_Set(b *testing.B) {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(int64(i), lb, defaultLogbookCacheTTL)
	}
}

func BenchmarkCache_Get_Hit(b *testing.B) {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	// Pre-populate cache
	for i := 0; i < 1000; i++ {
		cache.Set(int64(i), lb, defaultLogbookCacheTTL)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(int64(i % 1000))
	}
}

func BenchmarkCache_Get_Miss(b *testing.B) {
	cache := newInMemoryLogbookCache()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(int64(i))
	}
}

func BenchmarkCache_SetUpdate(b *testing.B) {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	// Pre-populate with one entry
	cache.Set(1, lb, defaultLogbookCacheTTL)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		lb.UserID = int64(i)
		cache.Set(1, lb, defaultLogbookCacheTTL)
	}
}

func BenchmarkCache_Invalidate(b *testing.B) {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(int64(i), lb, defaultLogbookCacheTTL)
		cache.Invalidate(int64(i))
	}
}

func BenchmarkCache_LRU_Eviction(b *testing.B) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 1000,
	}
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Set(int64(i), lb, defaultLogbookCacheTTL)
	}
}

func BenchmarkCache_MixedWorkload(b *testing.B) {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	// Pre-populate
	for i := 0; i < 500; i++ {
		cache.Set(int64(i), lb, defaultLogbookCacheTTL)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		op := i % 10
		id := int64(i % 1000)

		switch {
		case op < 7: // 70% reads
			cache.Get(id)
		case op < 9: // 20% writes
			cache.Set(id, lb, defaultLogbookCacheTTL)
		default: // 10% invalidations
			cache.Invalidate(id)
		}
	}
}

func BenchmarkCache_Parallel_Get(b *testing.B) {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	// Pre-populate
	for i := 0; i < 1000; i++ {
		cache.Set(int64(i), lb, defaultLogbookCacheTTL)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := int64(0)
		for pb.Next() {
			cache.Get(i % 1000)
			i++
		}
	})
}

func BenchmarkCache_Parallel_Set(b *testing.B) {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := int64(0)
		for pb.Next() {
			cache.Set(i%1000, lb, defaultLogbookCacheTTL)
			i++
		}
	})
}

func BenchmarkCache_Parallel_MixedWorkload(b *testing.B) {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	// Pre-populate
	for i := 0; i < 500; i++ {
		cache.Set(int64(i), lb, defaultLogbookCacheTTL)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			op := i % 10
			id := int64(i % 1000)

			switch {
			case op < 7: // 70% reads
				cache.Get(id)
			case op < 9: // 20% writes
				cache.Set(id, lb, defaultLogbookCacheTTL)
			default: // 10% invalidations
				cache.Invalidate(id)
			}
			i++
		}
	})
}

func BenchmarkCache_Expiration_Check(b *testing.B) {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	// Pre-populate with expired entries
	for i := 0; i < 1000; i++ {
		cache.Set(int64(i), lb, 1*time.Nanosecond)
	}

	// Wait for expiration
	time.Sleep(10 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(int64(i % 1000))
	}
}

func BenchmarkCache_RandomAccess(b *testing.B) {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	// Pre-populate
	for i := 0; i < 10000; i++ {
		cache.Set(int64(i), lb, defaultLogbookCacheTTL)
	}

	// Create random access pattern
	rng := rand.New(rand.NewSource(42))
	ids := make([]int64, b.N)
	for i := 0; i < b.N; i++ {
		ids[i] = int64(rng.Intn(10000))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(ids[i])
	}
}

func BenchmarkCache_LRU_ThrashingWorstCase(b *testing.B) {
	cache := &inMemoryLogbookCache{
		entries:    make(map[int64]*logbookCacheEntry),
		maxEntries: 100,
	}
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	// Fill cache to capacity
	for i := 0; i < 100; i++ {
		cache.Set(int64(i), lb, defaultLogbookCacheTTL)
	}

	b.ResetTimer()
	// Keep adding entries that will cause evictions
	for i := 0; i < b.N; i++ {
		cache.Set(int64(100+i), lb, defaultLogbookCacheTTL)
	}
}

func BenchmarkCache_SequentialAccess(b *testing.B) {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	// Pre-populate
	for i := 0; i < 10000; i++ {
		cache.Set(int64(i), lb, defaultLogbookCacheTTL)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cache.Get(int64(i % 10000))
	}
}

func BenchmarkCache_HotKey(b *testing.B) {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	// Pre-populate
	for i := 0; i < 1000; i++ {
		cache.Set(int64(i), lb, defaultLogbookCacheTTL)
	}

	b.ResetTimer()
	// Access same key repeatedly (best case - already at front)
	for i := 0; i < b.N; i++ {
		cache.Get(1)
	}
}

func BenchmarkCache_ColdKey(b *testing.B) {
	cache := newInMemoryLogbookCache()
	lb := types.Logbook{ID: 1, Callsign: "W1AW", UserID: 100}

	// Pre-populate
	for i := 0; i < 1000; i++ {
		cache.Set(int64(i), lb, defaultLogbookCacheTTL)
	}

	b.ResetTimer()
	// Always access tail (worst case - requires full list traversal)
	for i := 0; i < b.N; i++ {
		cache.Get(0) // First entry added, should be at tail
		// Re-add to push it back to tail for next iteration
		cache.Set(0, lb, defaultLogbookCacheTTL)
		for j := 1; j < 1000; j++ {
			cache.Set(int64(j), lb, defaultLogbookCacheTTL)
		}
	}
}
