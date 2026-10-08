#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
cd -- "$project_root"
export GOFLAGS="${GOFLAGS:-} -buildvcs=false"
baseline_dir="$(mktemp -d)"
files=(
  simplecache-operator/api/v1/zz_generated.deepcopy.go
  simplecache-operator/config/crd/bases/cache.x1kun.com_simplecaches.yaml
  simplecache-operator/config/rbac/role.yaml
)
# Each cleanup target is a numbered copy created by this script.
cleanup() {
  for i in "${!files[@]}"; do
    rm -f -- "$baseline_dir/$i"
  done
  rmdir "$baseline_dir"
}
trap cleanup EXIT
for i in "${!files[@]}"; do
  cp -- "${files[$i]}" "$baseline_dir/$i"
done
make -C simplecache-operator manifests generate
result=0
for i in "${!files[@]}"; do
  if ! cmp -s -- "$baseline_dir/$i" "${files[$i]}"; then
    echo "Generated file is stale: ${files[$i]}" >&2
    diff -u -- "$baseline_dir/$i" "${files[$i]}" || true
    result=1
  fi
done
exit "$result"
