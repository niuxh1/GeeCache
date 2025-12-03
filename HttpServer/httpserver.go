package httpserver

import (
	consistenthash "geecache/ConsistentHash"
	httpclient "geecache/HttpClient"
	"sync"
)

var defaultBasePath = "/_geecache/"

const num = 50

type HttpAddr struct {
	Host string
	Path string
	mu   sync.Mutex
	peers *consistenthash.Map
	HttpClients map[string]*httpclient.HttpClient
}

func NewHttpAddr(host string) *HttpAddr {
	return &HttpAddr{
		Host: host,
		Path: defaultBasePath,
	}
}


func (p *HttpAddr) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(num, nil)
	p.peers.AddKeys(peers...)
	p.HttpClients = make(map[string]*httpclient.HttpClient,len(peers))
	for _, peer := range peers {
		p.HttpClients[peer] = &httpclient.HttpClient{BaseURL: peer + p.Path}
	}
}


func (p *HttpAddr) PickPeer(key string) (*httpclient.HttpClient, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.Host {
		p.Log("Pick peer %s", peer)
		return p.HttpClients[peer], true
	}
	return nil, false
}