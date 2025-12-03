package group

import (
	cache "geecache/Cache"
	callbackfunc "geecache/CallbackFunc"
	pickpeer "geecache/PickPeer"
	singleflight "geecache/SingleFlight"
	pb "geecache/geecachepb"
	"log"
	"sync"
)

type Group struct {
	cache  *cache.Cache
	f      callbackfunc.CallbackFunc
	name   string
	peers  pickpeer.PeerPicker
	loader *singleflight.Group
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

func NewGroup(name string, cache_bytes int64, f callbackfunc.CallbackFunc) *Group {
	if f == nil {
		panic("should need callback function")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		cache: &cache.Cache{
			Cache_bytes: cache_bytes,
		},
		f:      f,
		name:   name,
		loader: &singleflight.Group{},
	}
	groups[name] = g
	return g
}

func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}
func (g *Group) RegisterPeers(peers pickpeer.PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

func (g *Group) Get(key string) (cache.ByteView, error) {
	view, err := g.loader.Do(key, func() (interface{}, error) {
		if g.peers != nil {
			if peer, ok := g.peers.PickPeer(key); ok {
				if bytes, err := g.getFromPeer(peer, key); err == nil {
					return cache.NewByteView(bytes), nil
				}
				log.Println("[GeeCache] Failed to get from peer", peer)
			}
		}
		// 从回调函数获取数据，需要转换为 ByteView
		bytes, err := g.f(key)
		if err != nil {
			return cache.ByteView{}, err
		}
		return cache.NewByteView(bytes), nil
	})
	if err != nil {
		return cache.ByteView{}, err
	}
	return view.(cache.ByteView), nil
}
func (g *Group) getFromPeer(peer pickpeer.PeerGetter, key string) ([]byte, error) {
	req := &pb.Request{
		Group: g.name,
		Key:   key,
	}
	res := &pb.Response{}
	err := peer.Get(req, res)
	if err != nil {
		return nil, err
	}
	return res.Value, nil
}
