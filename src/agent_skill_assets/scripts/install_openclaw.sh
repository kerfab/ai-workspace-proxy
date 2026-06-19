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

default_skills_dir=""
if [ -n "${HOME:-}" ]; then
  default_skills_dir="$HOME/.openclaw/workspace/skills"
fi
skills_dir="${OPENCLAW_SKILLS_DIR:-}"

if [ -z "$skills_dir" ] && [ -d "$default_skills_dir" ]; then
  skills_dir="$default_skills_dir"
fi

while [ -z "$skills_dir" ] || [ ! -d "$skills_dir" ]; do
  printf 'OpenClaw skills directory was not found automatically.\n' >&2
  printf 'Enter the full path to the OpenClaw skills directory: ' >&2
  IFS= read -r skills_dir
  skills_dir="${skills_dir%/}"
done

skills_dir="$(CDPATH= cd "$skills_dir" && pwd -P)"
skill_dir="$skills_dir/$skill_name"

home_dir="${HOME:-}"
case "$skills_dir" in
  ""|"/"|"/tmp"|"/tmp/"*)
    echo "Refusing unsafe OpenClaw skills directory: $skills_dir" >&2
    exit 1
    ;;
esac

if [ -n "$home_dir" ] && [ "$skills_dir" = "$home_dir" ]; then
  echo "Refusing unsafe OpenClaw skills directory: $skills_dir" >&2
  exit 1
fi

if [ "$package_dir" = "$skill_dir" ]; then
  sh "$skill_dir/scripts/bootstrap_skill.sh"
  printf '\033[1;32mSUCCESS\033[0m AI Workspace Proxy OpenClaw skill bootstrapped at: %s\n' "$skill_dir"
  exit 0
fi

case "$skill_dir" in
  ""|"/"|"$skills_dir"|"/tmp"|"/tmp/"*)
    echo "Refusing unsafe skill directory: $skill_dir" >&2
    exit 1
    ;;
esac

rm -rf "$skill_dir"
mkdir -p "$skill_dir"
cp -a "$package_dir"/. "$skill_dir"/

sh "$skill_dir/scripts/bootstrap_skill.sh"

printf '\033[1;32mSUCCESS\033[0m AI Workspace Proxy OpenClaw skill installed at: %s\n' "$skill_dir"
