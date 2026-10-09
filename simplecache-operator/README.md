# SimpleCache Operator

A Kubebuilder controller for cache.x1kun.com/v1 SimpleCache. It reconciles a StatefulSet, headless peer and ClusterIP API Services, cache ServiceAccount, and namespace EndpointSlice Role/Binding.

The cache environment uses POD_NAME/POD_NAMESPACE, DISCOVERY_MODE=kubernetes, PEER_SERVICE, capacity and TTL. It has no static PEERS list. Scale changes replicas only; cache configuration or image updates may roll Pods.

## API

Spec fields: size (default 3, range 1–10), required image, cacheBytes (default 64MiB, range 1–128MiB), ttlSeconds (default 60, range 1–3600), and optional resources. The unused foo scaffold field is removed.

Status reports observedGeneration, readyReplicas and Ready conditions. InvalidSpec/ResourceConflict/ReconcileFailed describe failures; Ready reflects the observed StatefulSet rollout, not identical instantaneous member views.

The controller rejects unrelated name collisions, preserves allocated/immutable fields and API defaults, watches managed children, and avoids redundant writes. New sets use Parallel; compatible existing immutable policy is retained. Resources and startup/liveness/readiness probes are configured.

## Checks

```bash
make manifests generate
make test
make test-ci
make lint-fix
```

Envtest starts its own API server/etcd. Tests cover resources/ownership, API defaults and idempotence, Status, replica-only 3→5→2 scaling, image changes, drift/conflicts, resource budgets, missing CRs, and actual Manager child-watch repair. It does not run workload controllers or prove deployed RBAC/networking.

Root generated-check and k8s-render validate generated consistency and deployment overlays. Repository CI lives in ../.github/workflows.

## Isolated deployment

From the repository root use make kind-up, then make kind-smoke. The dedicated cluster is simplecache-stage4 and kubeconfig stays under bin/kind. Explicit kind-delete removes only that cluster. No image is published.

Live verification remains pending because Docker/Kind tool execution was interrupted; envtest must not be presented as a successful real-cluster run. The sample CR assumes a local cache image already available in the cluster.

Generated CRD/RBAC/DeepCopy must come from types and markers. Preserve PROJECT and Kubebuilder scaffold markers. No external resources require a custom finalizer.
