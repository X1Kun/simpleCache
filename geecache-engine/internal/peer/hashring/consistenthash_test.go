package hashring

import (
	"fmt"
	"testing"
)

func TestMappingAndRemapping(t *testing.T) {
	a, b := New(100, nil), New(100, nil)
	a.Add("A", "B", "C")
	b.Add("C", "A", "B")
	moved := 0
	counts := map[string]int{}
	old := map[string]string{}
	for i := 0; i < 1000; i++ {
		key := fmt.Sprint(i)
		old[key] = a.Get(key)
		if old[key] != b.Get(key) {
			t.Fatal("order changed mapping")
		}
	}
	a.Add("D")
	for key, owner := range old {
		next := a.Get(key)
		counts[next]++
		if next != owner {
			moved++
			if next != "D" {
				t.Fatal("expansion moved a key between old nodes")
			}
		}
	}
	if moved == 0 || moved == 1000 {
		t.Fatalf("unexpected remapping: %d", moved)
	}
	t.Logf("moved=%d/1000, distribution=%v (no strict uniformity promise)", moved, counts)
}
func TestCollisionAndEmptyRing(t *testing.T) {
	hash := func([]byte) uint32 { return 1 }
	a, b := New(2, hash), New(2, hash)
	if a.Get("key") != "" {
		t.Fatal("empty ring has owner")
	}
	a.Add("B", "A")
	b.Add("A", "B")
	if a.Get("key") != "A" || b.Get("key") != "A" {
		t.Fatal("collision tie depends on input order")
	}
	a.Add("A")
	if len(a.keys) != 1 {
		t.Fatal("duplicate virtual positions")
	}
}
