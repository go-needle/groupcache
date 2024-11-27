# groupcache
Distributed caching library based on protobuf, go-needle-btcp, go-needle-cache



### quickly start
```golang
// main.go
package main

import (
	"flag"
	"fmt"
	"github.com/go-needle/groupcache"
	"log"
	"net/http"
	"time"
)

var db = map[string]string{
	"Tom":  "630",
	"Jack": "589",
	"Sam":  "567",
}

var db2 = map[string]string{
	"Tom":  "15",
	"Jack": "17",
	"Sam":  "15",
}

func createScoreGroup() *groupcache.Group {
	return groupcache.NewGroup("score", 2<<10, time.Duration(1024)*time.Second, groupcache.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

func createAgeGroup() *groupcache.Group {
	return groupcache.NewGroup("age", 2<<10, time.Duration(1024)*time.Second, groupcache.GetterFunc(
		func(key string) ([]byte, error) {
			log.Println("[SlowDB] search key", key)
			if v, ok := db2[key]; ok {
				return []byte(v), nil
			}
			return nil, fmt.Errorf("%s not exist", key)
		}))
}

func startCacheServer(addr string, addrs []string, groups []*groupcache.Group) {
	peers := groupcache.NewBttcpPicker(addr)
	peers.Set(addrs...)
	for _, group := range groups {
		group.RegisterPeers(peers)
	}
	peers.ListenAndServe()
}

func startAPIServer(peerAddr, apiAddr string) {
	client := groupcache.NewBttcpClient(peerAddr, 128, true)
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			group := r.URL.Query().Get("group")
			key := r.URL.Query().Get("key")
			get, err := client.Get(group, key)
			if err != nil {
				fmt.Println(err)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(get)

		}))
	log.Println("frontend server is running at", apiAddr)
	log.Fatal(http.ListenAndServe(apiAddr, nil))

}

func main() {
	var port int
	var api bool
	flag.IntVar(&port, "port", 8001, "groupcache server port")
	flag.BoolVar(&api, "api", false, "Start a api server?")
	flag.Parse()

	if api {
		startAPIServer("127.0.0.1:8001", "0.0.0.0:9999")
	}

	addrMap := map[int]string{
		8001: "127.0.0.1:8001",
		8002: "127.0.0.1:8002",
		8003: "127.0.0.1:8003",
	}

	var addrs []string
	for _, v := range addrMap {
		addrs = append(addrs, v)
	}

	groups := []*groupcache.Group{createAgeGroup(), createScoreGroup()}
	startCacheServer(addrMap[port], addrs, groups)
}
```