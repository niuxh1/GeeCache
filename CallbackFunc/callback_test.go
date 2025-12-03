package callbackfunc

import (
	"errors"
	"sync"
	"testing"
)

// ---------- 基础功能测试 ----------

func TestCallbackFunc_Basic(t *testing.T) {
	// 定义一个简单的回调
	cb := CallbackFunc(func(key string) ([]byte, error) {
		return []byte("value-" + key), nil
	})

	result, err := cb.Get("test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(result) != "value-test" {
		t.Fatalf("expected 'value-test', got '%s'", result)
	}
}

func TestCallbackFunc_ReturnsError(t *testing.T) {
	expectedErr := errors.New("key not found")

	cb := CallbackFunc(func(key string) ([]byte, error) {
		return nil, expectedErr
	})

	result, err := cb.Get("missing")
	if err != expectedErr {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
	if result != nil {
		t.Fatalf("expected nil result, got %v", result)
	}
}

func TestCallbackFunc_NilCallback(t *testing.T) {
	var cb CallbackFunc = nil

	// 调用 nil 函数会 panic，这里验证行为
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic when calling nil CallbackFunc")
		}
	}()

	cb.Get("key")
}

// ---------- 不同数据源模拟 ----------

func TestCallbackFunc_MapDataSource(t *testing.T) {
	// 模拟从 map 获取数据
	dataStore := map[string][]byte{
		"user:1": []byte(`{"id":1,"name":"Alice"}`),
		"user:2": []byte(`{"id":2,"name":"Bob"}`),
	}

	cb := CallbackFunc(func(key string) ([]byte, error) {
		if v, ok := dataStore[key]; ok {
			return v, nil
		}
		return nil, errors.New("not found")
	})

	// 命中
	if v, err := cb.Get("user:1"); err != nil || string(v) != `{"id":1,"name":"Alice"}` {
		t.Fatalf("user:1 lookup failed: %v, %s", err, v)
	}

	// 未命中
	if _, err := cb.Get("user:999"); err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestCallbackFunc_SlowDataSource(t *testing.T) {
	// 模拟慢数据源（如数据库）
	callCount := 0

	cb := CallbackFunc(func(key string) ([]byte, error) {
		callCount++
		// 模拟耗时操作（实际不 sleep，只计数）
		return []byte("slow-" + key), nil
	})

	_, _ = cb.Get("k1")
	_, _ = cb.Get("k2")
	_, _ = cb.Get("k1") // 重复调用

	if callCount != 3 {
		t.Fatalf("expected 3 calls, got %d", callCount)
	}
}

// ---------- 并发安全测试 ----------

func TestCallbackFunc_ConcurrentAccess(t *testing.T) {
	var mu sync.Mutex
	accessLog := make([]string, 0)

	cb := CallbackFunc(func(key string) ([]byte, error) {
		mu.Lock()
		accessLog = append(accessLog, key)
		mu.Unlock()
		return []byte("v-" + key), nil
	})

	const numGoroutines = 50
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			key := "key"
			if id%2 == 0 {
				key = "even"
			} else {
				key = "odd"
			}
			_, _ = cb.Get(key)
		}(i)
	}

	wg.Wait()

	if len(accessLog) != numGoroutines {
		t.Fatalf("expected %d accesses, got %d", numGoroutines, len(accessLog))
	}
}

// ---------- Get 方法等价性测试 ----------

func TestCallbackFunc_GetEqualsDirectCall(t *testing.T) {
	cb := CallbackFunc(func(key string) ([]byte, error) {
		return []byte("direct-" + key), nil
	})

	// 通过 Get 方法调用
	r1, e1 := cb.Get("test")
	// 直接调用函数
	r2, e2 := cb("test")

	if string(r1) != string(r2) || e1 != e2 {
		t.Fatalf("Get() and direct call should be equivalent")
	}
}

// ---------- Benchmark ----------

func BenchmarkCallbackFunc_Get(b *testing.B) {
	cb := CallbackFunc(func(key string) ([]byte, error) {
		return []byte("benchmark-value"), nil
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = cb.Get("bench-key")
	}
}

func BenchmarkCallbackFunc_ConcurrentGet(b *testing.B) {
	cb := CallbackFunc(func(key string) ([]byte, error) {
		return []byte("concurrent-value"), nil
	})

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cb.Get("parallel-key")
		}
	})
}
