package hashring

import (
	"hash/crc32"
	"sort"
	"strconv"
)

type Hash func([]byte) uint32
type Map struct {
	hash     Hash
	replicas int
	keys     []uint32
	owners   map[uint32]string
}

func New(replicas int, hash Hash) *Map {
	if replicas <= 0 {
		panic("replicas must be positive")
	}
	if hash == nil {
		hash = crc32.ChecksumIEEE
	}
	return &Map{hash: hash, replicas: replicas, owners: make(map[uint32]string)}
}
func (m *Map) Add(nodes ...string) {
	for _, node := range nodes {
		for i := 0; i < m.replicas; i++ {
			hash := m.hash([]byte(strconv.Itoa(i) + ":" + node))
			if old, exists := m.owners[hash]; exists {
				if node < old {
					m.owners[hash] = node
				}
			} else {
				m.keys = append(m.keys, hash)
				m.owners[hash] = node
			}
		}
	}
	sort.Slice(m.keys, func(i, j int) bool { return m.keys[i] < m.keys[j] })
}
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}
	hash := m.hash([]byte(key))
	i := sort.Search(len(m.keys), func(i int) bool { return m.keys[i] >= hash })
	return m.owners[m.keys[i%len(m.keys)]]
}
