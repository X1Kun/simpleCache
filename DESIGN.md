# SimpleCache design

Status: target design with an implemented static-membership read path. Read README.md for active capabilities and docs/CI.md for the checks actually enabled. EndpointSlice membership and the complete Operator pass remain planned.

## 1. Objective and scope

Build a small Go read cache that connects routing, membership changes, peer failure handling, and reproducible functional validation. Reuse the existing LRU, Bloom filter, Protobuf protocol, and Kubebuilder scaffold.

Deliver functionality before performance experiments. Heavy load tools, long-running benchmarks, and throughput claims are optional follow-up work.

The first source is a finite immutable demo dataset: Tom, Jack, Sam, and Auto-0 through Auto-9999. Every node uses the same source values and key set. The Getter interface allows later source replacement without adding a database dependency now.

Boundaries:

- Cache memory can be lost and rebuilt through the source; no PVC or persistence.
- No write/delete API, replication protocol, consensus, or strong consistency.
- Hash ownership selects a preferred cache node, not the only copy of authoritative data.
- Source availability is required for successful fallback.
- Membership views converge eventually; a process-level atomic update does not synchronize all nodes.
- SingleFlight and source limits are local to a process, not global cluster protection.
- Scale changes rebuild affected entries on demand; no active cache migration.
- The current static fallback copies may remain until TTL or eviction.

Raft, replicas, distributed locks, autoscaling, cross-cluster operation, tracing, complex circuit breakers, and hot-key replicas are out of scope.

## 2. Baseline and active state

| Area | Original limitation | Active or planned treatment |
| --- | --- | --- |
| Local cache | Initial LRU insertion omitted expiry | Fixed; capacity and TTL tests retained |
| Bloom | Only three hard-coded keys were inserted | Complete demo key set; generic Getter can disable Bloom |
| Peer reads | Handler re-entered full routing; errors could be overwritten | Single-hop GetLocal and explicit error handling implemented |
| Coalescing | One custom SingleFlight without cancellation | Two x/sync groups and bounded shared work implemented |
| Failure handling | Peer had no total request timeout | Timeout and local fallback implemented |
| Membership | PEERS environment changes rolled existing Pods | EndpointSlice consumption planned |
| Ring publication | Mutable state protected by a mutex | Lock-based static routing active; atomic snapshots planned |
| Operator | Partial resource updates, missing permissions/status/watches | Dedicated Operator pass planned |
| Observability | A mixed counter and static scrape targets | Basic request/source metrics active; deployment/dashboard pending |
| Verification | Fixed-port legacy load tests mixed with development | Fast tests and isolated envtest active; load tests archived |

The controller integration suite checks its current static-peer behavior. It does not imply that all planned deployment behavior has been delivered.

## 3. Responsibilities and layout

The data plane handles reads; the Operator expresses desired Kubernetes resources. Kubernetes maintains EndpointSlices; the planned cache process watches those slices directly. The Operator will not relay membership through CR Status or ConfigMaps.

```text
geecache-engine/
  cmd/simplecache/
  internal/app/
  internal/cache/lru/
  internal/peer/hashring/
  internal/peer/peerpb/
  internal/demo/
  internal/telemetry/
  internal/discovery/             planned
simplecache-operator/
  cmd/
  api/v1/
  internal/controller/
  config/
```

Each top-level component is one Go module. The cache library is part of the engine module, without a nested go.mod or a local replace.

cache defines source/peer/observer interfaces without importing HTTP, Protobuf, or Kubernetes. peer implements communication and routing. app assembles dependencies and owns HTTP lifecycles. Generated artifacts remain generator-owned.

## 4. Read path

API GET /api?key=...:

1. Validate a nonempty key of at most 256 bytes and establish the caller deadline.
2. Check local LRU; return hits.
3. Reject definite Bloom negatives as KEY_NOT_FOUND.
4. Join routeFlight for that key; check the cache again inside the shared function.
5. Pick the preferred owner from one coherent routing table.
6. If owner is local or the ring is empty, load locally.
7. Otherwise issue at most one peer request.
8. Return peer success, or terminal KEY_NOT_FOUND; attempt local fallback for peer transport/protocol/configuration failures.
9. Cache successful local source results; propagate explicit source errors.

Peer handlers only call GetLocal. That path checks cache/Bloom, joins originFlight, checks cache again, acquires a source slot, calls the Getter, and populates local cache.

The two SingleFlight groups are independent. Never recursively wait on the same key within one group. Never call Forget merely because one caller timed out.

Peer success is not cached at the entry node, preserving simple ownership behavior. This does not eliminate repeated peer traffic for hot keys; source-load coalescing is a different property.

## 5. Deadlines and shared work

| Work | Default budget |
| --- | --- |
| External API wait | 1500ms |
| Shared routed work | 1200ms |
| One peer request | 300ms |
| Independent source work, including slot queue | 800ms |

Peer child context is bounded by both its own timeout and remaining route time. Waiting for fallback must use the existing route context.

DoChan lets each caller stop waiting on its own context. Shared work derives an independent finite deadline from the process lifecycle, so the first caller cannot cancel other waiters.

Routing can return timeout while source work is still completing. Source work may cache success only within its own finite deadline. There is no unbounded background work and no waiter-reference-count mechanism.

Getter implementations must respond to context cancellation. Process shutdown first clears readiness and drains HTTP requests, then cancels shared work. Existing peer clients and transports are reused, with header/idle timeouts on servers.

## 6. Source concurrency protection

Use one bounded channel shared by the process's groups, default SOURCE_MAX_CONCURRENCY=32.

Acquire inside originFlight after the second cache lookup and before calling Getter. Queueing responds to source-context cancellation and consumes the same 800ms budget. Recheck context after acquisition and release the slot on every exit.

Duplicate-key waiters do not each occupy slots. Queue timeout does not count as an actual Getter call. Maximum cluster source concurrency grows with running replicas; this is not a global limiter or a bound on all queued request objects.

Verify the cap with controlled different-key work, cancellation, error exits, and two groups sharing one limiter. Tune the value later rather than adding worker pools or distributed throttling now.

## 7. Peer protocol and errors

Retain HTTP + Protobuf for successful values. Error responses use an HTTP status and fixed X-SimpleCache-Error value:

| Peer outcome | Response | Entry behavior |
| --- | --- | --- |
| Key absent | 404 + KEY_NOT_FOUND | Return business miss |
| Group absent | 404 + GROUP_NOT_FOUND | Record peer error and load locally |
| Untyped/unknown 404 | Protocol failure | Load locally |
| Network, timeout, 5xx, decode failure | Peer failure | Load locally within route wait budget |

Invalid input is 400, source unavailability 503, timeout 504. Successful empty values are valid; failures must not be encoded as successful empty values.

MaxValueBytes=1MiB and MaxBodyBytes=MaxValueBytes+1024 are separate bounds. Validate the value and encoded size server-side. Read at most the body limit plus one byte client-side, reject excess, decode, then validate value size again.

Use consistent path escaping/unescaping. Tests cover spaces, plus signs, slashes, Unicode, a value exactly at the boundary, oversized encoded bodies, and decoded oversized values.

## 8. Planned membership and snapshots

Use a namespace-scoped EndpointSlice informer selected by kubernetes.io/service-name=<peer-service>. Add/update/delete events enqueue work; one worker rebuilds from every slice currently in the informer store.

First implementation targets IPv4 Pod endpoints and the peer TCP port. Accept ready=true or unknown ready=nil; reject ready=false and terminating=true. Keep publishNotReadyAddresses=false. Validate TargetRef, address, and port.

Stable node identity is namespace/podName, matching Downward API values. Transport addresses are Pod IP/port. Pod replacement can change an address without changing ring identity.

Sort and deduplicate members deterministically. Address changes also count as membership changes. Conflicting duplicate identities need deterministic selection and diagnostics.

Construct a complete snapshot containing ring, clients, sorted members, and a local publication version. Publish once using atomic.Pointer. After publication, no map, slice, ring, or client configuration may be mutated. A read loads once and obtains owner and client from that same snapshot.

Use fixed virtual-node encoding, uint32 positions, and deterministic collision handling. Equal member sets must produce equal maps regardless of informer iteration order.

Listeners and source initialize before readiness. Initial informer sync is required, but readiness must not wait for this Pod to already be published as an endpoint. A valid empty member set publishes an empty ring and uses local source reads.

A watch failure retains the last valid snapshot; informer relist/reconnect eventually repairs it. Lack of events does not mean discovery failed. Static mode never silently replaces a broken Kubernetes discovery configuration.

Old requests can finish with old clients or time out; new requests use the new table. Membership changes do not clear all local cache entries.

## 9. Bloom and cache policy

Build Bloom from the complete immutable source key set before serving. Never add a key only after its first cache miss, and never concurrently mutate the published filter.

Use a target theoretical false-positive rate around 1%; actual rates require measurement. Known keys must never be rejected. A positive Bloom result still requires cache/source lookup.

Generic sources that cannot provide a complete key set disable Bloom. Dynamic writes would need a separate filter-update protocol; the static policy does not claim arbitrary mutable-source support.

Default logical capacity is 64MiB and TTL 60s. Expiration is lazy. Byte accounting covers keys and values, not process RSS. Values are limited to 1MiB; Group capacity must be positive. ByteView and source results are isolated with copies.

Small deterministic capacity and expiry tests are mandatory. Large padded-value pressure experiments are optional follow-up work.

## 10. Planned Operator contract

Retain cache.x1kun.com/v1 and Kind SimpleCache. The active API has size/image and an unused foo scaffold field. The planned pass removes foo and adds bounded/defaulted configuration.

| Planned Spec | Contract |
| --- | --- |
| size | Default 3; range 1–10 |
| image | Required, nonempty |
| cacheBytes | Default 64MiB; range 1–128MiB |
| ttlSeconds | Default 60; range 1–3600 |
| resources | Optional, with development defaults |

Manage StatefulSet, headless peer Service, API ClusterIP Service, cache ServiceAccount, namespace EndpointSlice Role, and RoleBinding. Apply owner references, reject unrelated name collisions, preserve allocated/immutable fields, and update only managed configuration.

New StatefulSets use Parallel management. Retain compatible immutable fields on existing resources. Inject Pod identity, namespace, service name, and cache settings. Remove PEERS from Pod templates: scale changes then modify replicas only; image/cache settings may roll Pods.

Add startup/liveness /healthz and readiness /readyz probes with a termination grace period. Planned starting resources are 100m CPU/128Mi requests and 256Mi memory limit, to be calibrated.

Status contains observedGeneration, readyReplicas, and metav1.Conditions. Ready reflects the observed StatefulSet rollout, not simultaneous ring agreement. Patch only actual status changes.

Watch owned resources. No external resource cleanup exists, so no custom finalizer is needed. Separate Operator write permissions from cache namespace get/list/watch discovery permissions; label selectors do not restrict RBAC authorization scope.

Regenerate CRD, DeepCopy, and RBAC from types/markers. Keep Kubebuilder scaffold markers and PROJECT metadata.

## 11. Metrics and dashboards

Use per-process registry and bounded labels. Never label by key, URL, error text, or snapshot version.

Active metrics distinguish API result/duration, API/peer local lookups, peer requests/duration, source loads/duration, source-slot waits, inflight source calls, Bloom rejects, and fallback reasons. Source counts mean actual Getter executions, not waiters.

Member count/publication/discovery-error metrics will describe the dynamic snapshot path. Logical cache bytes and capacity-eviction counters still need their own instrumentation.

Local hit ratio uses first lookups, separated by entry kind; successful peer fetches are not local hits. Brief fault windows may require counter deltas rather than sparse rate charts.

Planned monitoring uses lightweight Prometheus/Grafana with dynamic Pod discovery, scrape permissions, provisioned datasource, and a basic dashboard. Do not add kube-prometheus-stack merely for this demo.

Panels: API throughput/errors; API percentiles; local hit ratio; peer results/latency; source/fallback; members/updates. These are observability targets, not unmeasured performance claims.

## 12. Verification and CI

Fast tests cover TTL insertion/update, eviction and reload, value isolation, known-key Bloom acceptance, request coalescing, independent cancellation, source-slot budgets/caps, typed errors, size bounds, URL encoding, and shutdown drainage.

Planned discovery tests cover multiple slices, readiness/termination, deletion, address changes, duplicate input, empty rings, and concurrent snapshot readers. Small offline hash samples check deterministic ownership and expansion properties, reporting actual distribution rather than enforcing exact theoretical fractions.

Controller envtest checks resource CRUD, ownership, idempotence, current static peer updates, and missing-resource handling. It does not run StatefulSet/garbage-collection controllers or prove deployed permissions and networking.

The root CI separates quality from Operator API-server tests. It adds formatting, vet, race/coverage, builds, Compose parsing, generated consistency, and manifest rendering. Container builds are separate and do not publish images. No default Kind or heavy load job is enabled.

Later isolated Kind smoke should cover:
- Correct values and missing keys across three nodes.
- A small concurrent cold hot-key batch with one actual owner source load.
- A deterministic slow peer that remains in the routing view, proving timeout fallback.
- Deleting the owner while preserving the entry process.
- CR scaling 3→5→2, eventual membership convergence, and no unrelated old-Pod restart.
- Valid empty membership and recovery from discovery interruptions.

Use fresh keys so local cache does not hide peer failure. Cache Pod deletion is not a physical Node outage. A 10s local convergence target is an experimental goal, not a Kubernetes fault-detection guarantee.

Heavy benchmarks remain optional. If added, record machine/container limits, source latency, capacity, TTL, warmup, client mode, error/value checks, and repeated results. Do not infer high-concurrency performance from a few functional requests.

## 13. Delivery and completion

Deliver coherent changes in four stages: structure, request reliability/lifecycle, membership, and Operator. CI and English documentation support the active stage. Version control is handled separately by the user.

Functionality is complete only when code, relevant tests, a repeatable lightweight cluster path, monitoring, and accurate limits are available. Performance evidence is separate. A functional green test run supports mechanism claims, not published QPS or production availability.

## References

- [HTTP client deadlines](https://pkg.go.dev/net/http#Client)
- [SingleFlight](https://pkg.go.dev/golang.org/x/sync/singleflight)
- [Atomic pointers](https://pkg.go.dev/sync/atomic)
- [EndpointSlices](https://kubernetes.io/docs/concepts/services-networking/endpoint-slices/)
- [StatefulSets](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)
- [Kubebuilder envtest](https://book.kubebuilder.io/reference/envtest)
