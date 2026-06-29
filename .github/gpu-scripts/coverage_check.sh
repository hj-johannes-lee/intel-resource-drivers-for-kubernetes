#!/usr/bin/env bash
# Copyright (C) 2025-2026 Intel Corporation
#
# SPDX-License-Identifier: Apache-2.0

set -euo pipefail

make "$1" | tee coverage.txt
RESULT=$(awk '/total:/ {print ($3+0)}' coverage.txt)

if (( $(echo "$RESULT >= $2" | bc -l) )); then
    echo "$1 $RESULT% meets threshold $2%"
    exit 0
else
    echo "$1 $RESULT% is below threshold $2%. Add more tests!"
    exit 1
fi
