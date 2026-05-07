#!/bin/bash
set -euo pipefail

get_pod_health() {
  local namespace=$1
  kubectl get pods -n "$namespace" -o wide

  echo "--- Pod describe ---"
  kubectl describe pods -n "$namespace" 2>/dev/null

  echo "--- Pod logs ---"
  for pod in $(kubectl get pods -n "$namespace" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null); do
    for container in $(kubectl get pod "$pod" -n "$namespace" -o jsonpath='{.spec.initContainers[*].name} {.spec.containers[*].name}' 2>/dev/null); do
      echo "--- Logs for pod=$pod - container=$container ---"
      kubectl logs -n "$namespace" "$pod" -c "$container" 2>/dev/null
      echo "--- Previous logs for pod=$pod - container=$container ---"
      kubectl logs -n "$namespace" "$pod" -c "$container" --previous 2>/dev/null
    done
  done
}

check_no_crashloops() {
  local namespace=$1
  local chart_name=$2

  echo "Waiting for pods in $chart_name to start..."
  for _ in $(seq 1 30); do
    if [ "$(kubectl get pods -n "$namespace" --no-headers 2>/dev/null | wc -l)" -gt 0 ]; then
      break
    fi
  sleep 2
  done

  echo "Waiting for pods to become Ready..."
  if ! kubectl wait --for=condition=Ready pods --all -n "$namespace" --timeout=180s; then
    echo "ERROR: pods did not reach Ready state"
    get_pod_health "$namespace"
    return 1
  fi

  restarts=$(kubectl get pods -n "$namespace" \
    -o jsonpath='{range .items[*]}{.status.containerStatuses[*].restartCount}{"\n"}{end}' 2>/dev/null | \
    awk '{sum+=$1} END {print sum+0}')

  if [ "$restarts" -gt 0 ]; then
    echo "ERROR: Detected $restarts restart(s)"
    get_pod_health "$namespace"
    return 1
  fi

  echo "Pods are Running and Ready, no restarts"
  kubectl get pods -n "$namespace" -o wide
}

main() {
  if [ ! -d "charts" ]; then
    echo "ERROR: 'charts' directory not found"
    exit 1
  fi

  for chart in charts/*; do
    [ -d "$chart" ] || continue

    chart_name=$(basename "$chart")
    release_name="${chart_name}-test"
    namespace="$chart_name"

    echo "================================"
    echo "Testing: $chart_name"
    echo "================================"

    kubectl create namespace "$namespace"

    extra_args=()
    case "$chart_name" in
      intel-gpu-resource-driver|intel-gaudi-resource-driver)
        extra_args+=(--set kubeletPlugin.healthMonitoring.enabled=false)
        ;;
    esac

    helm install "$release_name" "$chart" \
      --namespace "$namespace" \
      "${extra_args[@]}" \
      --timeout 180s

    check_no_crashloops "$namespace" "$chart_name"

    echo "Uninstalling $chart_name..."
    helm uninstall "$release_name" --namespace "$namespace"

    echo "$chart_name: OK"
  done
}

main
