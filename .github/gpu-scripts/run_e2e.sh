#!/usr/bin/env bash
# Copyright (C) 2025-2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

export PATH="/usr/local/go/bin:$PATH"
export KUBECONFIG="$HOME/.kube/config"
LOCAL_IP=$(hostname -I | awk '{print $1}')
KUBE_API_IP=$(kubectl config view --raw -o jsonpath='{.clusters[0].cluster.server}' | sed 's|https\?://||' | cut -d: -f1)

export http_proxy="http://proxy-dmz.intel.com:911"
export https_proxy="http://proxy-dmz.intel.com:912"
export no_proxy="127.0.0.1,localhost,10.165.116.220,${LOCAL_IP},${KUBE_API_IP},10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,.svc,.svc.cluster.local,.cluster.local,intel.com,devel"
export NO_PROXY="${no_proxy}"

if ! type ginkgo; then
  echo "📦 Installing Ginkgo CLI..."
  GINKGO_VER="$(go list -m -f '{{.Version}}' github.com/onsi/ginkgo/v2 || echo '')"
  if [ -n "$GINKGO_VER" ]; then
    go install "github.com/onsi/ginkgo/v2/ginkgo@${GINKGO_VER}"
  else
    go install github.com/onsi/ginkgo/v2/ginkgo@v2.28.1
  fi
  gopath_bin="$(go env GOPATH)/bin"
  export PATH="$PATH:$gopath_bin"
fi

unset http_proxy https_proxy no_proxy
set -euo pipefail

GPU_IMAGE_TAG="${GPU_IMAGE_TAG:-ger-is-registry.caas.intel.com/dgpu-orchestration/intel-gpu-resource-driver:devel}"
echo "📦 Using image: $GPU_IMAGE_TAG"

echo "🧪 Running E2E tests..."

GINKGO_FOCUS="GPU DRA Driver"
if [ "${ACTIONS_RUNNER_NAME:-}" = "cri" ]; then
  GINKGO_FOCUS="GPU DRA driver is running in CRI simics"
fi

GPU_IMAGE_TAG="$GPU_IMAGE_TAG" ginkgo -v --focus "$GINKGO_FOCUS" ./test/e2e

echo "✅ E2E tests passed successfully"
