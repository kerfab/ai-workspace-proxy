#!/usr/bin/env bash
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

set -euo pipefail

if [[ "${EUID}" -ne 0 ]]; then
  echo "Please run as root."
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends \
  ca-certificates \
  curl \
  git \
  golang-go \
  build-essential

mkdir -p ./bin ./db ./logs
chmod 700 ./db ./logs

echo "Setup completed."
echo "Next steps:"
echo "  1. Source config.env.example or export equivalent env vars"
echo "  2. Run: make build"
echo "  3. Run: make run"
