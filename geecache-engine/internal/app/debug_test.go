package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/X1Kun/simpleCache/geecache-engine/internal/cache"
)

func TestDebugAddressIsOptInAndLoopbackOnly(t *testing.T) {
	for _, address := range []string{"", "127.0.0.1:0", "127.0.0.1:6060", "[::1]:6060"} {
		if err := validateDebugAddr(address); err != nil {
			t.Errorf("%q: %v", address, err)
		}
	}
	for _, address := range []string{":6060", "0.0.0.0:6060", "[::]:6060", "localhost:6060", "192.168.1.1:6060", "127.0.0.1:65536", "127.0.0.1:-1"} {
		if validateDebugAddr(address) == nil {
			t.Errorf("unsafe address accepted: %q", address)
		}
	}
	t.Setenv("DISCOVERY_MODE", "static")
	t.Setenv("SELF_ADDR", "")
	t.Setenv("PEERS", "")
	t.Setenv("DEBUG_ADDR", "")
	c, err := FromEnv(8001, true)
	if err != nil || c.DebugAddr != "" || c.ProfileContention {
		t.Fatalf("default debug configuration: %+v %v", c, err)
	}
	t.Setenv("DEBUG_ADDR", "0.0.0.0:6060")
	if _, err = FromEnv(8001, true); err == nil {
		t.Fatal("unsafe environment accepted")
	}
	if err = RunWithSource(context.Background(), Config{DebugAddr: "0.0.0.0:6060"}, nil); err == nil {
		t.Fatal("direct configuration bypassed debug validation")
	}
}

func TestProfilesAreAbsentFromPublicAPI(t *testing.T) {
	g := cache.NewGroup("test", 1024, cache.GetterFunc(func(context.Context, string) ([]byte, error) { return []byte("value"), nil }), cache.Options{})
	for _, path := range []string{"/debug/pprof/", "/debug/pprof/heap", "/debug/pprof/profile"} {
		w := httptest.NewRecorder()
		APIHandler(g, func() bool { return true }, nil).ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != http.StatusNotFound {
			t.Fatalf("public profile %s: %d", path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	DebugHandler().ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/debug/pprof/heap", nil))
	if w.Code != 200 || w.Body.Len() == 0 {
		t.Fatalf("debug heap profile: %d", w.Code)
	}
	w = httptest.NewRecorder()
	DebugHandler().ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/debug/pprof/heap", nil))
	if w.Code != 405 {
		t.Fatal("debug profile accepted POST")
	}
}
