# SimpleCache

A read-only distributed cache written in Go, with a Kubernetes Operator. The engine supports reliable reads and atomic EndpointSlice membership; the Operator provides declarative resources, discovery permissions, probes, and rollout Status. The original dedicated Kind smoke passed locally; the expanded adversarial suite records its own results per run.

See [Design](docs/design.md) for architecture and constraints, and [Plan](docs/plan.md) for remaining work.

## Features

- Local LRU caching with TTL, logical byte limits, and immutable value copies.
- A Bloom filter initialized from the complete read-only demo key set.
- Consistent-hash routing and single-hop HTTP/Protobuf peer reads.
- Immutable routing snapshots, stable Pod identity independent of IP, and ready EndpointSlice discovery.
- Separate SingleFlight groups for routing and actual source loads.
- Bounded peer requests, local source fallback, and typed peer errors.
- A shared per-process source concurrency limit; queueing counts toward the source deadline.
- Health/readiness endpoints, request metrics, and graceful shutdown.
- Idempotent Operator reconciliation, child-resource watches, replica-only scaling, and conflict/rollout conditions.

Static mode remains available for local development. Kubernetes mode uses Pod identity, the governing peer Service, and a cache-specific ServiceAccount with namespace EndpointSlice permissions. The Operator supplies this contract without broadcasting PEERS in Pod templates.

## Layout

```text
geecache-engine/
  cmd/simplecache/       CLI, signals, and startup
  internal/app/         Configuration, API, and HTTP lifecycle
  internal/cache/       Cache policy, request coalescing, source limits
  internal/peer/        HTTP clients/handlers and atomic routing snapshots
  internal/discovery/   Ready EndpointSlice membership
  internal/demo/        Finite read-only source
  internal/telemetry/   Per-process Prometheus registry
simplecache-operator/   Kubebuilder API, controller, and manifests
.github/workflows/     Repository-level CI
```

## Development checks

Use a Go toolchain compatible with Go 1.25.3. Both modules use the same Kubernetes dependency generation.

```bash
make check             # Formatting, engine tests/race, both modules' build/vet
make test-ci           # Uncached engine race tests with coverage
make test-operator-ci  # Controller envtest with race detection and coverage
make compose-config    # Parse the local Compose configuration
make generated-check  # Compare generated CRD/RBAC/DeepCopy with source
make k8s-render        # Render Operator deployment and sample manifests
make validate          # Engine race checks, adversarial diagnostics and profiles
```

Envtest downloads temporary API-server and etcd binaries. It does not deploy cache Pods or use your existing cluster. Heavy load tests are excluded from these checks.

## Run a local node

```bash
cd geecache-engine
go run -buildvcs=false ./cmd/simplecache -port=8001 -api=true
```

Without a configured peer list, static membership contains only this node. Configure SELF_ADDR and comma-separated PEERS HTTP URLs for a multi-node demo. The CLI defaults remain peer port 8002 and API disabled; the command above sets both explicitly.

```bash
curl -i 'http://localhost:9999/api?key=Auto-666'
curl -i 'http://localhost:9999/api?key=missing'
curl -i 'http://localhost:9999/readyz'
curl -s 'http://localhost:9999/metrics'
```

Known keys return their values. Missing keys return 404; empty or oversized keys return 400. Peer failures fall back when the source is available and the remaining budget permits it. Source failures return 503 and timeouts return 504.

| Environment variable | Default |
| --- | --- |
| CACHE_BYTES | 64MiB of logical key/value bytes |
| TTL_SECONDS | 60 |
| SOURCE_MAX_CONCURRENCY | 32 per process |
| API_ADDR | 0.0.0.0:9999 |
| DISCOVERY_MODE | static; kubernetes requires POD_NAME, POD_NAMESPACE, PEER_SERVICE |

## Kubernetes discovery

The informer selects every IPv4 EndpointSlice for PEER_SERVICE in POD_NAMESPACE, excludes unready/terminating Pods, and atomically publishes a complete routing view. Unknown readiness follows the EndpointSlice compatibility convention. A namespace/Pod name is the ring identity; IP/port is the transport address.

Readiness requires a first successful informer sync and valid publication, but does not wait for this Pod to appear in its own endpoints. An empty healthy set uses the local source. Discovery failures retain the last valid view; stale peer calls remain bounded and can fall back.

The process needs namespace get/list/watch permissions for discovery.k8s.io/endpointslices. The Operator reconciles those permissions, a headless peer Service, an API ClusterIP Service, and the StatefulSet. Configure size/image/cacheBytes/ttlSeconds/resources through the CR. Scaling changes only replicas; image or cache settings can roll Pods. Allocated and immutable Kubernetes fields are preserved.

## Isolated Kubernetes smoke

```bash
make kind-up     # Build/load local images and deploy to simplecache-stage4
make kind-smoke  # Concurrent reads, repeated scaling, repair, scheduling failure/recovery
make kind-delete # Explicit cleanup of that dedicated cluster only
```

If the default Go module proxy is unreachable during image builds, run
`GOPROXY=https://goproxy.cn,direct make kind-up`. The script forwards GOPROXY
to both Docker builds; setting it only in the host's Go configuration does not
configure the build containers. Module checksum verification remains enabled.

The scripts use bin/kind/kubeconfig and an explicit context, without changing the user's normal kubeconfig. They do not touch dev-cluster or orion-live. Repeated kind-up restarts only this project's test workloads to load rebuilt dev tags. This setup uses one physical host and does not prove multi-machine availability.

The original smoke passed in the user's local terminal: reads, discovery RBAC, scaling without restarting retained Pods, child repair, and Pod recovery. The expanded smoke deliberately modifies the dedicated test CR, requires an initial healthy three-node cluster, and restores its original spec on normal success/failure. Hard termination or host failure may require manual recovery. It never targets another cluster.

## Repeatable validation reports

Run `make validate` for local correctness/race checks and short diagnostics, and
`make kind-smoke` for the deployed-cluster scenarios. Each invocation creates a
unique `bin/reports/<UTC timestamp>-<mode>-<random>/` directory, already ignored by
Git. Both success and failure retain `report.md`, `report.json`, raw logs and
stage exit codes. Skipped tests are shown separately, not counted as passes.
The root make test, race, test-ci, test-operator and test-operator-ci targets also
generate separate reports. Operator runs retain ginkgo.json, raw envtest logs and
coverage. Counts come from actual Ginkgo It specs, excluding suite hooks and the
Go suite entry point; pending specs are shown separately from skipped specs.
Count sources are explicit. Uncollected counts display N/A (JSON null), not zero.
New Operator runs require structured evidence; missing or corrupt reports fail
validation. Legacy logs without structured evidence cannot reconstruct counts.
Direct go test commands and module-local Makefiles do not use this wrapper.

Local scenarios cover 512 same-key waiters, 256 distinct keys with an eight-slot
source limit, deadline overload/recovery, eviction/reload of an oversized working
set, stale peer timeout/recovery, and deterministic 100,000-key expansion mapping.
The 1/4 expansion fraction is a reference, not an exact pass threshold.
Three one-second HTTP diagnostics report P50/P95/P99, errors and source calls;
they use loopback and a synthetic 2ms source, not the deployed 100ms demo source.

Local runs also save coverage, the scenario test binary, CPU/heap/mutex/block
profiles, and CPU/heap top summaries. Inspect them with
`go tool pprof -http=127.0.0.1:0 <run>/scenarios.test <run>/cpu.pprof`.
These are test-process profiles; no public pprof endpoint is enabled.
Race-stage timings are correctness observations, not performance baselines.

Kind runs read continuously through a retained Pod while two scaling cycles and
owner Pod replacement execute. Zero HTTP errors and zero incorrect values are
required. A separate impossible CPU request must produce an Unschedulable Pod
and Ready=False, then recover after restoring the CR. The request cannot allocate
host resources; rollback explicitly removes only Pending test Pods to avoid
StatefulSet's unready bad-revision rollback stall. It does not claim that the
Operator autonomously recovers every invalid rollout or survives host-wide pressure.
Cluster snapshots, events, Operator logs, Pod metrics and probe percentiles are
saved for diagnosis.

Logical cache bytes, capacity, entries and removals are exposed as metrics.
Logical bytes exclude Go object overhead and RSS; removals include lazy expiry.
No throughput improvement is claimed without a comparable measured change.

## Deadlines and consistency

The API wait budget is 1500ms, shared routing work 1200ms, one peer request at most 300ms, and independent source work 800ms including queueing.

Caller cancellation stops that caller's wait. Shared work has its own bounded lifecycle. Source work may finish after a route times out and cache its successful result. Shutdown marks the server unready, drains active HTTP requests, then cancels shared work.

The demo source contains Tom, Jack, Sam, and Auto-0 through Auto-9999 with a simulated 100ms delay. It is not a real database. There is no write API, replication protocol, persistence, or strong consistency guarantee. SingleFlight and source limits operate per process.

Successful peer responses are not cached at the entry node. Fallback copies can remain until TTL or eviction. No current throughput or high-concurrency performance numbers are claimed.

## CI

The root [CI workflow](.github/workflows/ci.yml) runs on main pushes, pull requests targeting main, and manual dispatch. Quality and Operator envtest are independent jobs. They cover formatting, vet, race/coverage, adversarial diagnostics/profiles, binaries, Compose parsing, generated-file consistency, and Kustomize rendering. Local validation reports/profiles and Operator coverage are uploaded even on failure.

The separate [container workflow](.github/workflows/containers.yml) builds both images on main pushes or manual dispatch without publishing. Neither workflow starts Kind or runs heavy load tests.

Envtest uses an isolated API server/etcd and explicitly cleans child resources. It verifies the current controller contract, not deployed permissions, workload readiness, or real-cluster convergence. Both local images were built for the original successful Kind smoke. Hosted CI results remain separate evidence.
