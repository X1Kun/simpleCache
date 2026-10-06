package cache

import (
	"hash/fnv"
	"math"
)

type BloomFilter struct {
	bitset []uint64 // 位图（BitMap）
	size   uint64   // 位图的长度
	hashes uint     // 使用的哈希函数的数量
}

func NewBloomFilter(size uint64, hashes uint) *BloomFilter {
	if size == 0 || hashes == 0 {
		panic("Bloom filter size and hash count must be positive")
	}
	return &BloomFilter{
		// Round up to whole uint64 words without allocating an extra word at multiples of 64.
		bitset: make([]uint64, (size-1)/64+1),
		size:   size,
		hashes: hashes,
	}
}

// newBloomForKeys builds the whole filter before it is used by request handlers.
// Add must not run concurrently with Contains.
func newBloomForKeys(keys []string) *BloomFilter {
	n := float64(len(keys))
	bits := uint64(math.Ceil(-n * math.Log(0.01) / (math.Ln2 * math.Ln2)))
	if bits < 64 {
		bits = 64
	}
	hashes := uint(1)
	if n > 0 {
		hashes = uint(math.Max(1, math.Round(float64(bits)/n*math.Ln2)))
	}
	bf := NewBloomFilter(bits, hashes)
	for _, key := range keys {
		bf.Add(key)
	}
	return bf
}

func bloomHashes(key string) (uint64, uint64) {
	h1, h2 := fnv.New64a(), fnv.New64()
	_, _ = h1.Write([]byte(key))
	_, _ = h2.Write([]byte(key))
	return h1.Sum64(), h2.Sum64() | 1
}

// Add 将一个 Key 的“指纹”录入布隆过滤器
func (bf *BloomFilter) Add(key string) {
	h1, h2 := bloomHashes(key)
	for i := uint(0); i < bf.hashes; i++ {
		// 计算出当前哈希函数对应的 bit 位置
		idx := (h1 + uint64(i)*h2) % bf.size
		// 把对应的 bit 位标记为 1
		// idx/64 找到是哪个 uint64， idx%64 找到是这个 uint64 里的第几个 bit
		bf.bitset[idx/64] |= 1 << (idx % 64)
	}
}

// Contains 判断一个 Key 是否“可能存在”
func (bf *BloomFilter) Contains(key string) bool {
	h1, h2 := bloomHashes(key)
	for i := uint(0); i < bf.hashes; i++ {
		idx := (h1 + uint64(i)*h2) % bf.size

		// 检查对应的 bit 位是不是 0
		// 如果有任何一个哈希函数算出来的位置是 0，说明这个 Key 绝对不存在！
		if bf.bitset[idx/64]&(1<<(idx%64)) == 0 {
			return false
		}
	}
	// 所有的位置都是 1，说明它“大概率”存在
	return true
}
