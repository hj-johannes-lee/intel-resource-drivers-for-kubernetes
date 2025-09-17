#!/usr/bin/env bash
set -euo pipefail

echo "🔧 Initializing environment..."

export K8S_VERSION="${K8S_VERSION:-1.33.0}"
export http_proxy="http://proxy-dmz.intel.com:911"
export https_proxy="http://proxy-dmz.intel.com:912"

LOCAL_IP=$(hostname -I | awk '{print $1}')
echo "🔍 Detected local IP: $LOCAL_IP"
export no_proxy="127.0.0.1,localhost,10.165.116.220,${LOCAL_IP},10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,.svc,.svc.cluster.local,.cluster.local,intel.com,devel"
export NO_PROXY="${no_proxy}"


wait_for_apt() {
    echo "⏳ Waiting for apt to become available..."
    while sudo lsof /var/lib/dpkg/lock-frontend >/dev/null 2>&1 || \
          sudo lsof /var/lib/apt/lists/lock >/dev/null 2>&1 || \
          sudo lsof /var/cache/apt/archives/lock >/dev/null 2>&1 || \
          pgrep -x unattended-upgr >/dev/null; do
        echo "  🔒 Another apt process is running (possibly unattended-upgrades). Waiting 10s..."
        sleep 10
    done
    echo "✅ apt is now available."
}

echo "🔧 Installing containerd..."
sudo apt-get update
wait_for_apt
sudo apt-get install -y containerd

echo "🔧 Configuring containerd..."
sudo mkdir -p /etc/containerd

sudo -E tee /etc/containerd/config.toml >/dev/null <<'EOF'
version = 2
[plugins]
  [plugins."io.containerd.grpc.v1.cri"]
    enable_cdi = true
    cdi_spec_dirs = ["/etc/cdi", "/var/run/cdi"]
    [plugins."io.containerd.grpc.v1.cri".containerd]
      default_runtime_name = "runc"
      [plugins."io.containerd.grpc.v1.cri".containerd.runtimes]
        [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc]
          container_annotations = ["gpu.*"]
          runtime_type = "io.containerd.runc.v2"
          [plugins."io.containerd.grpc.v1.cri".containerd.runtimes.runc.options]
            SystemdCgroup = true
    [plugins."io.containerd.grpc.v1.cri".registry]
      [plugins."io.containerd.grpc.v1.cri".registry.mirrors]
        [plugins."io.containerd.grpc.v1.cri".registry.mirrors."docker.io"]
          endpoint = ["https://cache-registry.caas.intel.com/v2/cache/","https://registry.fi.intel.com/v2/docker"]
  [plugins."io.containerd.internal.v1.opt"]
    path = "/var/lib/containerd/opt"
EOF

echo "🌐 Setting proxy for containerd..."
sudo mkdir -p /etc/systemd/system/containerd.service.d
cat <<EOF | sudo tee /etc/systemd/system/containerd.service.d/proxy.conf
[Service]
Environment="HTTP_PROXY=${http_proxy}"
Environment="HTTPS_PROXY=${https_proxy}"
Environment="NO_PROXY=${no_proxy}"
EOF

sudo systemctl daemon-reexec
sudo systemctl daemon-reload
sudo systemctl restart containerd
sudo systemctl enable containerd

echo "⚙️ Installing Kubernetes version $K8S_VERSION..."

sudo rm -f /etc/apt/keyrings/kubernetes-apt-keyring.gpg
curl -fsSL "https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION%.*}/deb/Release.key" | \
    sudo gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg
echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/v${K8S_VERSION%.*}/deb/ /" | \
    sudo tee /etc/apt/sources.list.d/kubernetes.list

wait_for_apt
sudo apt-get update
wait_for_apt
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --allow-change-held-packages kubelet kubeadm kubectl
sudo apt-mark hold kubelet kubeadm kubectl
sudo systemctl enable --now kubelet

echo "🔧 Preparing cluster initialization..."
sudo modprobe br_netfilter
grep -qxF 'br_netfilter' /etc/modules || echo 'br_netfilter' | sudo tee -a /etc/modules

sudo kubeadm reset --force --cri-socket=unix:///run/containerd/containerd.sock

REPO_ROOT=$(git rev-parse --show-toplevel)
CONFIG_FILE="/tmp/kubeadm-config.yaml"

echo "📄 Using cluster configuration from repo..."
cp "${REPO_ROOT}/hack/clusterconfig.yaml" "$CONFIG_FILE"

echo "🚀 Initializing Kubernetes cluster..."
unset http_proxy https_proxy no_proxy
sudo kubeadm init --config="${CONFIG_FILE}" --v=5

mkdir -p "$HOME/.kube"
sudo cp -f /etc/kubernetes/admin.conf "$HOME/.kube/config"
sudo chown "$(id -u):$(id -g)" "$HOME/.kube/config"
chmod 600 "$HOME/.kube/config"
export KUBECONFIG="$HOME/.kube/config"

echo "🌐 Installing Flannel CNI..."
FLANNEL_VERSION="${FLANNEL_VERSION:-v0.27.3}"
kubectl apply -f "https://raw.githubusercontent.com/flannel-io/flannel/${FLANNEL_VERSION}/Documentation/kube-flannel.yml"
kubectl taint nodes --all node-role.kubernetes.io/control-plane-

echo "✅ Kubernetes cluster setup completed successfully"
