package peer

import (
	"errors"
	"github.com/X1Kun/simpleCache/geecache-engine/internal/cache"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPeerDoesNotEncodeSourceErrorAsSuccess(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"missing", cache.ErrNotFound, http.StatusNotFound},
		{"source-error", errors.New("source unavailable"), http.StatusInternalServerError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := "peer-" + tc.name
			cache.NewGroup(name, 1024, cache.GetterFunc(func(string) ([]byte, error) { return nil, tc.err }))
			pool := NewHTTPPool("http://localhost")
			request := httptest.NewRequest(http.MethodGet, "/_geecache/"+name+"/key", nil)
			recorder := httptest.NewRecorder()
			pool.ServeHTTP(recorder, request)
			if recorder.Code != tc.status {
				t.Fatalf("got %d want %d", recorder.Code, tc.status)
			}
		})
	}
}
