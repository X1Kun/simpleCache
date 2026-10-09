#!/usr/bin/env bash
set -euo pipefail
source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
cd -- "$project_root"
mkdir -p bin/kind
found=false
while IFS= read -r name; do
  if [[ "$name" == "$cluster_name" ]]; then found=true; fi
done < <(kind get clusters)
if [[ "$found" == false ]]; then
  kind create cluster --name "$cluster_name" --config deploy/kind/config.yaml --kubeconfig "$KUBECONFIG" --wait 180s
else
  kind export kubeconfig --name "$cluster_name" --kubeconfig "$KUBECONFIG"
fi
test "$(kubectl config current-context)" = "$context_name"
# Docker builds do not inherit the host's Go module proxy automatically.
build_goproxy="${GOPROXY:-https://proxy.golang.org,direct}"
docker build --build-arg "GOPROXY=$build_goproxy" --tag simplecache-engine:stage4 geecache-engine
docker build --build-arg "GOPROXY=$build_goproxy" --tag simplecache-operator:stage4 simplecache-operator
kind load docker-image simplecache-engine:stage4 simplecache-operator:stage4 --name "$cluster_name"
make -C simplecache-operator kustomize
simplecache-operator/bin/kustomize build deploy/kind/operator | k apply -f -
# Dev tags are reused: restart only this project's test manager to load the new image.
k -n simplecache-operator-system rollout restart deployment/simplecache-operator-controller-manager
k -n simplecache-operator-system rollout status deployment/simplecache-operator-controller-manager --timeout=180s
k wait --for=condition=Established crd/simplecaches.cache.x1kun.com --timeout=60s
k apply -f deploy/kind/cache.yaml
if k -n "$namespace" get statefulset simplecache >/dev/null 2>&1; then
  k -n "$namespace" rollout restart statefulset/simplecache
fi
wait_size 3
wait_members 3
echo "SimpleCache is ready in $context_name"
