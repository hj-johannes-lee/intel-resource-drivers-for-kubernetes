#!/usr/bin/env bash
set -euo pipefail

# Configuration
ACTIONS_RUNNER_NAME=${ACTIONS_RUNNER_NAME:-}
LDAP_USERNAME=${LDAP_USERNAME:-}
LDAP_PASSWORD=${LDAP_PASSWORD:-}

MAX_SSH_RETRIES=15
RETRY_DELAY=10
SYSTEM_ID="573563"
SSH_USER="sdp"

get_system_ip() {
    echo "$ACTIONS_RUNNER_NAME"
    if [[ "$ACTIONS_RUNNER_NAME" == "coral" ]]; then
        os_ip=10.10.10.10
        return 0
    fi
    echo "Retrieving system IP..."
    local response
    response=$(curl -s -X GET "https://onecloudapi.intel.com/${CI_USER_ONECLOUD_TOKEN}/system/info/${SYSTEM_ID}")
    os_ip=$(echo "${response}" | jq -r '.osip')
    if [[ -z "${os_ip}" || "${os_ip}" == "null" ]]; then
        echo "❌ Failed to retrieve system IP"
        exit 1
    fi
    echo "✅ System IP: ${os_ip}"
}

wait_for_ssh() {
    echo "⏳ Waiting for SSH..."
    local attempt=1
    while [[ $attempt -le $MAX_SSH_RETRIES ]]; do
        if ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=no -o BatchMode=yes "${SSH_USER}@${os_ip}" exit 2>/dev/null; then
            echo "✅ SSH connection established"
            return 0
        fi
        echo "  Retry $attempt/$MAX_SSH_RETRIES..."
        sleep "$RETRY_DELAY"
        ((attempt++))
    done
    echo "❌ Cannot connect to ${SSH_USER}@${os_ip}"
    exit 1
}

setup_proxy() {
    echo "📤 Setting up proxy..."
    ssh -o StrictHostKeyChecking=no "${SSH_USER}@${os_ip}" << 'EOF'
echo "⚙️ Setting proxy..."
sudo tee -a /etc/environment > /dev/null << 'PROXY_EOF'
http_proxy=http://proxy-dmz.intel.com:911
https_proxy=http://proxy-dmz.intel.com:912
no_proxy=127.0.0.1,localhost,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,downloadmirror.intel.com,devel,.svc,.svc.cluster.local
PROXY_EOF

echo "✅ Proxy configured."
EOF
}

install_intel_certs_and_dt() {
    echo "🔑 Installing Intel certs and devtool..."
    ssh -o StrictHostKeyChecking=no "${SSH_USER}@${os_ip}" << 'EOF'
set -euo pipefail

sudo apt-get update
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y unzip curl

echo "🔐 Installing Intel CA certificates..."
curl -s http://certificates.intel.com/repository/certificates/IntelSHA2RootChain-Base64.zip -o /tmp/IntelSHA2RootChain-Base64.zip
sudo unzip -o /tmp/IntelSHA2RootChain-Base64.zip -d /usr/local/share/ca-certificates/
sudo update-ca-certificates --fresh

echo "⬇️ Downloading devtool (dt)..."
curl --retry 3 --fail --location https://gfx-assets.intel.com/artifactory/gfx-build-assets/build-tools/devtool-go/latest/artifacts/linux64/dt --output ~/dt || {
    echo "⚠️ SSL cert failed, retrying with --insecure"
    curl --retry 3 --fail --insecure --location https://gfx-assets.intel.com/artifactory/gfx-build-assets/build-tools/devtool-go/latest/artifacts/linux64/dt --output ~/dt
}

echo "📂 Installing devtool..."
chmod +x ~/dt

echo "✅ Devtool (dt) installed"
EOF
}

cleanup_existing_runner() {
    echo "🧹 Cleaning up existing runner installations..."
    ssh -o StrictHostKeyChecking=no "${SSH_USER}@${os_ip}" << 'EOF'
set -euo pipefail

echo "🛑 Stopping any running GitHub Actions services..."
# Stop all runner services
sudo systemctl stop actions.runner.* || true
sudo pkill -f Runner.Listener || true
sudo pkill -f Runner.Worker || true

echo "🗂️ Removing existing runner directories..."
# Clean up runner directories
rm -rf ~/gha-runner-setup || true
rm -rf ~/.local/share/powershell || true

echo "🔄 Removing any systemd services..."
# Remove systemd services
sudo rm -f /etc/systemd/system/actions.runner.* || true
sudo systemctl daemon-reload

echo "✅ Cleanup completed"
EOF
}

install_and_run_runner() {
    echo "🚧 Installing and starting GitHub runner..."

    ssh -o StrictHostKeyChecking=no "${SSH_USER}@${os_ip}" 'cat > ~/run_runner.sh' << 'EOSCRIPT'
#!/usr/bin/env bash
set -euo pipefail

echo "📦 Running GitHub runner install script"

mkdir -p ~/gha-runner-setup
cd ~/gha-runner-setup

~/dt github install-runner \
    --location=~/gha-runner-setup/actions-runner \
    --ldap-domain=GER \
    --ldap-username="${LDAP_USERNAME}" \
    --ldap-password="${LDAP_PASSWORD}" \
    --no-prompt \
    --name="resource-drivers-for-kubernetes.${ACTIONS_RUNNER_NAME}" \
    --label "${ACTIONS_RUNNER_NAME}"
EOSCRIPT

    ssh -o StrictHostKeyChecking=no "${SSH_USER}@${os_ip}" "chmod +x ~/run_runner.sh && LDAP_USERNAME='${LDAP_USERNAME}' LDAP_PASSWORD='${LDAP_PASSWORD}' ACTIONS_RUNNER_NAME='${ACTIONS_RUNNER_NAME}' ~/run_runner.sh"
}

install_docker_and_registry() {
    echo "🐳 Installing Docker and setting up registry..."
    ssh -o StrictHostKeyChecking=no "${SSH_USER}@${os_ip}" << 'EOF'
set -euo pipefail

sudo apt-get update
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates \
    curl \
    gnupg \
    lsb-release

sudo mkdir -p /etc/apt/keyrings
sudo rm -f /etc/apt/keyrings/docker.gpg
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /etc/apt/keyrings/docker.gpg

echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" \
    | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null

sudo apt-get update
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y docker-ce docker-ce-cli containerd.io docker-compose-plugin

mkdir -p /home/"$(whoami)"/.docker
echo '{
 "proxies":
 {
   "default":
   {
 	"httpProxy": "'$http_proxy'",
 	"httpsProxy": "'$https_proxy'",
 	"noProxy": "'$no_proxy'"
   }
 }
}' > /home/"$(whoami)"/.docker/config.json

sudo mkdir -p /etc/systemd/system/docker.service.d/
echo '[Service]
Environment="HTTP_PROXY='$http_proxy'"
Environment="HTTPS_PROXY='$https_proxy'"
Environment="NO_PROXY='$no_proxy'"' \
    | sudo tee /etc/systemd/system/docker.service.d/proxy.conf

sudo tee /etc/docker/daemon.json << JSON
{
  "registry-mirrors": ["https://cache-registry.caas.intel/cache"]
}
JSON

sudo usermod -aG docker "$(whoami)"

sudo systemctl daemon-reexec
sudo systemctl daemon-reload
sudo systemctl restart docker

sudo docker run -d -p 5000:5000 --restart=always --name registry registry:2
EOF
}

main() {
    if [[ "$ACTIONS_RUNNER_NAME" == "gpu" ]]; then
        echo "🕒 Waiting 12 minutes for system boot..."
        sleep 720
    fi
    get_system_ip
    wait_for_ssh
    setup_proxy
    install_docker_and_registry
    install_intel_certs_and_dt
    cleanup_existing_runner
    install_and_run_runner
    echo "✅ Setup system completed successfully."
}

main
