package groupcache

import (
	"fmt"
	"hash/crc32"
	"log"
	"net/http"
	"testing"
	"time"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

func createGroup() *Group {
	return NewGroup("scores", 2<<20, time.Duration(1024)*time.Second, GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

func startCacheServer(addr string, addrs []string, g *Group) {
	peers := NewBttcpPicker(addr)
	peers.Set(addrs...)
	g.RegisterPeers(peers)
	go peers.ListenAndServe()
}

func startAPIServer(apiAddr string, g *Group) {
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key")
			view, err := g.Get(key)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(view.ByteSlice())

		}))
	log.Println("fontend server is running at", apiAddr)
	log.Fatal(http.ListenAndServe(apiAddr[7:], nil))

}

// go mod tidy
func TestGroupCache(t *testing.T) {
	SetHashReplicas(100)
	SetHash(crc32.ChecksumIEEE)
	SetPoolSize(128)
	addrs := []string{"127.0.0.1:8001"}
	g := createGroup()
	startCacheServer(addrs[0], addrs, g)
	apiAddr := "http://127.0.0.1:9999"
	startAPIServer(apiAddr, g)
}
