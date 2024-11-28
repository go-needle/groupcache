package groupcache

import (
	"errors"
	"fmt"
	"github.com/go-needle/bttcp"
	"github.com/go-needle/groupcache/consistenthash"
	pb "github.com/go-needle/groupcache/groupcachepb"
	"google.golang.org/protobuf/proto"
	"hash/crc32"
	"log"
	"strconv"
	"strings"
	"sync"
)

var (
	defaultReplicas = 50
	defaultHash     = crc32.ChecksumIEEE
)

// BttcpPicker implements PeerPicker for a pool of bttcp peers.
type BttcpPicker struct {
	// this peer's base URL, e.g. "127.0.0.1:8000"
	self         string
	mu           sync.Mutex // guards peers and bttcpGetters
	peers        *consistenthash.Map
	bttcpGetters map[string]*bttcpGetter // keyed by e.g. "10.0.0.2:8008"
}

var _ PeerPicker = (*BttcpPicker)(nil)

// NewBttcpPicker initializes a bttcp picker of peers.
func NewBttcpPicker(self string) *BttcpPicker {
	return &BttcpPicker{
		self: self,
	}
}

// Log info with server name
func (p *BttcpPicker) Log(format string, v ...interface{}) {
	log.Printf("[Server %s] %s", p.self, fmt.Sprintf(format, v...))
}

// PickPeer picks a peer according to key
func (p *BttcpPicker) PickPeer(key string) (PeerGetter, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if peer := p.peers.Get(key); peer != "" && peer != p.self {
		p.Log("Pick peer %s", peer)
		return p.bttcpGetters[peer], true
	}
	return nil, false
}

// Set updates the pool's list of peers.
func (p *BttcpPicker) Set(peers ...string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.peers = consistenthash.New(defaultReplicas, defaultHash)
	p.peers.Add(peers...)
	p.bttcpGetters = make(map[string]*bttcpGetter, len(peers))
	for _, peer := range peers {
		p.bttcpGetters[peer] = &bttcpGetter{baseURL: peer, client: bttcp.NewClient(peer, 1024, false)}
	}
}

func SetHashReplicas(replicas int) {
	defaultReplicas = replicas
}

func SetHash(fn consistenthash.Hash) {
	defaultHash = fn
}

// ListenAndServe run a bttcp server on self address
func (p *BttcpPicker) ListenAndServe() {
	s := bttcp.NewServer(bttcp.HandlerFunc(func(b []byte) []byte {
		req := pb.Request{}
		err := proto.Unmarshal(b, &req)
		if err != nil {
			body, err := proto.Marshal(&pb.Response{Value: []byte(err.Error()), Code: 500})
			if err != nil {
				return nil
			}
			return body
		}
		group := GetGroup(req.Group)
		if group == nil {
			body, err := proto.Marshal(&pb.Response{Value: []byte("no group"), Code: 404})
			if err != nil {
				return nil
			}
			return body
		}
		view, err := group.Get(req.Key)
		if err != nil {
			body, err := proto.Marshal(&pb.Response{Value: []byte(err.Error()), Code: 500})
			if err != nil {
				return nil
			}
			return body
		}
		body, err := proto.Marshal(&pb.Response{Value: view.ByteSource(), Code: 200})
		if err != nil {
			return nil
		}
		p.Log("group: %s  key: %s", req.Group, req.Key)
		return body
	}))
	ports := strings.Split(p.self, ":")[1]
	port, err := strconv.Atoi(ports)
	if err != nil {
		panic(err)
	}
	s.Run(port)
}

type bttcpGetter struct {
	baseURL string
	client  *bttcp.Client
}

func (h *bttcpGetter) Get(group string, key string) ([]byte, error) {
	body, err := proto.Marshal(&pb.Request{Group: group, Key: key})
	if err != nil {
		return nil, err
	}
	b, err := h.client.Send(body)
	if err != nil {
		return nil, err
	}
	if b == nil {
		return nil, errors.New("no response from server")
	}
	res := pb.Response{}
	err = proto.Unmarshal(b, &res)
	if err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, errors.New(string(res.Value))
	}
	return res.Value, nil
}

var _ PeerGetter = (*bttcpGetter)(nil)

// BttcpClient is used to link the peer for groupcache
type BttcpClient struct {
	*bttcpGetter
}

func NewBttcpClient(addr string, poolSize int, isTestConn bool) *BttcpClient {
	return &BttcpClient{bttcpGetter: &bttcpGetter{baseURL: addr, client: bttcp.NewClient(addr, poolSize, isTestConn)}}
}
