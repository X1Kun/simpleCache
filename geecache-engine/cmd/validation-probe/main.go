// validation-probe verifies values while a dedicated Kind cluster changes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

func main() {
	target := flag.String("url", "", "API origin URL")
	dir := flag.String("dir", "", "Artifact directory")
	flag.Parse()
	if *target == "" || *dir == "" {
		fmt.Fprintln(os.Stderr, "-url and -dir are required")
		os.Exit(2)
	}
	client := &http.Client{Timeout: 2 * time.Second}
	defer client.CloseIdleConnections()
	var mu sync.Mutex
	var durations []float64
	failures, wrong := 0, 0
	var examples []string
	start := time.Now()
	deadline := start.Add(3 * time.Minute)
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; time.Now().Before(deadline); i++ {
				if _, err := os.Stat(filepath.Join(*dir, "probe.stop")); err == nil {
					return
				}
				key := fmt.Sprintf("Auto-%d", (worker*2000+i)%10000)
				began := time.Now()
				response, err := client.Get(*target + "/api?key=" + key)
				bad, incorrect := false, false
				detail := ""
				if err != nil {
					bad = true
					detail = err.Error()
				} else {
					body, e := io.ReadAll(io.LimitReader(response.Body, 1<<20))
					response.Body.Close()
					bad = e != nil || response.StatusCode != 200
					incorrect = !bad && string(body) != "Value-for-"+key
					detail = fmt.Sprintf("key=%s status=%d body=%q read_error=%v", key, response.StatusCode, string(body), e)
				}
				mu.Lock()
				durations = append(durations, float64(time.Since(began).Microseconds())/1000)
				if bad {
					failures++
				}
				if incorrect {
					wrong++
				}
				if (bad || incorrect) && len(examples) < 10 {
					examples = append(examples, detail)
				}
				mu.Unlock()
				time.Sleep(100 * time.Millisecond)
			}
		}(worker)
	}
	wg.Wait()
	_, stopErr := os.Stat(filepath.Join(*dir, "probe.stop"))
	sort.Float64s(durations)
	p := func(q float64) float64 {
		if len(durations) == 0 {
			return 0
		}
		return durations[int(float64(len(durations)-1)*q)]
	}
	r := map[string]any{"environment": "dedicated Kind, stable entry Pod via port-forward", "workers": 4, "requests": len(durations), "errors": failures, "incorrect_values": wrong, "elapsed_seconds": time.Since(start).Seconds(), "p50_ms": p(.5), "p95_ms": p(.95), "p99_ms": p(.99), "failure_examples": examples}
	b, _ := json.MarshalIndent(r, "", "  ")
	if err := os.WriteFile(filepath.Join(*dir, "probe.json"), append(b, '\n'), 0644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	compact, _ := json.Marshal(r)
	fmt.Println("EVIDENCE " + string(compact))
	if len(durations) == 0 || failures != 0 || wrong != 0 || stopErr != nil {
		os.Exit(1)
	}
}
