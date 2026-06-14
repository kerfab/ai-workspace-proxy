## Updating/Refreshing This Skill

When the user asks to update or refresh this skill, run the updater script in this installed skill package.

Run:

`sh "{baseDir}/scripts/update_skill.sh"`

If that script is missing, do not guess paths. Ask the user to reinstall the AI agent skill from the proxy dashboard.

After the command succeeds, start fresh by reading the new `SKILL.md` again. Ignore all previous knowledge loaded from older Markdown files for this skill, because policies, workspaces, and instructions may have changed.
