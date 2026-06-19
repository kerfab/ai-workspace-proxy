#!/usr/bin/env python3
# Copyright (c) 2026 Opensense Ltd. (Hong Kong). All rights reserved.
# Proprietary software. No use, copy, modification, distribution, disclosure,
# or reverse engineering is permitted without prior written authorization
# from Opensense Ltd.

"""Verify update_skill.sh forwards motive and platform to the proxy helper."""

import json
import os
import shutil
import subprocess
import tempfile
import textwrap
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]
UPDATE_SCRIPT = REPO_ROOT / "src" / "agent_skill_assets" / "scripts" / "update_skill.sh"


def write(path: Path, content: str, mode: int | None = None) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(content, encoding="utf-8")
    if mode is not None:
        path.chmod(mode)


def create_skill_dir(root: Path, captured_env_path: Path) -> None:
    write(root / "SKILL.md", "# AI Workspace Proxy\n")
    write(root / "config" / "agents-workspace-api-access.config.json", "{}\n")
    (root / "scripts").mkdir(parents=True, exist_ok=True)
    shutil.copy2(UPDATE_SCRIPT, root / "scripts" / "update_skill.sh")
    write(root / "scripts" / "install_skill.sh", "#!/bin/sh\nexit 0\n", 0o755)
    write(root / "scripts" / "bootstrap_skill.sh", "#!/bin/sh\nexit 0\n", 0o755)
    write(
        root / "scripts" / "workspace_proxy_tool.py",
        textwrap.dedent(
            f"""\
            #!/usr/bin/env python3
            import json
            import os
            import sys
            import zipfile
            from pathlib import Path

            captured = {{
                "agent_motive": os.environ.get("AIWP_AGENT_MOTIVE", ""),
                "argv": sys.argv[1:],
            }}
            Path({str(captured_env_path)!r}).write_text(json.dumps(captured), encoding="utf-8")

            save_to = sys.argv[sys.argv.index("--save-to") + 1]
            files = {{
                "SKILL.md": "# AI Workspace Proxy\\n",
                "config/agents-workspace-api-access.config.json": "{{}}\\n",
                "scripts/workspace_proxy_tool.py": "#!/usr/bin/env python3\\n",
                "scripts/update_skill.sh": "#!/bin/sh\\nexit 0\\n",
                "scripts/install_skill.sh": "#!/bin/sh\\nexit 0\\n",
                "scripts/bootstrap_skill.sh": "#!/bin/sh\\nexit 0\\n",
            }}
            with zipfile.ZipFile(save_to, "w") as archive:
                for name, content in files.items():
                    archive.writestr(name, content)
            """
        ),
        0o755,
    )


def main() -> int:
    with tempfile.TemporaryDirectory(prefix=".update-skill-test-", dir=REPO_ROOT) as raw_tmp:
        tmp = Path(raw_tmp)
        skill_dir = tmp / "installed-skill"
        captured_env_path = tmp / "captured.json"
        zip_path = tmp / "skill.zip"
        new_skill_dir = tmp / "new-skill"
        create_skill_dir(skill_dir, captured_env_path)

        env = os.environ.copy()
        env["AI_WORKSPACE_PROXY_SKILL_DIR"] = str(skill_dir)
        env["AI_WORKSPACE_PROXY_SKILL_ZIP"] = str(zip_path)
        env["AI_WORKSPACE_PROXY_NEW_SKILL_DIR"] = str(new_skill_dir)
        result = subprocess.run(
            [
                "sh",
                str(skill_dir / "scripts" / "update_skill.sh"),
                "--agent-motive",
                "Refreshing the skill so the user has current policy instructions.",
            ],
            cwd=REPO_ROOT,
            env=env,
            text=True,
            capture_output=True,
            check=False,
        )
        if result.returncode != 0:
            print(result.stdout)
            print(result.stderr)
            return result.returncode

        captured = json.loads(captured_env_path.read_text(encoding="utf-8"))
        expected = {
            "agent_motive": "Refreshing the skill so the user has current policy instructions.",
        }
        for key, value in expected.items():
            if captured.get(key) != value:
                print(f"Unexpected {key}: {captured.get(key)!r}")
                print(json.dumps(captured, indent=2))
                return 1
        if captured.get("argv") != ["proxy", "download-skill", "--platform", "generic", "--save-to", str(zip_path)]:
            print("Unexpected helper argv:")
            print(json.dumps(captured, indent=2))
            return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
