package app

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/X1Kun/simpleCache/geecache-engine/internal/cache"
	"github.com/X1Kun/simpleCache/geecache-engine/internal/demo"
	"github.com/X1Kun/simpleCache/geecache-engine/internal/discovery"
	"github.com/X1Kun/simpleCache/geecache-engine/internal/peer"
	"github.com/X1Kun/simpleCache/geecache-engine/internal/telemetry"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func APIHandler(group *cache.Group, ready func() bool, metrics *telemetry.Metrics) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		if !ready() {
			http.Error(w, "not ready", 503)
			return
		}
		w.WriteHeader(200)
	})
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		result := "error"
		defer func() {
			if metrics != nil {
				metrics.Observe("api", result, time.Since(start))
			}
		}()
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			http.Error(w, "method not allowed", 405)
			return
		}
		if !ready() {
			http.Error(w, "not ready", 503)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 1500*time.Millisecond)
		defer cancel()
		value, err := group.Get(ctx, r.URL.Query().Get("key"))
		result = cache.Result(err)
		if err != nil {
			http.Error(w, err.Error(), peer.Status(err))
			return
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(value.ByteSlice())
	})
	if metrics != nil {
		mux.Handle("/metrics", promhttp.HandlerFor(metrics.Registry, promhttp.HandlerOpts{}))
	}
	return mux
}
func Run(ctx context.Context, c Config) error {
	// The signal context controls shutdown; in-flight work survives the grace period.
	work, cancel := context.WithCancel(context.Background())
	defer cancel()
	router := peer.NewRouter(c.SelfID, 300*time.Millisecond)
	defer router.Close()
	if err := router.Update(c.Members); err != nil {
		return err
	}
	source := demo.New(100 * time.Millisecond)
	metrics := telemetry.New()
	metrics.Membership(len(router.Members()), false)
	group := cache.NewGroup("scores", c.CacheBytes, source, cache.Options{
		TTL: c.TTL, KnownKeys: source.Keys(), Peers: router, Lifecycle: work, Limiter: cache.NewSourceLimiter(c.SourceConcurrency), Observer: metrics,
	})
	var watcher *discovery.Watcher
	if c.DiscoveryMode == "kubernetes" {
		config, err := rest.InClusterConfig()
		if err != nil {
			return err
		}
		client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}
		watcher, err = discovery.New(client, c.Namespace, c.PeerService, router)
		if err != nil {
			return err
		}
		watcher.Report = metrics.Membership
		watcher.ReportError = metrics.DiscoveryError
	} else if c.DiscoveryMode != "static" {
		return errors.New("unsupported discovery mode")
	}
	var serving atomic.Bool
	ready := func() bool { return serving.Load() && (watcher == nil || watcher.Ready()) }
	servers := []*http.Server{{Addr: c.PeerAddr, Handler: peer.NewHandler(group), ReadHeaderTimeout: 3 * time.Second, IdleTimeout: 30 * time.Second}}
	if c.API {
		servers = append(servers, &http.Server{Addr: c.APIAddr, Handler: APIHandler(group, ready, metrics), ReadHeaderTimeout: 3 * time.Second, IdleTimeout: 30 * time.Second})
	}
	// Bind every port before declaring readiness or starting discovery.
	listeners := make([]net.Listener, 0, len(servers))
	for _, server := range servers {
		listener, err := net.Listen("tcp", server.Addr)
		if err != nil {
			for _, l := range listeners {
				_ = l.Close()
			}
			return err
		}
		listeners = append(listeners, listener)
	}
	if watcher != nil {
		go watcher.Run(work)
	}
	slog.Info("Cache server started", "identity", c.SelfID, "discovery", c.DiscoveryMode)
	return serve(ctx, servers, listeners, &serving)
}

// serve drains active HTTP requests before the caller stops shared source work.
func serve(ctx context.Context, servers []*http.Server, listeners []net.Listener, serving *atomic.Bool) error {
	errorsC := make(chan error, len(servers))
	for i, s := range servers {
		go func(server *http.Server, listener net.Listener) { errorsC <- server.Serve(listener) }(s, listeners[i])
	}
	serving.Store(true)
	var result error
	select {
	case <-ctx.Done():
	case err := <-errorsC:
		if !errors.Is(err, http.ErrServerClosed) {
			result = err
		}
	}
	serving.Store(false)
	shutdown, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	for _, server := range servers {
		if err := server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			if result == nil {
				result = err
			}
		}
	}
	return result
}
