# SimpleCache

A read-only distributed cache written in Go, with a Kubernetes Operator. The cache engine currently supports reliable reads with static membership. Dynamic discovery and the remaining Operator improvements are planned.

See [Architecture](docs/ARCHITECTURE.md), [Design](DESIGN.md), and [CI](docs/CI.md).

## Features

- Local LRU caching with TTL, logical byte limits, and immutable value copies.
- A Bloom filter initialized from the complete read-only demo key set.
- Consistent-hash routing and single-hop HTTP/Protobuf peer reads.
- Separate SingleFlight groups for routing and actual source loads.
- Bounded peer requests, local source fallback, and typed peer errors.
- A shared per-process source concurrency limit; queueing counts toward the source deadline.
- Health/readiness endpoints, request metrics, and graceful shutdown.

Membership currently comes from static configuration and routing uses locks. EndpointSlice discovery, atomic routing snapshots, and complete Operator resource reconciliation are not part of the active version.

## Layout

```text
geecache-engine/
  cmd/simplecache/       CLI, signals, and startup
  internal/app/         Configuration, API, and HTTP lifecycle
  internal/cache/       Cache policy, request coalescing, source limits
  internal/peer/        HTTP clients/handlers and static routing
  internal/demo/        Finite read-only source
  internal/telemetry/   Per-process Prometheus registry
simplecache-operator/   Kubebuilder API, controller, and manifests
.github/workflows/     Repository-level CI
```

## Development checks

Use a Go toolchain compatible with Go 1.25.3, as required by the Operator. The engine module still declares Go 1.24.4.

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
| DISCOVERY_MODE | static; the active version only supports this mode |

## Deadlines and consistency

The API wait budget is 1500ms, shared routing work 1200ms, one peer request at most 300ms, and independent source work 800ms including queueing.

Caller cancellation stops that caller's wait. Shared work has its own bounded lifecycle. Source work may finish after a route times out and cache its successful result. Shutdown marks the server unready, drains active HTTP requests, then cancels shared work.

The demo source contains Tom, Jack, Sam, and Auto-0 through Auto-9999 with a simulated 100ms delay. It is not a real database. There is no write API, replication protocol, persistence, or strong consistency guarantee. SingleFlight and source limits operate per process.

Successful peer responses are not cached at the entry node. Fallback copies can remain until TTL or eviction. No current throughput or high-concurrency performance numbers are claimed.
