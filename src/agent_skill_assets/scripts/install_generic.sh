#!/bin/sh
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

set -eu

skill_name="ai-workspace-proxy"

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

script_dir="$(CDPATH= cd "$(dirname "$script_path")" && pwd -P)"
package_dir="$(CDPATH= cd "$script_dir/.." && pwd -P)"

test -f "$package_dir/SKILL.md"
test -f "$package_dir/scripts/workspace_proxy_tool.py"
test -f "$package_dir/scripts/update_skill.sh"
test -f "$package_dir/scripts/bootstrap_skill.sh"
test -f "$package_dir/config/agents-workspace-api-access.config.json"

prompt_skill_dir() {
  prompt='Enter the full path where the AI Workspace Proxy skill should be installed: '
  if [ -t 0 ] && command -v bash >/dev/null 2>&1; then
    bash -c 'read -e -r -p "$1" value || exit 1; printf "%s\n" "$value"' sh "$prompt"
    return
  fi
  printf '%s' "$prompt" >&2
  IFS= read -r value
  printf '%s\n' "$value"
}

skill_dir="${AI_WORKSPACE_PROXY_SKILL_DIR:-}"
if [ -z "$skill_dir" ]; then
  skill_dir="$(prompt_skill_dir)"
fi
skill_dir="${skill_dir%/}"

if [ -z "$skill_dir" ]; then
  echo "Install path is required." >&2
  exit 1
fi

case "$skill_dir" in
  ""|"/"|"/tmp"|"/tmp/"*)
    echo "Refusing unsafe skill directory: $skill_dir" >&2
    exit 1
    ;;
esac

home_dir="${HOME:-}"
if [ -n "$home_dir" ] && [ "$skill_dir" = "$home_dir" ]; then
  echo "Refusing unsafe skill directory: $skill_dir" >&2
  exit 1
fi

if [ "$package_dir" = "$skill_dir" ]; then
  sh "$skill_dir/scripts/bootstrap_skill.sh"
  printf '\033[1;32mSUCCESS\033[0m AI Workspace Proxy skill bootstrapped at: %s\n' "$skill_dir"
  exit 0
fi

rm -rf "$skill_dir"
mkdir -p "$skill_dir"
cp -a "$package_dir"/. "$skill_dir"/

sh "$skill_dir/scripts/bootstrap_skill.sh"

printf '\033[1;32mSUCCESS\033[0m AI Workspace Proxy skill installed at: %s\n' "$skill_dir"
