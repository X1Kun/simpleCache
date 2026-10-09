#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
cluster_name=simplecache-stage4
context_name="kind-$cluster_name"
namespace=simplecache-test
export KUBECONFIG="$project_root/bin/kind/kubeconfig"
export GOFLAGS="${GOFLAGS:-} -buildvcs=false"
k() { kubectl --context "$context_name" "$@"; }
assert_context() {
  test "$(kubectl config current-context)" = "$context_name"
  test "$(k get namespace "$namespace" -o go-template='{{index .metadata.labels "app.kubernetes.io/part-of"}}')" = simplecache
}
wait_size() {
  local desired="$1" state generation observed ready condition
  for ((attempt=0; attempt<90; attempt++)); do
    state="$(k -n "$namespace" get simplecache simplecache -o jsonpath='{.metadata.generation} {.status.observedGeneration} {.status.readyReplicas} {.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null || true)"
    read -r generation observed ready condition <<< "$state" || true
    if [[ "$generation" == "$observed" && "$ready" == "$desired" && "$condition" == True ]]; then
      return 0
    fi
    sleep 2
  done
  k -n "$namespace" get pods,statefulsets,events
  return 1
}
wait_members() {
  local desired="$1" pods pod metrics valid
  for ((attempt=0; attempt<30; attempt++)); do
    pods="$(k -n "$namespace" get pods -l app=simplecache -o jsonpath='{.items[*].metadata.name}')"
    valid=0
    for pod in $pods; do
      metrics="$(k -n "$namespace" exec "$pod" -- wget -qO- http://127.0.0.1:9999/metrics 2>/dev/null || true)"
      while IFS= read -r line; do
        if [[ "$line" == "simplecache_members $desired" ]]; then
          valid=$((valid+1))
          break
        fi
      done <<< "$metrics"
    done
    if [[ "$valid" == "$desired" ]]; then return 0; fi
    sleep 1
  done
  return 1
}
