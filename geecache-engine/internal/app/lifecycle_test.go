package app

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/X1Kun/simpleCache/geecache-engine/internal/cache"
)

func TestShutdownDrainsActiveRequest(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	started, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	g := cache.NewGroup("shutdown", 1024, cache.GetterFunc(func(ctx context.Context, _ string) ([]byte, error) {
		close(started)
		select {
		case <-release:
			return []byte("value"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}), cache.Options{SourceTimeout: 2 * time.Second})
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		value, err := g.GetLocal(r.Context(), "key")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_, _ = w.Write(value.ByteSlice())
	})}
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var ready atomic.Bool
	done := make(chan error, 1)
	go func() { done <- serve(ctx, []*http.Server{server}, []net.Listener{listener}, &ready) }()
	type outcome struct {
		status int
		body   string
		err    error
	}
	response := make(chan outcome, 1)
	go func() {
		client := &http.Client{Timeout: 3 * time.Second}
		res, err := client.Get("http://" + listener.Addr().String())
		if err != nil {
			response <- outcome{err: err}
			return
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		response <- outcome{res.StatusCode, string(body), err}
	}()
	select {
	case <-started:
	case <-time.After(3 * time.Second):
		t.Fatal("request did not enter source")
	}
	cancel()
	deadline := time.After(time.Second)
	for ready.Load() {
		select {
		case <-deadline:
			t.Fatal("shutdown did not clear readiness")
		case <-time.After(time.Millisecond):
		}
	}
	unblock()
	select {
	case got := <-response:
		if got.err != nil || got.status != 200 || got.body != "value" {
			t.Fatalf("active request aborted: %+v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("request did not complete")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("shutdown did not finish")
	}
}
