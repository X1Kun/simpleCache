#!/usr/bin/env bash
set -euo pipefail
source "$(dirname -- "${BASH_SOURCE[0]}")/common.sh"
# This is an explicit manual cleanup; no other cluster name is accepted.
kind delete cluster --name "$cluster_name"
