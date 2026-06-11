## Updating This Skill

When the user asks to update or refresh this skill, run the updater script bundled with this skill.

The script may not have executable permissions. Run it with `sh`; do not try to execute it directly.

If the shell is already in the skill root, run:

`sh ./scripts/update_skill.sh`

If the shell is not in the skill root, run the script by full path:

`sh /full/path/to/ai-workspace-proxy/scripts/update_skill.sh`

If the script cannot locate the installed skill folder, provide the skill root explicitly:

`AI_WORKSPACE_PROXY_SKILL_DIR=/full/path/to/ai-workspace-proxy sh /full/path/to/ai-workspace-proxy/scripts/update_skill.sh`

Notes:

- The updater downloads the fresh package, unzips it into `/tmp`, replaces the current skill files, then deletes the temporary zip and folder.
- The updater uses the proxy token from `config/config.json` and is not tied to a specific workspace.
- After the command succeeds, start fresh by reading the new `SKILL.md` again. Ignore all previous knowledge loaded from older Markdown files for this skill, because policies, workspaces, and instructions may have changed.
