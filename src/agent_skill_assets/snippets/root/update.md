## Updating/Refreshing This Skill

If the user is already operating through this installed skill, treat requests such as "update the skill", "refresh the skill", "download the skill again", "reinstall the skill", or "reset the skill" as requests to refresh this installed AI Workspace Proxy skill package.

IMPORTANT: When one of those requests is made, run the updater script in this installed skill package.

Run:

`sh "{baseDir}/scripts/update_skill.sh"`

If the proxy requires an agent motive for accountability logging, include a short reason:

`sh "{baseDir}/scripts/update_skill.sh" --agent-motive "<why the skill update is being made>"`

If that script is missing, do not guess paths. Ask the user to reinstall the AI agent skill from the proxy dashboard.

IMPORTANT: After the command succeeds, you MUST forget all prior knowledge loaded from the older version of this skill and re-read this exact file from scratch: `{baseDir}/SKILL.md`

Do not rely on memory from earlier versions of the skill, because policies, workspaces, and instructions may have changed.
