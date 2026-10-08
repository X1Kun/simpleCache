package hashring

import (
	"fmt"
	"hash/crc32"
	"math"
	"strconv"
	"testing"
)

// Check the fraction of keys remapped after adding a node.
// Adding a node should remap a subset of keys.
func TestConsistency_Migration(t *testing.T) {
	// Use the production CRC32 hash.
	hash := New(50, func(key []byte) uint32 {
		return crc32.ChecksumIEEE(key)
	})

	// Start with three nodes.
	hash.Add("Node-A", "Node-B", "Node-C")

	// Generate 10,000 in-memory keys.
	keys := make([]string, 10000)
	for i := 0; i < 10000; i++ {
		keys[i] = strconv.Itoa(i)
	}

	// Record each key's original owner.
	origLocation := make(map[string]string)
	for _, k := range keys {
		origLocation[k] = hash.Get(k)
	}

	// Add Node-D.
	hash.Add("Node-D")

	// Count keys whose owner changed.
	movedCount := 0
	for _, k := range keys {
		newLoc := hash.Get(k)
		if newLoc != origLocation[k] {
			movedCount++
		}
	}

	// Reference distribution:
	// The ring grows from three to four nodes.
	// An ideal uniform ring would move about one quarter of the keys to Node-D.
	// The remaining keys would keep their original owners.
	ratio := float64(movedCount) / float64(len(keys))

	t.Logf("Total Keys: 10000")
	t.Logf("Moved Keys: %d", movedCount)
	t.Logf("Migration Ratio: %.4f (Theory: 0.2500)", ratio)

	// Use the existing tolerance of 10 percentage points.
	if math.Abs(ratio-0.25) > 0.1 {
		t.Errorf("Migration ratio is too far from expected. Got %.4f, want ~0.25", ratio)
	}
}

// Check the distribution provided by virtual nodes.
// Inspect ownership distribution for a deterministic in-memory key set.
func TestHashing_LoadBalance(t *testing.T) {
	// A small virtual-node count may produce an uneven distribution.
	// This baseline uses 50 virtual nodes.
	virtualNodes := 50
	hash := New(virtualNodes, func(key []byte) uint32 {
		return crc32.ChecksumIEEE(key)
	})

	// Add three physical nodes.
	servers := []string{"Server-1", "Server-2", "Server-3"}
	hash.Add(servers...)

	// Map 100,000 keys locally; no requests are sent.
	requestCount := 100000
	serverCounts := make(map[string]int)

	for i := 0; i < requestCount; i++ {
		server := hash.Get(fmt.Sprintf("user_id_%d", i))
		serverCounts[server]++
	}

	// Report and validate the baseline distribution.
	t.Logf("Testing with %d virtual nodes per server...", virtualNodes)
	for _, server := range servers {
		count := serverCounts[server]
		ratio := float64(count) / float64(requestCount)
		t.Logf("[%s] hits: %d (%.2f%%)", server, count, ratio*100)

		// An ideal uniform ring would assign about one third to each node.
		// Retain the baseline acceptance interval of 25% to 40%.
		if ratio < 0.25 || ratio > 0.40 {
			t.Errorf("Server %s load is unbalanced! ratio: %.2f", server, ratio)
		}
	}
}
