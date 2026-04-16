#!/bin/sh
set -eu

: "${DB_PATH:=/data/db/ai_workspace_proxy.sqlite3}"
: "${DENIED_LOG_PATH:=/data/logs/denied.log}"

mkdir -p /data/db
mkdir -p /data/logs
mkdir -p "$(dirname "$DB_PATH")"
mkdir -p "$(dirname "$DENIED_LOG_PATH")"

exec "$@"
