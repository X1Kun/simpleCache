# Continuous integration

The workflows follow the separation used by the neighboring Orion Live project: fast quality checks and isolated integration tests run independently, while container builds are kept off the pull-request critical path.

GitHub reads workflows from the repository root .github/workflows directory. The old Operator-subdirectory workflow copies have been replaced by these active workflows.

## CI workflow

.github/workflows/ci.yml runs on pushes to main, pull requests targeting main, and manual dispatch. It uses read-only repository permissions, cancels superseded runs for the same ref, and caches both modules' Go dependencies.

| Job | Checks |
| --- | --- |
| quality | Go formatting, vet, engine race tests/coverage, both binaries, Compose parsing, generated-file consistency, Kustomize rendering |
| operator-tests | A temporary API server/etcd, controller reconciliation tests, race detection, coverage |

The controller tests supply a valid CR and check the headless Service, StatefulSet, owner references, repeated reconciliation, static PEERS/image/replica updates, and missing-resource handling. Envtest has no garbage collector, so tests explicitly delete their child resources.

Coverage files are uploaded as separate artifacts. Engine and controller test commands use -count=1 and explicit timeouts; test results are not silently taken from a previous run.

## Container workflow

.github/workflows/containers.yml builds engine and Operator images on main pushes and manual dispatch. It does not publish images or require registry credentials.

Docker builds use the official Go module proxy by default for the engine. A regional proxy can be selected with a GOPROXY build argument. Both binaries disable automatic VCS stamping.

## Local equivalents

```bash
make fmt-check vet test-ci build
make compose-config generated-check k8s-render
make test-operator-ci
```

generated-check compares CRD, RBAC, and DeepCopy files before and after generation using temporary copies and cmp. It does not run Git commands.

## Limits

These jobs do not start Kind or run legacy load tests. Controller envtest does not simulate the StatefulSet controller, kubelet, actual RBAC deployment, cache Pod networking, or the future EndpointSlice discovery path.

The current controller still broadcasts a static peer list through the Pod template. Tests describe that active behavior; update them when the dedicated Operator redesign removes PEERS. A green CI run is not a claim of production readiness or measured throughput.
