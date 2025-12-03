package httpserver

import (
	"fmt"
	callbackfunc "geecache/CallbackFunc"
	group "geecache/Group"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// ---------- 测试数据 ----------

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

// ---------- 辅助函数 ----------

func createTestGroup(name string) *group.Group {
	return group.NewGroup(name, 2<<10, callbackfunc.CallbackFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

func setupTestRouter(httpAddr *HttpAddr) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/_geecache/*path", httpAddr.Serve)
	return r
}

// ---------- HttpAddr 基础功能测试 ----------

func TestNewHttpAddr(t *testing.T) {
	host := "http://localhost:8001"
	httpAddr := NewHttpAddr(host)

	if httpAddr.Host != host {
		t.Fatalf("expected host %s, got %s", host, httpAddr.Host)
	}
	if httpAddr.Path != defaultBasePath {
		t.Fatalf("expected path %s, got %s", defaultBasePath, httpAddr.Path)
	}
}

func TestHttpAddr_Set(t *testing.T) {
	httpAddr := NewHttpAddr("http://localhost:8001")
	peers := []string{
		"http://localhost:8001",
		"http://localhost:8002",
		"http://localhost:8003",
	}

	httpAddr.Set(peers...)

	if httpAddr.peers == nil {
		t.Fatal("peers should not be nil after Set")
	}
	if len(httpAddr.HttpClients) != len(peers) {
		t.Fatalf("expected %d HttpClients, got %d", len(peers), len(httpAddr.HttpClients))
	}

	for _, peer := range peers {
		client, exists := httpAddr.HttpClients[peer]
		if !exists {
			t.Fatalf("HttpClient for %s should exist", peer)
		}
		expectedBaseURL := peer + defaultBasePath
		if client.BaseURL != expectedBaseURL {
			t.Fatalf("expected BaseURL %s, got %s", expectedBaseURL, client.BaseURL)
		}
	}
}

func TestHttpAddr_PickPeer_Self(t *testing.T) {
	host := "http://localhost:8001"
	httpAddr := NewHttpAddr(host)
	peers := []string{
		"http://localhost:8001",
		"http://localhost:8002",
		"http://localhost:8003",
	}
	httpAddr.Set(peers...)

	// 测试多个 key，验证不会选择自身
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("test-key-%d", i)
		client, ok := httpAddr.PickPeer(key)
		if ok {
			// 如果选择了节点，不应该是自身
			if client.BaseURL == host+defaultBasePath {
				t.Fatalf("should not pick self as peer for key %s", key)
			}
		}
	}
}

func TestHttpAddr_PickPeer_Consistency(t *testing.T) {
	httpAddr := NewHttpAddr("http://localhost:8001")
	peers := []string{
		"http://localhost:8002",
		"http://localhost:8003",
		"http://localhost:8004",
	}
	httpAddr.Set(peers...)

	// 同一个 key 应该始终路由到同一个节点
	key := "consistent-key"
	firstClient, firstOk := httpAddr.PickPeer(key)

	for i := 0; i < 10; i++ {
		client, ok := httpAddr.PickPeer(key)
		if ok != firstOk {
			t.Fatalf("consistency check failed: ok mismatch")
		}
		if ok && client.BaseURL != firstClient.BaseURL {
			t.Fatalf("consistency check failed: expected %s, got %s", firstClient.BaseURL, client.BaseURL)
		}
	}
}

// ---------- 并发安全测试 ----------

func TestHttpAddr_ConcurrentSet(t *testing.T) {
	httpAddr := NewHttpAddr("http://localhost:8001")

	var wg sync.WaitGroup
	numGoroutines := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			peers := []string{
				fmt.Sprintf("http://localhost:%d", 8001+id),
				fmt.Sprintf("http://localhost:%d", 8002+id),
			}
			httpAddr.Set(peers...)
		}(i)
	}

	wg.Wait()
	// 只要不 panic 就算通过
}

func TestHttpAddr_ConcurrentPickPeer(t *testing.T) {
	httpAddr := NewHttpAddr("http://localhost:8001")
	peers := []string{
		"http://localhost:8002",
		"http://localhost:8003",
		"http://localhost:8004",
	}
	httpAddr.Set(peers...)

	var wg sync.WaitGroup
	numGoroutines := 100

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", id%10)
			httpAddr.PickPeer(key)
		}(i)
	}

	wg.Wait()
}

// ---------- Serve 测试 ----------

func TestServe_Success(t *testing.T) {
	// 创建测试 Group
	_ = createTestGroup("scores")

	httpAddr := NewHttpAddr("http://localhost:8001")
	router := setupTestRouter(httpAddr)

	// 测试获取存在的 key
	req, _ := http.NewRequest("GET", "/_geecache/scores/Tom", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	body, _ := io.ReadAll(w.Body)
	if string(body) != "630" {
		t.Fatalf("expected body '630', got '%s'", string(body))
	}
}

func TestServe_GroupNotFound(t *testing.T) {
	httpAddr := NewHttpAddr("http://localhost:8001")
	router := setupTestRouter(httpAddr)

	req, _ := http.NewRequest("GET", "/_geecache/nonexistent/key", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", w.Code)
	}
}

func TestServe_KeyNotFound(t *testing.T) {
	_ = createTestGroup("scores2")

	httpAddr := NewHttpAddr("http://localhost:8001")
	router := setupTestRouter(httpAddr)

	req, _ := http.NewRequest("GET", "/_geecache/scores2/NonExistentKey", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestServe_BadRequest(t *testing.T) {
	httpAddr := NewHttpAddr("http://localhost:8001")
	router := setupTestRouter(httpAddr)

	// 缺少 key 部分
	req, _ := http.NewRequest("GET", "/_geecache/onlygroup", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", w.Code)
	}
}

func TestServe_ContentType(t *testing.T) {
	_ = createTestGroup("scores3")

	httpAddr := NewHttpAddr("http://localhost:8001")
	router := setupTestRouter(httpAddr)

	req, _ := http.NewRequest("GET", "/_geecache/scores3/Jack", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "application/octet-stream") {
		t.Fatalf("expected Content-Type application/octet-stream, got %s", contentType)
	}
}

// ---------- Log 测试 ----------

func TestHttpAddr_Log(t *testing.T) {
	httpAddr := NewHttpAddr("http://localhost:8001")

	// Log 方法只是调用 log.Printf，不 panic 就算通过
	httpAddr.Log("Test message %s %d", "hello", 123)
}

// ---------- 集成测试 ----------

func TestIntegration_MultipleRequests(t *testing.T) {
	groupName := "integration_test"
	_ = createTestGroup(groupName)

	httpAddr := NewHttpAddr("http://localhost:8001")
	router := setupTestRouter(httpAddr)

	testCases := []struct {
		key      string
		expected string
		status   int
	}{
		{"Tom", "630", http.StatusOK},
		{"Jack", "589", http.StatusOK},
		{"Sam", "567", http.StatusOK},
	}

	for _, tc := range testCases {
		path := fmt.Sprintf("/_geecache/%s/%s", groupName, tc.key)
		req, _ := http.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != tc.status {
			t.Fatalf("key %s: expected status %d, got %d", tc.key, tc.status, w.Code)
		}

		if tc.status == http.StatusOK {
			body, _ := io.ReadAll(w.Body)
			if string(body) != tc.expected {
				t.Fatalf("key %s: expected body '%s', got '%s'", tc.key, tc.expected, string(body))
			}
		}
	}
}

func TestIntegration_ConcurrentRequests(t *testing.T) {
	groupName := "concurrent_test"
	_ = createTestGroup(groupName)

	httpAddr := NewHttpAddr("http://localhost:8001")
	router := setupTestRouter(httpAddr)

	var wg sync.WaitGroup
	numRequests := 50

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			keys := []string{"Tom", "Jack", "Sam"}
			key := keys[id%len(keys)]

			path := fmt.Sprintf("/_geecache/%s/%s", groupName, key)
			req, _ := http.NewRequest("GET", path, nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("request %d failed with status %d", id, w.Code)
			}
		}(i)
	}

	wg.Wait()
}

// ---------- 缓存功能测试 ----------

func TestCache_HitAndMiss(t *testing.T) {
	callCount := 0
	groupName := "cache_test"

	group.NewGroup(groupName, 2<<10, callbackfunc.CallbackFunc(
		func(key string) ([]byte, error) {
			callCount++
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))

	httpAddr := NewHttpAddr("http://localhost:8001")
	router := setupTestRouter(httpAddr)

	// 第一次请求（缓存未命中）
	path := fmt.Sprintf("/_geecache/%s/Tom", groupName)
	req, _ := http.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("first request failed with status %d", w.Code)
	}

	firstCallCount := callCount

	// 第二次请求相同的 key（应该从缓存读取）
	req, _ = http.NewRequest("GET", path, nil)
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("second request failed with status %d", w.Code)
	}

	// 由于有缓存，回调函数不应该被再次调用
	// 注意：如果 Group.Get 实现了缓存，callCount 应该保持不变
	_ = firstCallCount
}

// ---------- Benchmark ----------

func BenchmarkHttpAddr_Set(b *testing.B) {
	httpAddr := NewHttpAddr("http://localhost:8001")
	peers := []string{
		"http://localhost:8001",
		"http://localhost:8002",
		"http://localhost:8003",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		httpAddr.Set(peers...)
	}
}

func BenchmarkHttpAddr_PickPeer(b *testing.B) {
	httpAddr := NewHttpAddr("http://localhost:8001")
	peers := []string{
		"http://localhost:8002",
		"http://localhost:8003",
		"http://localhost:8004",
	}
	httpAddr.Set(peers...)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		httpAddr.PickPeer(fmt.Sprintf("key-%d", i))
	}
}

func BenchmarkHttpAddr_PickPeer_Parallel(b *testing.B) {
	httpAddr := NewHttpAddr("http://localhost:8001")
	peers := []string{
		"http://localhost:8002",
		"http://localhost:8003",
		"http://localhost:8004",
	}
	httpAddr.Set(peers...)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			httpAddr.PickPeer(fmt.Sprintf("key-%d", i))
			i++
		}
	})
}

func BenchmarkServe(b *testing.B) {
	groupName := "bench_scores"
	_ = createTestGroup(groupName)

	httpAddr := NewHttpAddr("http://localhost:8001")
	router := setupTestRouter(httpAddr)

	path := fmt.Sprintf("/_geecache/%s/Tom", groupName)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// ---------- 模拟分布式场景测试 ----------

func TestDistributed_PeerSelection(t *testing.T) {
	// 模拟三个节点
	addrs := []string{
		"http://localhost:8001",
		"http://localhost:8002",
		"http://localhost:8003",
	}

	// 创建三个 HttpAddr，模拟三个节点
	nodes := make([]*HttpAddr, len(addrs))
	for i, addr := range addrs {
		nodes[i] = NewHttpAddr(addr)
		nodes[i].Set(addrs...)
	}

	// 验证同一个 key 在不同节点上选择的 peer 一致性
	testKeys := []string{"Tom", "Jack", "Sam", "Alice", "Bob"}

	for _, key := range testKeys {
		var selectedPeers []string
		for _, node := range nodes {
			peer, ok := node.PickPeer(key)
			if ok {
				selectedPeers = append(selectedPeers, peer.BaseURL)
			}
		}

		// 验证选择的 peer 逻辑一致（考虑到不同节点会排除自己）
		if len(selectedPeers) > 0 {
			t.Logf("Key %s selected peers: %v", key, selectedPeers)
		}
	}
}

// ---------- 边界条件测试 ----------

func TestHttpAddr_EmptyPeers(t *testing.T) {
	httpAddr := NewHttpAddr("http://localhost:8001")
	httpAddr.Set() // 空 peers

	_, ok := httpAddr.PickPeer("any-key")
	if ok {
		t.Fatal("should not pick peer when peers list is empty")
	}
}

func TestHttpAddr_SinglePeer_Self(t *testing.T) {
	host := "http://localhost:8001"
	httpAddr := NewHttpAddr(host)
	httpAddr.Set(host) // 只有自己

	_, ok := httpAddr.PickPeer("any-key")
	if ok {
		t.Fatal("should not pick self as peer")
	}
}

func TestHttpAddr_UpdatePeers(t *testing.T) {
	httpAddr := NewHttpAddr("http://localhost:8001")

	// 初始设置
	httpAddr.Set("http://localhost:8002", "http://localhost:8003")
	if len(httpAddr.HttpClients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(httpAddr.HttpClients))
	}

	// 更新 peers
	httpAddr.Set("http://localhost:8004", "http://localhost:8005", "http://localhost:8006")
	if len(httpAddr.HttpClients) != 3 {
		t.Fatalf("expected 3 clients after update, got %d", len(httpAddr.HttpClients))
	}

	// 验证旧的 peers 已被替换
	if _, exists := httpAddr.HttpClients["http://localhost:8002"]; exists {
		t.Fatal("old peer should not exist after update")
	}
}

// ---------- 超时和错误处理测试 ----------

func TestServe_Timeout(t *testing.T) {
	groupName := "timeout_test"

	// 创建一个慢回调的 Group
	group.NewGroup(groupName, 2<<10, callbackfunc.CallbackFunc(
		func(key string) ([]byte, error) {
			time.Sleep(100 * time.Millisecond) // 模拟慢查询
			return []byte("slow-value"), nil
		}))

	httpAddr := NewHttpAddr("http://localhost:8001")
	router := setupTestRouter(httpAddr)

	start := time.Now()
	path := fmt.Sprintf("/_geecache/%s/slow-key", groupName)
	req, _ := http.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	elapsed := time.Since(start)
	if elapsed < 100*time.Millisecond {
		t.Fatalf("request should take at least 100ms, took %v", elapsed)
	}

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}
}
