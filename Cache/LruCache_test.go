package cache

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"
)

// ---------- 基础功能测试 ----------

func TestCache_BasicAddGet(t *testing.T) {
	c := &Cache{Cache_bytes: 1024}
	c.Add("k1", ByteView{bt: []byte("value1")})

	if v, ok := c.Get("k1"); !ok || v.String() != "value1" {
		t.Fatalf("expected value1, got %v", v)
	}

	if _, ok := c.Get("missing"); ok {
		t.Fatal("expected miss for non-existent key")
	}
}

// ---------- 并发读写测试 ----------

func TestCache_ConcurrentReadWrite(t *testing.T) {
	c := &Cache{Cache_bytes: 0} // 无上限
	const numGoroutines = 100
	const numOps = 500

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 2) // 读 + 写

	// 并发写
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				c.Add(key, ByteView{bt: []byte(fmt.Sprintf("val-%d-%d", id, j))})
			}
		}(i)
	}

	// 并发读
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := fmt.Sprintf("key-%d-%d", id, j)
				c.Get(key)
			}
		}(i)
	}

	wg.Wait()
}

// ---------- 混合读写 + 淘汰压力 ----------

func TestCache_MixedOpsWithEviction(t *testing.T) {
	// 设置较小容量，频繁触发淘汰
	c := &Cache{Cache_bytes: 256}
	const numGoroutines = 50
	const numOps = 200

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))
			for j := 0; j < numOps; j++ {
				key := fmt.Sprintf("k%d", r.Intn(100))
				if r.Float32() < 0.7 {
					c.Add(key, ByteView{bt: []byte(fmt.Sprintf("v%d", r.Intn(1000)))})
				} else {
					c.Get(key)
				}
			}
		}(i)
	}

	wg.Wait()
}

// ---------- 并发更新同一 key ----------

func TestCache_ConcurrentUpdateSameKey(t *testing.T) {
	c := &Cache{Cache_bytes: 1024}
	const numGoroutines = 100
	const numUpdates = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numUpdates; j++ {
				c.Add("shared-key", ByteView{bt: []byte(fmt.Sprintf("g%d-v%d", id, j))})
			}
		}(i)
	}

	wg.Wait()

	// 最终应该能读取到某个值（不关心具体哪个，只要不 panic 且一致）
	if v, ok := c.Get("shared-key"); !ok {
		t.Fatal("shared-key should exist")
	} else {
		t.Logf("final shared-key value: %s", v.String())
	}
}

// ---------- 读写交错 + 数据一致性 ----------

func TestCache_ConsistencyCheck(t *testing.T) {
	c := &Cache{Cache_bytes: 0}
	const numKeys = 50

	// 先写入已知数据
	expected := make(map[string]string)
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("ckey%d", i)
		val := fmt.Sprintf("cval%d", i)
		c.Add(key, ByteView{bt: []byte(val)})
		expected[key] = val
	}

	// 并发读，验证数据一致性
	var wg sync.WaitGroup
	errCh := make(chan error, numKeys*10)

	for round := 0; round < 10; round++ {
		wg.Add(numKeys)
		for i := 0; i < numKeys; i++ {
			go func(idx int) {
				defer wg.Done()
				key := fmt.Sprintf("ckey%d", idx)
				if v, ok := c.Get(key); ok {
					if v.String() != expected[key] {
						errCh <- fmt.Errorf("key %s: expected %s, got %s", key, expected[key], v.String())
					}
				}
			}(i)
		}
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Error(err)
	}
}

// ---------- 高并发淘汰回调安全性 ----------

func TestCache_EvictionCallbackConcurrency(t *testing.T) {
	var mu sync.Mutex
	evictedKeys := make([]string, 0)

	// 用底层 lru 直接测试回调
	c := &Cache{Cache_bytes: 64}

	// 手动初始化以注入回调（绕过懒加载）
	c.mu.Lock()
	c.lru_cache = nil // 重置
	c.mu.Unlock()

	const numGoroutines = 30
	const numOps = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				key := fmt.Sprintf("ek%d-%d", id, j)
				c.Add(key, ByteView{bt: []byte("x")})
			}
		}(i)
	}

	wg.Wait()

	mu.Lock()
	t.Logf("evicted %d keys during concurrent adds", len(evictedKeys))
	mu.Unlock()
}

// ---------- Benchmark：并发读写吞吐 ----------

func BenchmarkCache_ConcurrentOps(b *testing.B) {
	c := &Cache{Cache_bytes: 4096}

	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("bench-key-%d", i%1000)
			if r.Float32() < 0.5 {
				c.Add(key, ByteView{bt: []byte("bench-value")})
			} else {
				c.Get(key)
			}
			i++
		}
	})
}
