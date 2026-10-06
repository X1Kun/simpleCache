package app

import (
	"errors"
	geecache "github.com/X1Kun/simpleCache/geecache-engine/internal/cache"
	"github.com/X1Kun/simpleCache/geecache-engine/internal/peer"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"log"
	"net/http"
)

func StartCacheServer(listenAddr string, peers *peer.HTTPPool) {
	log.Printf("geecache is listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, peers))
}

func StartAPIServer(listenAddr string, gee *geecache.Group) {
	http.Handle("/api", http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key")
			view, err := gee.Get(key)
			if err != nil {
				status := http.StatusInternalServerError
				if errors.Is(err, geecache.ErrNotFound) {
					status = http.StatusNotFound
				} else if errors.Is(err, geecache.ErrInvalidKey) {
					status = http.StatusBadRequest
				}
				http.Error(w, err.Error(), status)
				return
			}
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Write(view.ByteSlice())

		}))
	// 暴露 metrics 接口供 Prometheus 抓取
	http.Handle("/metrics", promhttp.Handler())

	log.Println("API server is running at", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))

}
