package main

import (
	"flag"
	"fmt"
	"github.com/X1Kun/simpleCache/geecache-engine/internal/app"
	geecache "github.com/X1Kun/simpleCache/geecache-engine/internal/cache"
	"github.com/X1Kun/simpleCache/geecache-engine/internal/demo"
	"github.com/X1Kun/simpleCache/geecache-engine/internal/peer"
	"os"
	"strings"
	"time"
)

func createGroup() *geecache.Group {
	source := demo.New(100 * time.Millisecond)
	return geecache.NewGroupWithOptions("scores", 64<<20, source, geecache.GroupOptions{
		TTL:       time.Minute,
		KnownKeys: source.Keys(),
	})
}

func main() {
	var port int
	var api bool
	flag.IntVar(&port, "port", 8002, "Geecache server port")
	flag.BoolVar(&api, "api", false, "Start a api server?")
	flag.Parse()

	selfAddr := os.Getenv("SELF_ADDR")
	if selfAddr == "" {
		// 降级方案：如果没有环境变量，默认回退到本地测试模式
		selfAddr = fmt.Sprintf("http://localhost:%d", port)
	}

	peersEnv := os.Getenv("PEERS")
	var peersList []string
	if peersEnv != "" {
		peersList = strings.Split(peersEnv, ",")
	} else {
		// 降级方案：本地单机测时的假数据
		peersList = []string{"http://localhost:8001", "http://localhost:8002", "http://localhost:8003"}
	}

	gee := createGroup()
	peers := peer.NewHTTPPool(selfAddr)
	peers.Set(peersList...)
	gee.RegisterPeers(peers)
	if api {
		// API 暴露给外部用户，监听 0.0.0.0
		go app.StartAPIServer("0.0.0.0:9999", gee)
	}

	listenAddr := fmt.Sprintf("0.0.0.0:%d", port)
	app.StartCacheServer(listenAddr, peers)
}
