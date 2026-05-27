#!/usr/bin/env bash
set -euo pipefail

source /etc/environment
# Configuration
ACTIONS_RUNNER_NAME=${ACTIONS_RUNNER_NAME:-}
LDAP_USERNAME=${LDAP_USERNAME:-}
LDAP_PASSWORD=${LDAP_PASSWORD:-}
PROXY_URL=${PROXY_URL:-http://proxy-dmz.intel.com:912}
SYSTEM_IP=${SYSTEM_IP:-}
MAX_SSH_RETRIES=15
RETRY_DELAY=10
UNINSTALL_MODE=false

for arg in "$@"; do
    case "$arg" in
        --uninstall)
            UNINSTALL_MODE=true
            ;;
        *)
            echo "❌ Unknown argument: $arg"
            exit 1
            ;;
    esac
done

if [[ "$ACTIONS_RUNNER_NAME" == "cri" ]]; then
   SSH_USER="gta"
fi

get_system_ip() {

    if [[ -z "${SYSTEM_IP}" || "${SYSTEM_IP}" == "null" ]]; then
        echo "❌ Failed to retrieve system IP"
        exit 1
    fi
    echo "✅ System IP: ${SYSTEM_IP}"
}

wait_for_ssh() {
    echo "⏳ Waiting for SSH..."
    local attempt=1
    while [[ $attempt -le $MAX_SSH_RETRIES ]]; do
        if ssh -o ConnectTimeout=10 -o StrictHostKeyChecking=no -o BatchMode=yes "${SSH_USER}@${SYSTEM_IP}" exit 2>/dev/null; then
            echo "✅ SSH connection established"
            return 0
        fi
        echo "  Retry $attempt/$MAX_SSH_RETRIES..."
        sleep "$RETRY_DELAY"
        ((attempt++))
    done
    echo "❌ Cannot connect to ${SSH_USER}@${SYSTEM_IP}"
    exit 1
}

setup_proxy() {
    echo "📤 Setting up proxy..."
    ssh -o StrictHostKeyChecking=no "${SSH_USER}@${SYSTEM_IP}" "
echo '⚙️ Setting proxy...'
sudo tee -a /etc/environment > /dev/null << 'PROXY_EOF'
http_proxy=\"${PROXY_URL}\"
https_proxy=\"${PROXY_URL}\"
no_proxy=\"127.0.0.1,localhost,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,.intel.com,devel,.svc,.svc.cluster.local\"
HTTP_PROXY=\"${PROXY_URL}\"
HTTPS_PROXY=\"${PROXY_URL}\"
NO_PROXY=\"127.0.0.1,localhost,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,.intel.com,devel,.svc,.svc.cluster.local\"
PROXY_EOF

if [ -f /etc/wgetrc ]; then
    sudo sed -i 's|http://.*|${PROXY_URL}|' /etc/wgetrc
fi

echo '✅ Proxy configured.'
"
}

install_intel_certs_and_dt() {
    echo "🔑 Installing Intel certs and devtool..."
    ssh -o StrictHostKeyChecking=no "${SSH_USER}@${SYSTEM_IP}" << 'EOF'
set -euo pipefail

sudo apt-get update
sudo apt-get --fix-broken install -y
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y unzip curl

echo "🔐 Installing Intel CA certificates..."
curl -s http://certificates.intel.com/repository/certificates/IntelSHA2RootChain-Base64.zip -o /tmp/IntelSHA2RootChain-Base64.zip
sudo unzip -o /tmp/IntelSHA2RootChain-Base64.zip -d /usr/local/share/ca-certificates/
sudo update-ca-certificates --fresh

echo "⬇️ Downloading devtool (dt)..."
(set -o pipefail; curl -fL http://goto.intel.com/getdt | sh) || {
    echo "⚠️ SSL cert failed, retrying with --insecure"
    curl -fkL http://goto.intel.com/getdt | sh
}

echo "📂 Installing devtool..."
chmod +x ~/dt
~/dt update

DT_PROXIES_FILE="$HOME/.config/dt/cache/proxies.json"
if [ -f "$DT_PROXIES_FILE" ]; then
    echo "📤 Setting up proxy for dt..."
    sed -i "s#\"https\": *\"[^\"]*\"#\"https\": \"${https_proxy}\"#" "$DT_PROXIES_FILE"
    ~/dt refresh-proxy
fi

echo "✅ Devtool (dt) installed"
EOF
}

install_and_run_runner() {
    echo "🚧 Installing and starting GitHub runner..."
    
    ssh -o StrictHostKeyChecking=no "${SSH_USER}@${SYSTEM_IP}" 'cat > ~/run_runner.sh' << 'EOSCRIPT'
#!/usr/bin/env bash
set -euo pipefail

echo "📦 Running GitHub runner install script"

~/dt github install-runner \
    --location=gha-runner-setup/actions-runner \
    --ldap-domain=GER \
    --ldap-username="${LDAP_USERNAME}" \
    --ldap-password="${LDAP_PASSWORD}" \
    --no-prompt \
    --name="resource-drivers-for-kubernetes.${ACTIONS_RUNNER_NAME}" \
    --label "${ACTIONS_RUNNER_NAME}"
~/dt github service-runner start --location=gha-runner-setup/actions-runner
EOSCRIPT

    ssh -o StrictHostKeyChecking=no "${SSH_USER}@${SYSTEM_IP}" "chmod +x ~/run_runner.sh && LDAP_USERNAME='${LDAP_USERNAME}' LDAP_PASSWORD='${LDAP_PASSWORD}' ACTIONS_RUNNER_NAME='${ACTIONS_RUNNER_NAME}' ~/run_runner.sh"
}

install_docker_and_registry() {
    echo "🐳 Installing Docker and setting up registry..."
    ssh -o StrictHostKeyChecking=no "${SSH_USER}@${SYSTEM_IP}" << 'EOF'
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

if ! sudo docker ps -a --format '{{.Names}}' | grep -q '^registry$'; then
    sudo docker run -d -p 5000:5000 --restart=always --name registry registry:2
fi
EOF
}

uninstall_runner() {
    echo "🧹 Uninstalling GitHub runner..."
    ssh -o StrictHostKeyChecking=no "${SSH_USER}@${SYSTEM_IP}" "\$HOME/dt github uninstall-runner \
    --location=gha-runner-setup/actions-runner \
    --ldap-domain=GER \
    --ldap-username='${LDAP_USERNAME}' \
    --ldap-password='${LDAP_PASSWORD}' \
    --no-prompt"
    echo "✅ GitHub runner uninstalled"
}

main() {
    get_system_ip
    wait_for_ssh

    if [[ "$UNINSTALL_MODE" == "true" ]]; then
        uninstall_runner
        return 0
    fi

    if [[ "$ACTIONS_RUNNER_NAME" == "gpu" ]]; then
        echo "🕒 Waiting 12 minutes for system boot..."
        sleep 720
    fi
    setup_proxy
    install_intel_certs_and_dt
    install_and_run_runner
    echo "✅ Setup system completed successfully."
}

main
