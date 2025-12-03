package lru

import (
	"container/list"
)


type Cache struct {
	maxBytes int64
	nbytes   int64
	ll       *list.List
	cache    map[string]*list.Element

	OnEvicted func(key string) ([]byte ,error)
}

type entry struct {
	key   string
	value Value
}


type Value interface {
	Len() int
}

func New(maxBytes int64, onEvicted func(key string) ([]byte ,error)) *Cache {
	return &Cache{
		maxBytes:  maxBytes,
		ll:        list.New(),
		cache : make(map[string]*list.Element),
		OnEvicted: onEvicted,
	}
}

func (c *Cache) Get(key string) (Value,bool){
	
	if element,ok := c.cache[key];ok{
		c.ll.MoveToFront(element)
		kv := element.Value.(*entry)
		return kv.value,true
	}
	return nil,false
}

func (c *Cache) Delete()  {
	element := c.ll.Back()
	if element != nil {
		c.ll.Remove(element)
		kv := element.Value.(*entry)
		delete(c.cache, kv.key)
		c.nbytes -= int64(len(kv.key)) + int64(kv.value.Len())
		if c.OnEvicted != nil {
			c.OnEvicted(kv.key)
		}
	}
}

func (c *Cache) Add(key string, value Value)  {
	if element,ok := c.cache[key];ok{
		c.ll.MoveToFront(element)
		kv := element.Value.(*entry)
		c.nbytes += int64(value.Len()) - int64(kv.value.Len())
		kv.value = value
	}else{
		element := c.ll.PushFront(&entry{key,value})
		c.cache[key] = element
		c.nbytes += int64(len(key)) + int64(value.Len())
	}
	for c.maxBytes != 0 && c.nbytes > c.maxBytes {
		c.Delete()
	}
}

func (c *Cache) Len() int {
	return c.ll.Len()
}