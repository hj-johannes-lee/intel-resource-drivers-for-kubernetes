#!/bin/bash
set -euo pipefail

DEVICE_FAKER_IMAGE="${DEVICE_FAKER_IMAGE:-ger-is-registry.caas.intel.com/dgpu-orchestration/intel-device-faker:v0.5.0}"

# Returns device-faker type for a chart, or empty if not supported.
get_faker_type() {
  local chart_name=$1
  case "$chart_name" in
    *gpu*)   echo "gpu" ;;
    *gaudi*) echo "gaudi" ;;
    *)       echo "" ;;
  esac
}

# Installing chart
install_with_device_faker() {
  local chart=$1
  local release_name=$2
  local namespace=$3
  local faker_type=$4

  echo "Using device-faker (type=$faker_type) instead of real driver image"
  helm template "$release_name" "$chart" --namespace "$namespace" | \
    sed \
      -e "s|image: .*/intel-.*-resource-driver:.*|image: ${DEVICE_FAKER_IMAGE}|" \
      -e 's|command: \["/kubelet-'"${faker_type}"'-plugin"\]|command: ["/device-faker", "'"${faker_type}"'", "-t", "/opt/templates/'"${faker_type}"'-template.json", "-r", "-c", "-d", "/var/run/cdi/device-faker"]|' | \
    kubectl apply -n "$namespace" -f -
}

check_no_crashloops() {
  local namespace=$1
  local chart_name=$2

  echo "Checking for crashloops in $chart_name..."
  echo "Waiting for pod to start..."
  sleep 30

  restarts=$(kubectl get pods -n "$namespace" \
    -o jsonpath='{range .items[*]}{.status.containerStatuses[*].restartCount}{"\n"}{end}' 2>/dev/null | \
    awk '{sum+=$1} END {print sum+0}')

  if [ "$restarts" -gt 0 ]; then
    echo "ERROR: Detected $restarts restart(s)"
    kubectl get pods -n "$namespace" -o wide
    echo "--- Pod logs ---"
    for pod in $(kubectl get pods -n "$namespace" -o jsonpath='{.items[*].metadata.name}'); do
      echo "Logs for $pod:"
      kubectl logs -n "$namespace" "$pod" --all-containers --tail=100 2>/dev/null || true
    done
    echo "--- Pod describe ---"
    kubectl describe pods -n "$namespace" 2>/dev/null || true
    return 1
  fi

  echo "No crashloops detected"
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
    faker_type=$(get_faker_type "$chart_name")

    echo "================================"
    echo "Testing: $chart_name"
    echo "================================"

    kubectl create namespace "$namespace"

    echo "Installing $chart_name..."
    if [ -n "$faker_type" ]; then
      install_with_device_faker "$chart" "$release_name" "$namespace" "$faker_type"
    else
      echo "SKIP: device-faker does not support $chart_name, installing as-is"
      helm install "$release_name" "$chart" --namespace "$namespace" --timeout 180s
    fi

    check_no_crashloops "$namespace" "$chart_name"

    echo "Uninstalling $chart_name..."
    if [ -n "$faker_type" ]; then
      helm template "$release_name" "$chart" --namespace "$namespace" | kubectl delete -n "$namespace" -f - --ignore-not-found
    else
      helm uninstall "$release_name" --namespace "$namespace"
    fi

    echo "$chart_name: OK"
  done
}

main
