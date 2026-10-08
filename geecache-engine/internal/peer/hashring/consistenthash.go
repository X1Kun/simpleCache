package hashring

import (
	"hash/crc32"
	"sort"
	"strconv"
)

// Hash maps bytes to a ring position.
type Hash func(data []byte) uint32

// Map implements a consistent hash ring.
type Map struct {
	hash     Hash           // Hash function for virtual nodes and keys.
	replicas int            // Virtual nodes per physical node.
	keys     []int          // Sorted virtual-node positions.
	hashMap  map[int]string // Ring position to physical-node mapping.
}

func New(replicas int, fn Hash) *Map {
	m := &Map{
		hash:     fn,
		replicas: replicas,
		hashMap:  make(map[int]string),
	}
	if m.hash == nil {
		m.hash = crc32.ChecksumIEEE
	}
	return m
}

// Add creates virtual nodes, records their owners, and sorts their positions.
func (m *Map) Add(keys ...string) {
	for _, key := range keys {
		for i := 0; i < m.replicas; i++ {
			// Hash each replica index followed by the node key.
			hash := int(m.hash([]byte(strconv.Itoa(i) + key)))
			// Append the virtual-node position.
			m.keys = append(m.keys, hash)
			// Associate the virtual-node position with its physical owner.
			m.hashMap[hash] = key
		}
	}
	sort.Ints(m.keys)
}

// Get finds the first ring position at or after the key hash and returns its owner.
func (m *Map) Get(key string) string {
	if len(m.keys) == 0 {
		return ""
	}
	hash := int(m.hash([]byte(key)))
	// Find the clockwise successor; modulo wraps past the final position.
	idx := sort.Search(len(m.keys), func(i int) bool {
		return m.keys[i] >= hash
	})
	// Resolve the selected virtual position to a physical owner.
	return m.hashMap[m.keys[idx%len(m.keys)]] // Keys beyond the final position wrap to the first node.
}
