#!/usr/bin/env bash
set -euo pipefail
source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
report_dir="${SIMPLECACHE_REPORT_DIR:?Run make kind-smoke to capture a report}"
forward_pid=""; probe_pid=""; original_spec=""
cleanup() {
  local code="$?" probe_code=0
  trap - EXIT
  touch "$report_dir/probe.stop"
  if [[ -n "$probe_pid" ]]; then wait "$probe_pid" || probe_code=$?; fi
  if [[ -n "$forward_pid" ]]; then kill "$forward_pid" 2>/dev/null || true; wait "$forward_pid" 2>/dev/null || true; fi
  k -n "$namespace" get pods,statefulsets,services,simplecaches -o yaml >"$report_dir/cluster-after.yaml" 2>&1 || true
  k -n "$namespace" get events --sort-by=.lastTimestamp >"$report_dir/events.log" 2>&1 || true
  k -n simplecache-operator-system logs deployment/simplecache-operator-controller-manager --tail=200 >"$report_dir/operator.log" 2>&1 || true
  if [[ -n "$original_spec" ]]; then
    k -n "$namespace" patch simplecache simplecache --type=json -p "[{\"op\":\"replace\",\"path\":\"/spec\",\"value\":$original_spec}]" || code=1
    # StatefulSet rollback can wait on an unready bad-revision Pod indefinitely.
    k -n "$namespace" delete pods -l app=simplecache --field-selector=status.phase=Pending --ignore-not-found || code=1
    wait_size 3 && wait_members 3 || code=1
  fi
  if [[ "$probe_code" != 0 ]]; then code=1; fi
  exit "$code"
}
assert_context
wait_size 3
wait_members 3
original_spec="$(k -n "$namespace" get simplecache simplecache -o jsonpath='{.spec}')"
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
k -n "$namespace" get pods,statefulsets,services,simplecaches -o yaml >"$report_dir/cluster-before.yaml"
# A random loopback port avoids conflicts with other projects' port-forwards.
k -n "$namespace" port-forward --address 127.0.0.1 pod/simplecache-1 :9999 >"$report_dir/port-forward.log" 2>&1 &
forward_pid=$!
port=""
for ((attempt=0;attempt<50;attempt++)); do
  port="$(sed -n 's/^Forwarding from 127\.0\.0\.1:\([0-9]*\) -> 9999$/\1/p' "$report_dir/port-forward.log" | head -1)"
  if [[ -n "$port" ]]; then break; fi
  if ! kill -0 "$forward_pid" 2>/dev/null; then cat "$report_dir/port-forward.log"; exit 1; fi
  sleep 0.2
done
test -n "$port"
"$report_dir/probe" -url "http://127.0.0.1:$port" -dir "$report_dir" >"$report_dir/probe.log" 2>&1 &
probe_pid=$!
permission="$(k -n "$namespace" auth can-i list endpointslices.discovery.k8s.io --as="system:serviceaccount:$namespace:simplecache-cache")"
test "$permission" = yes
value="$(k -n "$namespace" exec simplecache-1 -- wget -qO- 'http://127.0.0.1:9999/api?key=Auto-666')"
test "$value" = Value-for-Auto-666
uids="$(k -n "$namespace" get pods simplecache-0 simplecache-1 -o jsonpath='{range .items[*]}{.metadata.name}={.metadata.uid}{"\n"}{end}')"
for cycle in 1 2; do
echo "Scaling cycle $cycle with concurrent reads"
k -n "$namespace" patch simplecache simplecache --type=merge -p '{"spec":{"size":5}}'
wait_size 5
wait_members 5
test "$uids" = "$(k -n "$namespace" get pods simplecache-0 simplecache-1 -o jsonpath='{range .items[*]}{.metadata.name}={.metadata.uid}{"\n"}{end}')"
k -n "$namespace" patch simplecache simplecache --type=merge -p '{"spec":{"size":2}}'
wait_size 2
wait_members 2
test "$uids" = "$(k -n "$namespace" get pods simplecache-0 simplecache-1 -o jsonpath='{range .items[*]}{.metadata.name}={.metadata.uid}{"\n"}{end}')"
done
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
touch "$report_dir/probe.stop"
wait "$probe_pid"
probe_pid=""
kill "$forward_pid" 2>/dev/null || true
wait "$forward_pid" 2>/dev/null || true
forward_pid=""
cat "$report_dir/probe.log"
for pod in simplecache-0 simplecache-1 simplecache-2; do
  k -n "$namespace" exec "$pod" -- wget -qO- http://127.0.0.1:9999/metrics >"$report_dir/$pod-metrics.txt"
done
# An impossible CPU request tests scheduling/Status without stressing the host.
# All changes are confined to this dedicated test CR and restored on exit.
k -n "$namespace" patch simplecache simplecache --type=merge -p '{"spec":{"resources":{"requests":{"cpu":"100000","memory":"128Mi"},"limits":{"cpu":"100000","memory":"256Mi"}}}}'
unschedulable=false
for ((attempt=0;attempt<60;attempt++)); do
  reasons="$(k -n "$namespace" get pods -l app=simplecache -o jsonpath='{range .items[*]}{range .status.conditions[?(@.type=="PodScheduled")]}{.reason}{"\n"}{end}{end}')"
  if [[ "$reasons" == *Unschedulable* ]]; then unschedulable=true; break; fi
  sleep 1
done
test "$unschedulable" = true
k -n "$namespace" get pods,simplecaches -o yaml >"$report_dir/unschedulable.yaml"
k -n "$namespace" get events --sort-by=.lastTimestamp >"$report_dir/unschedulable-events.log"
not_ready=false
for ((attempt=0;attempt<30;attempt++)); do
  condition="$(k -n "$namespace" get simplecache simplecache -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')"
  if [[ "$condition" == False ]]; then not_ready=true; break; fi
  sleep 1
done
test "$not_ready" = true
k -n "$namespace" patch simplecache simplecache --type=json -p "[{\"op\":\"replace\",\"path\":\"/spec\",\"value\":$original_spec}]"
k -n "$namespace" delete pods -l app=simplecache --field-selector=status.phase=Pending --ignore-not-found
wait_size 3
wait_members 3
echo 'EVIDENCE {"scaling_cycles":2,"sizes":[3,5,2,5,2,3],"retained_pod_uids_stable":true,"child_repair":true,"pod_recovery":true,"unschedulable_request_and_recovery":true,"rollback_pending_pods_explicitly_removed":true}'
echo "Concurrent reads, scaling, repair, scheduling failure and recovery passed"
