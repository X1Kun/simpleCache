package peer

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/X1Kun/simpleCache/geecache-engine/internal/cache"
	pb "github.com/X1Kun/simpleCache/geecache-engine/internal/peer/peerpb"
	"google.golang.org/protobuf/proto"
)

type forbiddenPicker struct{}

func (forbiddenPicker) PickPeer(string) (cache.PeerGetter, bool) {
	panic("Peer handler attempted to route")
}
func clientFor(server *httptest.Server) *Client {
	return &Client{
		address: strings.TrimPrefix(server.URL, "http://"), http: server.Client(), timeout: time.Second,
	}
}
func TestSingleHopEncodingAndValueBoundary(t *testing.T) {
	g := cache.NewGroup("group / \u4e2d\u6587", 2<<20, cache.GetterFunc(func(_ context.Context, key string) ([]byte, error) {
		if key == "max" {
			return bytes.Repeat([]byte("x"), cache.MaxValueBytes), nil
		}
		if key == "missing" {
			return nil, cache.ErrNotFound
		}
		return []byte(key), nil
	}), cache.Options{Peers: forbiddenPicker{}})
	server := httptest.NewServer(NewHandler(g))
	defer server.Close()
	c := clientFor(server)
	for _, key := range []string{"a b", "a+b", "a/b", "\u4e2d\u6587", "max"} {
		value, err := c.Get(context.Background(), g.Name(), key)
		if err != nil {
			t.Fatal(key, err)
		}
		if key == "max" {
			if len(value) != cache.MaxValueBytes {
				t.Fatal("max value truncated")
			}
		} else if string(value) != key {
			t.Fatalf("encoding: %q -> %q", key, value)
		}
	}
	if _, err := c.Get(context.Background(), g.Name(), "missing"); !errors.Is(err, cache.ErrNotFound) {
		t.Fatal(err)
	}
	if _, err := c.Get(context.Background(), "unknown", "key"); !errors.Is(err, ErrGroupNotFound) {
		t.Fatal(err)
	}
}
func TestFallbackAndNotFoundClassification(t *testing.T) {
	for _, tc := range []struct {
		name, code  string
		status      int
		wantCalls   int
		wantMissing bool
	}{
		{"key-missing", "KEY_NOT_FOUND", 404, 0, true}, {"group-missing", "GROUP_NOT_FOUND", 404, 1, false},
		{"untyped-404", "", 404, 1, false}, {"source-error", "", 503, 1, false}, {"invalid-body", "", 200, 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set(ErrorHeader, tc.code)
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte{255})
			}))
			defer server.Close()
			router := NewRouter("local", 100*time.Millisecond)
			defer router.Close()
			if err := router.Update([]Member{{ID: "remote", Address: strings.TrimPrefix(server.URL, "http://")}}); err != nil {
				t.Fatal(err)
			}
			calls := 0
			g := cache.NewGroup(t.Name(), 1024, cache.GetterFunc(func(context.Context, string) ([]byte, error) { calls++; return []byte("fallback"), nil }), cache.Options{Peers: router})
			value, err := g.Get(context.Background(), "key")
			if tc.wantMissing {
				if !errors.Is(err, cache.ErrNotFound) {
					t.Fatal(err)
				}
			} else if err != nil || value.String() != "fallback" {
				t.Fatal(err)
			}
			if calls != tc.wantCalls {
				t.Fatalf("calls=%d", calls)
			}
		})
	}
}
func TestSlowPeerAndOversizedBody(t *testing.T) {
	for _, slow := range []bool{true, false} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if slow {
				<-r.Context().Done()
				return
			}
			_, _ = w.Write(bytes.Repeat([]byte("x"), MaxBodyBytes+1))
		}))
		router := NewRouter("local", 40*time.Millisecond)
		if err := router.Update([]Member{{ID: "remote", Address: strings.TrimPrefix(server.URL, "http://")}}); err != nil {
			t.Fatal(err)
		}
		g := cache.NewGroup("fallback", 1024, cache.GetterFunc(func(context.Context, string) ([]byte, error) { return []byte("ok"), nil }), cache.Options{Peers: router})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		v, err := g.Get(ctx, "key")
		cancel()
		router.Close()
		server.Close()
		if err != nil || v.String() != "ok" {
			t.Fatal(err)
		}
	}
}

func TestRejectDecodedValueAboveLimit(t *testing.T) {
	body, err := proto.Marshal(&pb.Response{Value: make([]byte, cache.MaxValueBytes+1)})
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > MaxBodyBytes {
		t.Fatal("fixture must fit within the body limit")
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) }))
	defer server.Close()
	if _, err := clientFor(server).Get(context.Background(), "scores", "key"); !errors.Is(err, cache.ErrValueTooLarge) {
		t.Fatalf("decoded oversized value accepted: %v", err)
	}
}
