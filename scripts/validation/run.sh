#!/usr/bin/env bash
set -euo pipefail
project_root="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/../.." && pwd)"
mode="${1:-local}"
case "$mode" in local|unit|race|operator|kind) ;; *) echo "Usage: $0 [local|unit|race|operator|kind]" >&2; exit 2;; esac
cd -- "$project_root"
export GOFLAGS="${GOFLAGS:-} -buildvcs=false"
mkdir -p bin/reports
report_dir="$(mktemp -d "$project_root/bin/reports/$(date -u +%Y%m%dT%H%M%SZ)-$mode-XXXXXX")"
echo "Report directory: $report_dir"
# Build the reporter before any disruptive scenario starts.
(cd geecache-engine && go build -o "$report_dir/reporter" ./cmd/validation-report)
active_stage=""; started=0; recorded=false
finish() {
  local code="$?"
  trap - EXIT
  if [[ -n "$active_stage" && "$recorded" == false ]]; then
    printf '%s\t%s\t%s\n' "$active_stage" "${code:-1}" "$((SECONDS-started))" >>"$report_dir/stages.tsv"
  fi
  if ! "$report_dir/reporter" -dir "$report_dir"; then code=1; fi
  echo "Report: $report_dir/report.md"
  exit "$code"
}
trap finish EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
stage() {
  active_stage="$1"; shift; started=$SECONDS; recorded=false
  local code=0
  "$@" 2>&1 | tee "$report_dir/$active_stage.log" || code=$?
  printf '%s\t%s\t%s\n' "$active_stage" "$code" "$((SECONDS-started))" >>"$report_dir/stages.tsv"
  recorded=true
  return "$code"
}
local_race() {
  cd "$project_root/geecache-engine"
  go test -json -race -count=1 -timeout=3m -coverprofile="$report_dir/coverage.out" ./... | tee "$report_dir/engine-race.jsonl"
}
local_unit() {
  cd "$project_root/geecache-engine"
  go test -json -count=1 -timeout=3m ./... | tee "$report_dir/engine-unit.jsonl"
}
operator_tests() {
  # Distinguish fresh runs requiring structured evidence from legacy logs.
  touch "$report_dir/ginkgo-required"
  make -C "$project_root/simplecache-operator" test-ci GINKGO_JSON_REPORT="$report_dir/ginkgo.json" || return "$?"
  cp "$project_root/simplecache-operator/cover.out" "$report_dir/operator-coverage.out"
}
diagnostics() {
  cd "$project_root/geecache-engine"
  go test -json -count=1 -timeout=2m -o "$report_dir/scenarios.test" \
    -cpuprofile="$report_dir/cpu.pprof" -memprofile="$report_dir/heap.pprof" \
    -mutexprofile="$report_dir/mutex.pprof" -blockprofile="$report_dir/block.pprof" \
    ./internal/validation | tee "$report_dir/diagnostics.jsonl"
}
if [[ "$mode" == unit ]]; then
  stage engine-unit local_unit
elif [[ "$mode" == race ]]; then
  stage engine-race local_race
elif [[ "$mode" == operator ]]; then
  stage operator-tests operator_tests
elif [[ "$mode" == local ]]; then
  stage engine-race local_race
  stage diagnostics diagnostics
  stage cpu-top go tool pprof -top -nodecount=15 "$report_dir/scenarios.test" "$report_dir/cpu.pprof"
  stage heap-top go tool pprof -top -nodecount=15 "$report_dir/scenarios.test" "$report_dir/heap.pprof"
else
  # Raw cluster diagnostics are collected even when the smoke fails.
  export SIMPLECACHE_REPORT_DIR="$report_dir"
  stage probe-build bash -c 'cd "$1/geecache-engine" && go build -o "$2/probe" ./cmd/validation-probe' _ "$project_root" "$report_dir"
  stage kind-smoke bash "$project_root/scripts/kind/smoke.sh"
fi
