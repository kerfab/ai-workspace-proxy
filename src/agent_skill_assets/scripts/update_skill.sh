#!/bin/sh
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

set -eu

ZIP_PATH="${AI_WORKSPACE_PROXY_SKILL_ZIP:-/tmp/ai-workspace-proxy-agent-skill.zip}"
NEW_SKILL_DIR="${AI_WORKSPACE_PROXY_NEW_SKILL_DIR:-/tmp/ai-workspace-proxy-skill}"
AGENT_MOTIVE="${AIWP_AGENT_MOTIVE:-}"
SKILL_PLATFORM="${AI_WORKSPACE_PROXY_SKILL_PLATFORM:-}"

usage() {
  cat <<'EOF'
Usage: update_skill.sh [--agent-motive MOTIVE] [--platform generic|openclaw]

Refresh the installed AI Workspace Proxy skill package.

Options:
  --agent-motive TEXT       One or two sentences explaining why the update is being made
  --platform VALUE          Skill platform to refresh; detected automatically when omitted
  -h, --help                Show this help

The motive can also be supplied with AIWP_AGENT_MOTIVE. Command-line options
override environment variables.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --agent-motive)
      [ "$#" -ge 2 ] || { echo "Missing value for --agent-motive" >&2; exit 2; }
      AGENT_MOTIVE="$2"
      shift 2
      ;;
    --agent-motive=*)
      AGENT_MOTIVE="${1#--agent-motive=}"
      shift
      ;;
    --platform)
      [ "$#" -ge 2 ] || { echo "Missing value for --platform" >&2; exit 2; }
      SKILL_PLATFORM="$2"
      shift 2
      ;;
    --platform=*)
      SKILL_PLATFORM="${1#--platform=}"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

is_skill_dir() {
  dir="$1"
  [ -n "$dir" ] || return 1
  [ -f "$dir/SKILL.md" ] || return 1
  [ -f "$dir/scripts/workspace_proxy_tool.py" ] || return 1
  [ -f "$dir/scripts/bootstrap_skill.sh" ] || return 1
  [ -f "$dir/config/agents-workspace-api-access.config.json" ] || return 1
  grep -q '^# AI Workspace Proxy$' "$dir/SKILL.md"
}

canonical_dir() {
  CDPATH= cd "$1" && pwd -P
}

find_skill_dir() {
  if [ -n "${AI_WORKSPACE_PROXY_SKILL_DIR:-}" ]; then
    canonical_dir "$AI_WORKSPACE_PROXY_SKILL_DIR"
    return
  fi

  script_path="$0"
  case "$script_path" in
    */*) ;;
    *)
      found_script="$(command -v "$script_path" 2>/dev/null || true)"
      if [ -n "$found_script" ]; then
        script_path="$found_script"
      fi
      ;;
  esac

  script_dir="$(CDPATH= cd "$(dirname "$script_path")" 2>/dev/null && pwd -P || true)"
  if [ -n "$script_dir" ]; then
    parent_dir="$(CDPATH= cd "$script_dir/.." 2>/dev/null && pwd -P || true)"
    if is_skill_dir "$parent_dir"; then
      printf '%s\n' "$parent_dir"
      return
    fi
  fi

  if [ -n "${HOME:-}" ]; then
    set -- "$PWD" "$HOME/.openclaw" "$HOME/.codex" "$HOME"
  else
    set -- "$PWD"
  fi

  for root do
    [ -d "$root" ] || continue
    find "$root" -type f -name SKILL.md -print 2>/dev/null
  done | while IFS= read -r skill_file; do
    skill_dir="$(dirname "$skill_file")"
    if is_skill_dir "$skill_dir"; then
      canonical_dir "$skill_dir"
      break
    fi
  done
}

SKILL_DIR="$(find_skill_dir)"
test -n "$SKILL_DIR"
is_skill_dir "$SKILL_DIR"

if [ -z "$SKILL_PLATFORM" ]; then
  if grep -q 'OpenClaw' "$SKILL_DIR/scripts/install_skill.sh"; then
    SKILL_PLATFORM="openclaw"
  else
    SKILL_PLATFORM="generic"
  fi
fi

case "$SKILL_PLATFORM" in
  generic|openclaw) ;;
  *)
    echo "Invalid platform: $SKILL_PLATFORM" >&2
    exit 2
    ;;
esac

home_dir="${HOME:-}"
if [ -n "$home_dir" ] && [ "$SKILL_DIR" = "$home_dir" ]; then
  echo "Refusing unsafe skill directory: $SKILL_DIR" >&2
  exit 1
fi

case "$SKILL_DIR" in
  "/"|"/tmp"|"/tmp/"*)
    echo "Refusing unsafe skill directory: $SKILL_DIR" >&2
    exit 1
    ;;
esac

AIWP_AGENT_MOTIVE="$AGENT_MOTIVE" \
python3 "$SKILL_DIR/scripts/workspace_proxy_tool.py" proxy download-skill --platform "$SKILL_PLATFORM" --save-to "$ZIP_PATH"

rm -rf "$NEW_SKILL_DIR"
mkdir -p "$NEW_SKILL_DIR"
unzip -q "$ZIP_PATH" -d "$NEW_SKILL_DIR"

test -f "$NEW_SKILL_DIR/SKILL.md"
test -f "$NEW_SKILL_DIR/scripts/workspace_proxy_tool.py"
test -f "$NEW_SKILL_DIR/scripts/update_skill.sh"
test -f "$NEW_SKILL_DIR/scripts/install_skill.sh"
test -f "$NEW_SKILL_DIR/scripts/bootstrap_skill.sh"
test -f "$NEW_SKILL_DIR/config/agents-workspace-api-access.config.json"

find "$SKILL_DIR" -mindepth 1 -maxdepth 1 -exec rm -rf {} +
cp -a "$NEW_SKILL_DIR"/. "$SKILL_DIR"/

sh "$SKILL_DIR/scripts/bootstrap_skill.sh"

rm -rf "$ZIP_PATH" "$NEW_SKILL_DIR"

printf 'AI Workspace Proxy skill updated at: %s\n' "$SKILL_DIR"
