// Package geecache 提供分布式缓存系统的核心接口定义
//
// 本文件定义了 GeeCache 系统中所有模块间通信的接口契约，
// 便于模块解耦和单元测试的 mock 实现。
package geecache

import (
	cache "geecache/Cache"
	pb "geecache/geecachepb"
)

// =============================================================================
// 缓存接口
// =============================================================================

// Cacher 定义缓存的基本操作接口
type Cacher interface {
	// Get 根据 key 获取缓存值
	// 返回 ByteView 和是否命中
	Get(key string) (cache.ByteView, bool)

	// Add 添加或更新缓存项
	Add(key string, value cache.ByteView)
}

// =============================================================================
// 节点选择接口
// =============================================================================

// PeerPicker 定义节点选择器接口
// 用于根据 key 选择合适的远程节点
type PeerPicker interface {
	// PickPeer 根据 key 选择一个远程节点
	// 返回 PeerGetter 接口和是否成功选择
	// 如果 key 应由本地节点处理，返回 nil, false
	PickPeer(key string) (peer PeerGetter, ok bool)
}

// PeerGetter 定义从远程节点获取数据的接口
type PeerGetter interface {
	// Get 从远程节点获取缓存数据
	// in: 请求参数（group 和 key）
	// out: 响应数据（value）
	Get(in *pb.Request, out *pb.Response) error
}

// =============================================================================
// 数据源回调接口
// =============================================================================

// Getter 定义缓存未命中时从数据源获取数据的接口
type Getter interface {
	// Get 根据 key 从数据源获取数据
	// 当缓存未命中时调用此方法
	Get(key string) ([]byte, error)
}

// GetterFunc 是 Getter 接口的函数适配器
// 允许将普通函数作为 Getter 使用
type GetterFunc func(key string) ([]byte, error)

// Get 实现 Getter 接口
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// =============================================================================
// 缓存组接口
// =============================================================================

// GroupManager 定义缓存组管理接口
type GroupManager interface {
	// Get 获取缓存值
	Get(key string) (cache.ByteView, error)

	// RegisterPeers 注册节点选择器
	RegisterPeers(peers PeerPicker)
}

// =============================================================================
// 一致性哈希接口
// =============================================================================

// HashFunc 定义哈希函数类型
type HashFunc func(data []byte) uint32

// ConsistentHash 定义一致性哈希接口
type ConsistentHash interface {
	// AddKeys 添加节点到哈希环
	AddKeys(keys ...string)

	// Get 根据 key 获取对应的节点
	Get(key string) string
}

// =============================================================================
// HTTP 服务接口
// =============================================================================

// HTTPServer 定义 HTTP 服务器接口
type HTTPServer interface {
	// Set 设置节点列表
	Set(peers ...string)

	// PickPeer 选择节点（实现 PeerPicker 接口）
	PickPeer(key string) (PeerGetter, bool)
}

// =============================================================================
// Singleflight 接口
// =============================================================================

// Caller 定义请求合并接口
// 确保对于相同的 key，并发请求只执行一次实际调用
type Caller interface {
	// Do 执行函数，相同 key 的并发调用会等待第一个调用完成并共享结果
	Do(key string, fn func() (interface{}, error)) (interface{}, error)
}

// =============================================================================
// 值类型接口
// =============================================================================

// Value 定义可计算长度的值接口
// LRU 缓存使用此接口计算缓存项大小
type Value interface {
	// Len 返回值的字节长度
	Len() int
}

// ByteViewer 定义只读字节视图接口
type ByteViewer interface {
	Value

	// ByteSlice 返回字节切片的副本
	ByteSlice() []byte

	// String 返回字符串形式
	String() string
}
