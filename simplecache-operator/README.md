# SimpleCache Operator

A Kubebuilder controller for the namespaced cache.x1kun.com/v1 SimpleCache API.

## Active behavior

The controller creates a governing headless Service and StatefulSet. It derives stable Pod DNS peer URLs from spec.size and injects the resulting PEERS environment variable. Changes to size, image, or the peer list update the StatefulSet.

The CR currently exposes size and image, plus an unused scaffold foo field. Status conditions exist in the schema but are not yet populated by reconciliation.

## Layout

- api/v1: API types and generated DeepCopy code.
- cmd: controller-runtime Manager entry point.
- internal/controller: reconciliation and envtest.
- config/crd and config/rbac: generated manifests.
- config/samples: a valid example resource.
- test/e2e: upstream Manager/metrics scaffold tests.

Repository workflows are in ../.github/workflows. This directory no longer has standalone workflow copies.

## Development

Use Go 1.25.3 or a compatible newer toolchain.

```bash
make manifests generate
make test
make test-ci
make lint-fix
```

test and test-ci download API-server/etcd assets and use envtest. They do not deploy a workload to your existing cluster. The CI variant adds race detection, an uncached run, a timeout, and coverage output.

The root Makefile also provides make generated-check and make k8s-render for consistency and rendering without deployment.

## Deployment status

A sample CR is provided at config/samples/cache_v1_simplecache.yaml. It uses a local simplecache:dev image; that image must exist in the selected cluster before a deployment can start.

The active baseline is not a complete deployment contract: Service/StatefulSet permissions, child-resource watches, cache probes, status reconciliation, and dynamic membership need the planned Operator work. Envtest uses its own administrative client and does not validate those deployment permissions.

The inherited test-e2e target creates an isolated Kind cluster and tests the Manager scaffold. It is not enabled in the new CI and does not establish cache-cluster correctness.

Generated CRD/RBAC/DeepCopy files must be regenerated from source. Preserve Kubebuilder scaffold markers and PROJECT metadata.
