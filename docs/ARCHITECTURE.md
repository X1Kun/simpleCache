# Architecture and reading guide

Cache processes handle data requests. The Operator manages Kubernetes resources. The active engine uses static membership; the Operator still follows the original static-peer reconciliation model.

## Reading order

1. cmd/simplecache/main.go: flags, signals, and process errors.
2. internal/app/config.go: environment validation and static peer normalization.
3. internal/app/server.go: dependency assembly, API handlers, listeners, and shutdown.
4. internal/cache/group.go: Get/GetLocal, two SingleFlight layers, and deadlines.
5. internal/cache/limiter.go: source concurrency slots.
6. internal/peer/client.go, server.go, and status.go: HTTP/Protobuf, errors, URL encoding, and size bounds.
7. internal/peer/router.go and hashring: lock-protected static routing.
8. internal/demo/source.go and internal/telemetry/metrics.go: cancellable source and isolated metrics.

All paths in this list are relative to geecache-engine.

## Dependencies

cache owns the Getter, PeerGetter, PeerPicker, and Observer interfaces. It does not import HTTP, Protobuf, or Kubernetes. peer implements transport and routing using those interfaces. app assembles the packages. Groups and HTTP handlers are registered explicitly rather than through global registries.

## Read flow

API: validate and check local cache/Bloom → routeFlight → peer or local source.

A peer response succeeds directly. Only KEY_NOT_FOUND is a terminal business miss. GROUP_NOT_FOUND, untyped 404, transport errors, invalid Protobuf, and oversized responses are peer failures eligible for local fallback.

Peer handlers call only GetLocal: local cache/Bloom → originFlight → second cache lookup → source slot → Getter → cache result. They never select another peer, preventing forwarding loops when views differ.

routeFlight and originFlight are separate instances. The same flight must never recursively wait on the same key. Timed-out callers do not use Forget to start duplicate source work.

## Cancellation and bounds

Each caller waits with its own context. Shared work derives a finite deadline from the process lifecycle, not the first caller's context.

Routing stops waiting when its deadline expires. Independent source work can continue only until its own deadline. Queueing consumes that source budget, and duplicate-key waiters do not each acquire a slot. The application supplies one limiter shared across its groups.

The transport reuses connections and sets timeouts. Value bytes are limited to 1MiB; encoded Protobuf bodies have a separate 1MiB+1024 limit. Path escaping preserves spaces, plus signs, slashes, and Unicode keys.

## Lifecycle and metrics

All listeners bind before readiness becomes true. Shutdown clears readiness, closes listeners, drains active requests, and then cancels shared source work.

Metrics count actual peer requests and Getter executions. Coalesced waiters do not create extra source-load counts. Cache lookup metrics distinguish API and peer entry points.

## Control-plane boundary

The current Operator creates a StatefulSet and governing headless Service, and derives PEERS from desired replicas. Its deployment permissions, status handling, probes, and child-resource watches still need the planned Operator pass.

Envtest verifies API resource creation, ownership, idempotence, and the current static-peer update behavior. It does not prove deployed RBAC, Pod readiness, network availability, or cluster convergence. See [CI](CI.md) for the checks that run automatically.
