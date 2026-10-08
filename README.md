# SimpleCache

A read-only distributed cache written in Go, with a Kubernetes Operator. The cache engine supports reliable reads, atomic routing snapshots, and static or EndpointSlice membership. The remaining Operator integration is planned.

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

Static mode remains available for local development. Kubernetes mode requires explicit Pod identity, a peer Service, and namespace EndpointSlice read permissions. The existing Operator still broadcasts static PEERS and does not yet supply the new discovery contract; real-cluster deployment is not claimed as verified.

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

The process needs namespace get/list/watch permissions for discovery.k8s.io/endpointslices. Do not set Kubernetes mode without completing its service/account/permission wiring. The next Operator pass supplies these resources and removes static PEERS from Pod templates.

## Deadlines and consistency

The API wait budget is 1500ms, shared routing work 1200ms, one peer request at most 300ms, and independent source work 800ms including queueing.

Caller cancellation stops that caller's wait. Shared work has its own bounded lifecycle. Source work may finish after a route times out and cache its successful result. Shutdown marks the server unready, drains active HTTP requests, then cancels shared work.

The demo source contains Tom, Jack, Sam, and Auto-0 through Auto-9999 with a simulated 100ms delay. It is not a real database. There is no write API, replication protocol, persistence, or strong consistency guarantee. SingleFlight and source limits operate per process.

Successful peer responses are not cached at the entry node. Fallback copies can remain until TTL or eviction. No current throughput or high-concurrency performance numbers are claimed.

## CI

The root [CI workflow](.github/workflows/ci.yml) runs on main pushes, pull requests targeting main, and manual dispatch. Quality and Operator envtest are independent jobs. They cover formatting, vet, race/coverage, binaries, Compose parsing, generated-file consistency, and Kustomize rendering; coverage is uploaded as artifacts.

The separate [container workflow](.github/workflows/containers.yml) builds both images on main pushes or manual dispatch without publishing. Neither workflow starts Kind or runs heavy load tests.

Envtest uses an isolated API server/etcd and explicitly cleans child resources. It verifies the current controller contract, not deployed permissions, workload readiness, or real-cluster convergence. Container workflows are configured, but local Docker build verification was interrupted and is not claimed as passed.
