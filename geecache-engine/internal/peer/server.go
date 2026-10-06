package peer

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/X1Kun/simpleCache/geecache-engine/internal/cache"
	pb "github.com/X1Kun/simpleCache/geecache-engine/internal/peer/peerpb"
	"google.golang.org/protobuf/proto"
)

type Handler struct{ groups map[string]*cache.Group }

func NewHandler(groups ...*cache.Group) *Handler {
	h := &Handler{groups: make(map[string]*cache.Group)}
	for _, g := range groups {
		if _, exists := h.groups[g.Name()]; exists {
			panic("duplicate group")
		}
		h.groups[g.Name()] = g
	}
	return h
}
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", 405)
		return
	}
	path := r.URL.EscapedPath()
	if !strings.HasPrefix(path, BasePath) {
		http.NotFound(w, r)
		return
	}
	parts := strings.SplitN(strings.TrimPrefix(path, BasePath), "/", 2)
	if len(parts) != 2 {
		http.Error(w, "bad peer path", 400)
		return
	}
	group, e1 := url.PathUnescape(parts[0])
	key, e2 := url.PathUnescape(parts[1])
	if e1 != nil || e2 != nil {
		http.Error(w, "bad encoding", 400)
		return
	}
	g, ok := h.groups[group]
	if !ok {
		w.Header().Set(ErrorHeader, "GROUP_NOT_FOUND")
		http.Error(w, "group not found", 404)
		return
	}
	value, err := g.GetLocal(r.Context(), key)
	if err != nil {
		if errors.Is(err, cache.ErrNotFound) {
			w.Header().Set(ErrorHeader, "KEY_NOT_FOUND")
		}
		http.Error(w, err.Error(), Status(err))
		return
	}
	message := &pb.Response{Value: value.ByteSlice()}
	if proto.Size(message) > MaxBodyBytes {
		http.Error(w, "peer body too large", 500)
		return
	}
	body, err := proto.Marshal(message)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(body)
}
