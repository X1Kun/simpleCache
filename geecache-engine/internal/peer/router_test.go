package peer

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestAtomicPublicationWithConcurrentReaders(t *testing.T) {
	r := NewRouter("local", time.Second)
	defer r.Close()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 500; j++ {
				r.PickPeer("key")
				r.Members()
			}
		}()
	}
	for i := 0; i < 100; i++ {
		members := []Member{{ID: "local", Address: "127.0.0.1:8001"}, {ID: "remote", Address: "127.0.0.1:8002"}}
		if i%2 == 0 {
			members = members[:1]
		}
		if err := r.Update(members); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
	if err := r.Update(nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := r.PickPeer("key"); ok {
		t.Fatal("empty snapshot selected a peer")
	}
}

func TestAddressChangeKeepsOwnershipAndOldSnapshot(t *testing.T) {
	r := NewRouter("entry", time.Second)
	defer r.Close()
	initial := []Member{{ID: "demo/a", Address: "10.0.0.1:8001"}, {ID: "demo/b", Address: "10.0.0.2:8001"}}
	if err := r.Update(initial); err != nil {
		t.Fatal(err)
	}
	old := r.state.Load()
	owners := map[string]string{}
	for i := 0; i < 1000; i++ {
		key := fmt.Sprint(i)
		owners[key] = old.ring.Get(key)
	}
	initial[0].Address = "10.0.0.9:8001"
	if err := r.Update(initial); err != nil {
		t.Fatal(err)
	}
	current := r.state.Load()
	if old.clients["demo/a"].address != "10.0.0.1:8001" || current.clients["demo/a"].address != "10.0.0.9:8001" {
		t.Fatal("published snapshot mutated or transport not updated")
	}
	for key, owner := range owners {
		if current.ring.Get(key) != owner {
			t.Fatal("address change remapped ownership")
		}
	}
	if err := r.Update([]Member{initial[1], initial[0], initial[0]}); err != nil {
		t.Fatal(err)
	}
	if r.state.Load() != current {
		t.Fatal("equivalent member set rebuilt the ring")
	}
	copy := r.Members()
	copy[0].Address = "modified"
	if r.Members()[0].Address == "modified" {
		t.Fatal("member accessor exposes mutable storage")
	}
	if err := r.Update([]Member{{ID: "bad", Address: "10.0.0.3:invalid"}}); err == nil {
		t.Fatal("invalid address accepted")
	}
	if r.state.Load() != current {
		t.Fatal("invalid update replaced valid snapshot")
	}
}
