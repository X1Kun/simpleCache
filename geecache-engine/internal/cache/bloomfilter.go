package cache

import (
	"hash/fnv"
	"math"
)

type BloomFilter struct {
	bitset []uint64 // Bitmap storage.
	size   uint64   // Number of bits.
	hashes uint     // Number of hash probes.
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

// Add sets the bits associated with a key.
func (bf *BloomFilter) Add(key string) {
	h1, h2 := bloomHashes(key)
	for i := uint(0); i < bf.hashes; i++ {
		// Compute the bit position for this hash probe.
		idx := (h1 + uint64(i)*h2) % bf.size
		// Set the bit.
		// Select the uint64 word and the bit within that word.
		bf.bitset[idx/64] |= 1 << (idx % 64)
	}
}

// Contains reports whether a key may exist.
func (bf *BloomFilter) Contains(key string) bool {
	h1, h2 := bloomHashes(key)
	for i := uint(0); i < bf.hashes; i++ {
		idx := (h1 + uint64(i)*h2) % bf.size

		// Check whether the corresponding bit is unset.
		// An unset bit proves absence from the initialized key set.
		if bf.bitset[idx/64]&(1<<(idx%64)) == 0 {
			return false
		}
	}
	// All bits are set; the key may exist.
	return true
}
