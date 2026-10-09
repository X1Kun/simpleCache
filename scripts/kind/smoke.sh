#!/usr/bin/env bash
set -euo pipefail
source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
assert_context
wait_size 3
wait_members 3
permission="$(k -n "$namespace" auth can-i list endpointslices.discovery.k8s.io --as="system:serviceaccount:$namespace:simplecache-cache")"
test "$permission" = yes
value="$(k -n "$namespace" exec simplecache-1 -- wget -qO- 'http://127.0.0.1:9999/api?key=Auto-666')"
test "$value" = Value-for-Auto-666
uids="$(k -n "$namespace" get pods simplecache-0 simplecache-1 -o jsonpath='{range .items[*]}{.metadata.name}={.metadata.uid}{"\n"}{end}')"
k -n "$namespace" patch simplecache simplecache --type=merge -p '{"spec":{"size":5}}'
wait_size 5
wait_members 5
test "$uids" = "$(k -n "$namespace" get pods simplecache-0 simplecache-1 -o jsonpath='{range .items[*]}{.metadata.name}={.metadata.uid}{"\n"}{end}')"
k -n "$namespace" patch simplecache simplecache --type=merge -p '{"spec":{"size":2}}'
wait_size 2
wait_members 2
test "$uids" = "$(k -n "$namespace" get pods simplecache-0 simplecache-1 -o jsonpath='{range .items[*]}{.metadata.name}={.metadata.uid}{"\n"}{end}')"
old_service="$(k -n "$namespace" get service simplecache-api -o jsonpath='{.metadata.uid}')"
k -n "$namespace" delete service simplecache-api
for ((attempt=0; attempt<30; attempt++)); do
  current="$(k -n "$namespace" get service simplecache-api -o jsonpath='{.metadata.uid}' 2>/dev/null || true)"
  if [[ -n "$current" && "$current" != "$old_service" ]]; then break; fi
  sleep 1
done
test -n "$current" && test "$current" != "$old_service"
old_pod="$(k -n "$namespace" get pod simplecache-0 -o jsonpath='{.metadata.uid}')"
k -n "$namespace" delete pod simplecache-0
for ((attempt=0; attempt<60; attempt++)); do
  current="$(k -n "$namespace" get pod simplecache-0 -o jsonpath='{.metadata.uid}' 2>/dev/null || true)"
  if [[ -n "$current" && "$current" != "$old_pod" ]]; then break; fi
  sleep 1
done
test -n "$current" && test "$current" != "$old_pod"
wait_size 2
wait_members 2
value="$(k -n "$namespace" exec simplecache-1 -- wget -qO- 'http://127.0.0.1:9999/api?key=Auto-667')"
test "$value" = Value-for-Auto-667
k -n "$namespace" patch simplecache simplecache --type=merge -p '{"spec":{"size":3}}'
wait_size 3
wait_members 3
echo "Read, discovery permissions, scaling, child repair and Pod recovery passed"
