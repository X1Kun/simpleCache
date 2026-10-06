package cache

import pb "github.com/X1Kun/simpleCache/geecache-engine/internal/peer/peerpb"

type PeerPicker interface {
	PickPeer(key string) (peer PeerGetter, ok bool) // 根据数据标签获取peer(url+获取数据方法)
}

type PeerGetter interface {
	// Get(group string, key string) ([]byte, error) // 根据缓存组名和key获取数据
	Get(in *pb.Request, out *pb.Response) error
}
