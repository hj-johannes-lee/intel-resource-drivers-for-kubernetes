#!/usr/bin/env bash
# Install KinD
curl -Lo /tmp/kind https://kind.sigs.k8s.io/dl/v0.31.0/kind-linux-amd64
sudo chmod +x /tmp/kind
sudo mv /tmp/kind /usr/local/bin/kind

# Install kubectl
curl -Lo /tmp/kubectl "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
sudo chmod +x /tmp/kubectl
sudo mv /tmp/kubectl /usr/local/bin/kubectl
