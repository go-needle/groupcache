package groupcache

import (
	"fmt"
	"github.com/go-needle/cache"
	"log"
	"sync"
	"time"
)

// A Getter loads data for a key.
type Getter interface {
	Get(key string) ([]byte, error)
}

// GetterFunc loads data for a key.
type GetterFunc func(key string) ([]byte, error)

// Get implements Getter interface function
func (f GetterFunc) Get(key string) ([]byte, error) {
	return f(key)
}

// A Group is a cache namespace and associated data loaded spread over
type Group struct {
	name      string
	getter    Getter
	mainCache cache.Cache
	peers     PeerPicker
}

var (
	mu     sync.RWMutex
	groups = make(map[string]*Group)
)

// NewGroup create a new instance of Group
func NewGroup(name string, cacheBytes int64, keySurvivalTime time.Duration, getter Getter) *Group {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()
	g := &Group{
		name:      name,
		getter:    getter,
		mainCache: cache.NewLRU(cacheBytes, keySurvivalTime),
	}
	groups[name] = g
	return g
}

// GetGroup returns the named group previously created with NewGroup, or
// nil if there's no such group.
func GetGroup(name string) *Group {
	mu.RLock()
	g := groups[name]
	mu.RUnlock()
	return g
}

// Get value for a key from cache
func (g *Group) Get(key string) (cache.ByteView, error) {
	if key == "" {
		return cache.NewByteView(nil), fmt.Errorf("key is required")
	}

	if v, ok := g.mainCache.Get(key); ok {
		log.Println("[cache] hit")
		return v, nil
	}
	v, err := g.load(key)
	if err != nil {
		return cache.NewByteView(nil), err
	}
	return cache.NewByteView(v), nil
}

// RegisterPeers registers a PeerPicker for choosing remote peer
func (g *Group) RegisterPeers(peers PeerPicker) {
	if g.peers != nil {
		panic("RegisterPeerPicker called more than once")
	}
	g.peers = peers
}

func (g *Group) load(key string) (value []byte, err error) {
	if g.peers != nil {
		if peer, ok := g.peers.PickPeer(key); ok {
			if value, err = g.getFromPeer(peer, key); err == nil {
				return value, nil
			}
			log.Println("[GeeCache] Failed to get from peer", err)
		}
	}

	return g.getLocally(key)
}

func (g *Group) getFromPeer(peer PeerGetter, key string) ([]byte, error) {
	bytes, err := peer.Get(g.name, key)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}

func (g *Group) getLocally(key string) ([]byte, error) {
	bytes, err := g.getter.Get(key)
	if err != nil {
		return nil, err
	}
	v := cache.CloneBytes(bytes)
	g.mainCache.Add(key, v)
	return v, nil
}
