#!/bin/sh
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

set -eu

: "${DB_PATH:=/data/db/ai_workspace_proxy.sqlite3}"

mkdir -p /data/db
mkdir -p "$(dirname "$DB_PATH")"

exec "$@"
