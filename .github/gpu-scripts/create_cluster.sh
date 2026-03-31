#!/usr/bin/env bash
set -euo pipefail

echo "🔧 Initializing environment..."

export http_proxy="http://proxy-dmz.intel.com:911"
export https_proxy="http://proxy-dmz.intel.com:912"

LOCAL_IP=$(hostname -I | awk '{print $1}')
echo "🔍 Detected local IP: $LOCAL_IP"
export no_proxy="127.0.0.1,localhost,10.165.116.220,${LOCAL_IP},10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,.svc,.svc.cluster.local,.cluster.local,intel.com,devel"
export NO_PROXY="${no_proxy}"

mkdir -p "$HOME/.kube"

echo "🚀 Initializing Kind cluster..."
kind create cluster --config hack/kind-config.yaml

kubectl get pods -A

echo "✅ Kind cluster setup completed successfully"
