package cache

import (
	lru "geecache/LRU"
	"sync"

)

type Cache struct {
	lru_cache *lru.Cache
	mu        sync.RWMutex
	Cache_bytes int64
}

func (c *Cache)Add(key string, value ByteView)  {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lru_cache == nil{
		c.lru_cache = lru.New(c.Cache_bytes,nil)
	}
	c.lru_cache.Add(key,value)
}

func (c *Cache)Get(key string)(ByteView,bool)  {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.lru_cache == nil{
		return ByteView{},false
	}
	if value,ok := c.lru_cache.Get(key);ok{
		return value.(ByteView),true
	}
	return ByteView{},false
}