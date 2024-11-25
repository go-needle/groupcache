package groupcache

import (
	"fmt"
	"github.com/go-needle/groupcache/consistenthash"
	"log"
	"sync"
)

const (
	defaultReplicas = 50
)

// BttcpPool implements PeerPicker for a pool of bttcp peers.
type BttcpPool struct {
	// this peer's base URL, e.g. "127.0.0.1:8000"
	self         string
	mu           sync.Mutex // guards peers and bttcpGetters
	peers        *consistenthash.Map
	bttcpGetters map[string]*bttcpGetter // keyed by e.g. "10.0.0.2:8008"
}

// NewBttcpPool initializes a bttcp pool of peers.
func NewBttcpPool(self string) *BttcpPool {
	return &BttcpPool{
		self: self,
	}
}

// Log info with server name
func (p *BttcpPool) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// PickPeer picks a peer according to key
func (p *BttcpPool) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.bttcpGetters[peer], true
	}
	return nil, false
}

// Set updates the pool's list of peers.
func (p *BttcpPool) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(defaultReplicas, nil)
	p.peers.Add(peers...)
	p.bttcpGetters = make(map[string]*bttcpGetters, len(peers))
	for _, peer := range peers {
		p.bttcpGetters[peer] = &bttcpGetters{baseURL: peer}
	}
}
