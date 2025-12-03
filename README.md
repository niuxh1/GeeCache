# GeeCache

一个基于 Go 语言实现的分布式缓存系统

## 特性

- **LRU 缓存淘汰策略** - 基于最近最少使用算法的内存管理
- **一致性哈希** - 实现分布式节点的负载均衡和数据分片
- **Singleflight** - 防止缓存击穿，合并并发请求
- **HTTP 通信** - 节点间基于 HTTP 协议通信
- **Protobuf 序列化** - 高效的二进制数据传输格式
- **并发安全** - 全面的锁机制保证线程安全

## 项目结构

```
GeeCache/
├── Cache/              # 缓存核心模块
│   ├── ByteView.go     # 只读字节视图（防止缓存值被修改）
│   └── LruCache.go     # 并发安全的 LRU 缓存封装
├── CallbackFunc/       # 回调函数定义
│   └── callback.go     # 缓存未命中时的数据获取函数
├── ConsistentHash/     # 一致性哈希
│   └── Hash.go         # 一致性哈希环实现
├── Group/              # 缓存组
│   └── group.go        # 缓存命名空间，支持多个独立缓存
├── HttpClient/         # HTTP 客户端
│   └── httpclient.go   # 节点间通信客户端
├── HttpServer/         # HTTP 服务端
│   ├── httpserver.go   # 节点管理和路由
│   └── serve.go        # HTTP 请求处理
├── LRU/                # LRU 算法
│   └── lru.go          # 最近最少使用淘汰算法
├── PickPeer/           # 节点选择接口
│   └── Picker.go       # PeerPicker 和 PeerGetter 接口
├── SingleFlight/       # 请求合并
│   └── singleflight.go # 防止缓存击穿
├── geecachepb/         # Protobuf 定义
│   ├── geecachepb.proto
│   └── geecachepb.pb.go
└── go.mod
```

## 快速开始

### 安装

```bash
go get geecache
```

### 基本使用

```go
package main

import (
    "fmt"
    "log"
  
    callbackfunc "geecache/CallbackFunc"
    group "geecache/Group"
    httpserver "geecache/HttpServer"
  
    "github.com/gin-gonic/gin"
)

// 模拟数据库
var db = map[string]string{
    "Tom":  "630",
    "Jack": "589",
    "Sam":  "567",
}

func main() {
    // 创建缓存组
    g := group.NewGroup("scores", 2<<10, callbackfunc.CallbackFunc(
        func(key string) ([]byte, error) {
            log.Println("[DB] search key", key)
            if v, ok := db[key]; ok {
                return []byte(v), nil
            }
            return nil, fmt.Errorf("%s not found", key)
        },
    ))
  
    // 创建 HTTP 服务
    addr := "http://localhost:8001"
    peers := httpserver.NewHttpAddr(addr)
    peers.Set(
        "http://localhost:8001",
        "http://localhost:8002",
        "http://localhost:8003",
    )
    g.RegisterPeers(peers)
  
    // 启动 Gin 服务器
    r := gin.Default()
    r.GET("/_geecache/*path", peers.Serve)
    r.Run(":8001")
}
```

### 分布式部署

启动多个节点：

```bash
# 节点 1
go run main.go -port=8001

# 节点 2
go run main.go -port=8002

# 节点 3
go run main.go -port=8003
```

### API 请求

```bash
# 获取缓存
curl http://localhost:8001/_geecache/scores/Tom
```

## 核心模块说明

### 1. LRU 缓存 (`LRU/lru.go`)

基于双向链表和哈希表实现的 LRU 缓存：

```go
cache := lru.New(1024, nil)  // 最大 1024 字节
cache.Add("key", value)
value, ok := cache.Get("key")
```

### 2. 一致性哈希 (`ConsistentHash/Hash.go`)

用于分布式场景下的节点选择：

```go
m := consistenthash.New(50, nil)  // 50 个虚拟节点
m.AddKeys("node1", "node2", "node3")
node := m.Get("my-key")  // 返回负责该 key 的节点
```

### 3. Singleflight (`SingleFlight/singleflight.go`)

防止缓存击穿，合并相同 key 的并发请求：

```go
var g singleflight.Group
result, err := g.Do("key", func() (interface{}, error) {
    // 只会执行一次，其他相同请求等待结果
    return fetchFromDB("key")
})
```

### 4. 缓存组 (`Group/group.go`)

命名空间隔离的缓存实例：

```go
g := group.NewGroup("users", 2<<20, callbackFunc)
value, err := g.Get("user:123")
```

## 架构图

```
                            ┌─────────────────┐
                            │   Client        │
                            └────────┬────────┘
                                     │
                                     ▼
┌─────────────┐    HTTP    ┌─────────────────┐    HTTP    ┌─────────────┐
│   Node 1    │◄──────────►│     Node 2      │◄──────────►│   Node 3    │
│             │            │                 │            │             │
│ ┌─────────┐ │            │ ┌─────────────┐ │            │ ┌─────────┐ │
│ │  Cache  │ │            │ │    Cache    │ │            │ │  Cache  │ │
│ └─────────┘ │            │ └─────────────┘ │            │ └─────────┘ │
│      │      │            │       │         │            │      │      │
│      ▼      │            │       ▼         │            │      ▼      │
│ ┌─────────┐ │            │ ┌─────────────┐ │            │ ┌─────────┐ │
│ │   LRU   │ │            │ │     LRU     │ │            │ │   LRU   │ │
│ └─────────┘ │            │ └─────────────┘ │            │ └─────────┘ │
└─────────────┘            └─────────────────┘            └─────────────┘
        │                          │                             │
        └──────────────────────────┼─────────────────────────────┘
                                   │
                          Consistent Hash
                         (数据分片 & 负载均衡)
```

## 请求流程

1. 客户端发起请求 `GET /_geecache/{group}/{key}`
2. 当前节点检查本地缓存
3. 如果缓存未命中，通过一致性哈希选择负责该 key 的节点
4. 如果是远程节点，发起 HTTP 请求获取数据
5. 如果是本地节点，调用回调函数从数据源获取
6. 使用 Singleflight 防止并发请求击穿
7. 返回结果并缓存

## 依赖

- [Gin](https://github.com/gin-gonic/gin) - HTTP Web 框架
- [Protocol Buffers](https://protobuf.dev/) - 数据序列化

## 测试

```bash
# 运行所有测试
go test -v ./...

# 运行特定包测试
go test -v ./HttpServer/...

# 运行基准测试
go test -bench=. ./...
```

## 许可证

MIT License
