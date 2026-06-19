#!/bin/sh
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

set -eu

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
skill_dir="$(CDPATH= cd "$script_dir/.." && pwd -P)"

test -f "$skill_dir/SKILL.md"
test -f "$skill_dir/scripts/workspace_proxy_tool.py"
test -f "$skill_dir/scripts/update_skill.sh"
test -f "$skill_dir/scripts/bootstrap_skill.sh"
test -f "$skill_dir/config/agents-workspace-api-access.config.json"

home_dir="${HOME:-}"
case "$skill_dir" in
  "/"|"/tmp"|"/tmp/"*)
    echo "Refusing unsafe skill directory: $skill_dir" >&2
    exit 1
    ;;
esac

if [ -n "$home_dir" ] && [ "$skill_dir" = "$home_dir" ]; then
  echo "Refusing unsafe skill directory: $skill_dir" >&2
  exit 1
fi

marker="{baseDir}"
SKILL_DIR_FOR_REWRITE="$skill_dir" python3 - "$marker" <<'PY'
import os
import re
import sys
from pathlib import Path

marker = sys.argv[1]
skill_dir = Path(os.environ["SKILL_DIR_FOR_REWRITE"]).resolve()
replacement = str(skill_dir)
path_marker = marker + "/"
path_replacement = replacement + "/"
inline_code = re.compile(r"`([^`\n]*" + re.escape(path_marker) + r"[^`\n]*)`")

def replace_code_path(match):
    return "`" + match.group(1).replace(path_marker, path_replacement) + "`"

for path in skill_dir.rglob("*.md"):
    text = path.read_text(encoding="utf-8")
    if path_marker not in text:
        continue
    path.write_text(inline_code.sub(replace_code_path, text), encoding="utf-8")
PY

printf 'AI Workspace Proxy skill bootstrapped at: %s\n' "$skill_dir"
