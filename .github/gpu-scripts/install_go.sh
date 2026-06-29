#!/usr/bin/env bash
# Copyright (C) 2025 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

export GO_VERSION="${GO_VERSION:-1.24.2}"

echo "⬇️ Installing Go ${GO_VERSION}..."
wget "https://go.dev/dl/go${GO_VERSION}.linux-amd64.tar.gz" -O /tmp/go.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf /tmp/go.tar.gz
export PATH=$PATH:/usr/local/go/bin
go version
echo "✅ Go ${GO_VERSION} installed successfully"
