# SimpleCache design

Current state: reliable reads, static/EndpointSlice membership, atomic snapshots, and the Operator discovery/resource contract are implemented. API-server/race tests validate the controller; the original Kind integration smoke passed locally. Expanded scenarios generate independent run evidence. See [the plan](plan.md) and [README](../README.md).

## Responsibilities

The cache process handles reads, source protection, routing, and member discovery. The Operator expresses desired Kubernetes resources. Kubernetes maintains EndpointSlices; cache nodes consume them directly.

| Package | Responsibility |
| --- | --- |
| cmd/simplecache | CLI, process signals |
| internal/app | Configuration, API, listeners, shutdown |
| internal/cache | LRU/Bloom policy, coalescing, source limits |
| internal/peer | HTTP/Protobuf, routing snapshots, hashing |
| internal/discovery | Service-selected EndpointSlice informer |
| internal/demo | Finite immutable source |
| internal/telemetry | Per-process metrics registry |

Engine paths are relative to geecache-engine. The Operator retains Kubebuilder's api/cmd/internal/config layout. Each component has one Go module. cache owns source, peer, and observer interfaces without importing HTTP, Protobuf, or Kubernetes.

## Read path

API: validate key → local LRU/Bloom → routeFlight → preferred peer or local source.

Peer handlers call GetLocal only: local LRU/Bloom → originFlight → second lookup → source slot → Getter → cache result. They never forward to another peer.

Separate routing/source SingleFlight groups avoid recursive same-key waits. Caller cancellation stops its own wait; bounded shared work is not canceled by the first caller. Forget is not used for caller timeouts.

| Budget | Default |
| --- | --- |
| API wait | 1500ms |
| Shared route | 1200ms |
| One peer request | 300ms |
| Source work, including slot wait | 800ms |
| Concurrent Getter calls | 32 per process |

Fallback waits within the existing route budget. Independent source work may finish later within its own deadline and cache success. Source implementations must honor context cancellation. One limiter is shared across application groups, and coalesced waiters do not each acquire slots.

## Peer errors and size bounds

Success uses Protobuf. Only 404 plus KEY_NOT_FOUND is a terminal business miss. GROUP_NOT_FOUND, untyped 404, transport errors, invalid Protobuf, and oversized responses may fall back to the local source. Invalid input returns 400, source failures 503, and timeouts 504.

Keys contain 1–256 bytes. Values are limited to 1MiB; encoded bodies use a separate 1MiB+1024 bound. Client reads are bounded before decoding, and value size is checked again afterward. Path escaping preserves spaces, plus signs, slashes, and Unicode.

Peer success is not copied into the entry cache. Successful fallback values may remain locally until expiration or eviction.

## Membership and publication

A namespaced informer selects all IPv4 Pod EndpointSlices for the peer Service. It accepts ready=true or unknown readiness, rejects ready=false/terminating endpoints, and validates references, addresses, and the named TCP port.

Identity is namespace/podName; IP/port is transport. Replacing a Pod IP does not change ownership. Inputs are sorted/deduplicated deterministically; equal inputs do not republish. Hash positions and collision handling are deterministic.

A single worker builds a complete immutable ring/client/member snapshot and publishes it through atomic.Pointer. Readers load once and obtain owner and client from that same version. Published maps, slices, and ring data are never mutated.

Initial informer sync and valid publication are readiness dependencies; self-endpoint publication is not. A valid empty set publishes an empty ring and uses local source reads. Discovery failures retain the last valid view while bounded peer fallback covers staleness.

## Cache, lifecycle, and metrics

Bloom is built from the complete immutable key set before serving. Generic sources without that set disable it. Known keys must never be rejected; positive Bloom results still require lookup.

Default logical capacity is 64MiB and TTL 60s. Expiration is lazy; accounting covers key/value bytes, not RSS. Values are copied to isolate cached storage.

All listeners bind before readiness. Shutdown clears readiness, drains HTTP requests, then cancels shared work.

Metrics distinguish API/peer local lookups, actual peer requests, actual Getter calls, source-slot waits/inflight, Bloom rejection, fallback, and members/publications/errors. Waiters are not extra source loads. Labels do not include keys, arbitrary URLs, or error text.

## Operator integration

The controller now reconciles the dynamic discovery contract:

- Manage peer/API Services, StatefulSet, cache account, and namespace discovery Role/Binding.
- Handle conflicts/errors, repair managed drift, and watch children.
- Add resources, startup/readiness/liveness probes, and meaningful Status.
- Preserve allocated/immutable fields and avoid default-induced update loops.
- Remove PEERS from the template so scaling changes replicas only.
- Regenerate CRD/RBAC/DeepCopy and update envtest assertions.

Ready Status describes observed Kubernetes rollout, not simultaneous ring agreement. InvalidSpec, ResourceConflict and ReconcileFailed expose failures. Status patches occur only on actual changes. No external resources require a custom finalizer. Keep manual CR scaling until there is one authoritative autoscaling interface.

New StatefulSets use Parallel management; existing compatible immutable policy/identity fields are preserved. Resource defaults respect explicitly smaller limits, and invalid request/limit combinations prevent workload creation. Managed Pod fields are merged without erasing API defaults or unrelated fields. Static PEERS and SELF_ADDR are removed from the cache environment.

## Boundaries and evidence

The demo source is read-only and identical across nodes, with 100ms simulated latency. There is no write API, replication, persistence, active migration, or strong consistency. SingleFlight and source limits are process-local. Membership converges eventually.

Unit/race tests and fake discovery validate local contracts. Envtest additionally checks real API defaulting, allocated-field preservation, Status, conflicts, scaling without template changes, and Manager child-watch repair. It does not prove deployed permissions or Pod networking. The original dedicated Kind smoke passed locally. Expanded Kind tests require error-free reads during scaling/Pod replacement and recovery from infeasible resource requests; each run reports actual outcomes. Local diagnostics measure logical capacity, hash movement, source limits and latency, with test-process profiles rather than a publicly exposed admin endpoint. Grafana provisioning and deployed-process profiling remain separate follow-up work.

No throughput, GPU, scheduler-plugin, or production-availability claim follows from these tests. Keep advanced scheduling and AI workloads as separate follow-up work.

References: [EndpointSlices](https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/), [SingleFlight](https://pkg.go.dev/golang.org/x/sync/singleflight), [envtest](https://book.kubebuilder.io/reference/envtest).
