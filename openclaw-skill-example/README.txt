AI Workspace Proxy OpenClaw Skill
===================================

Overview
--------
This folder contains an example OpenClaw skill that can be installed locally so OpenClaw can use the AI Workspace Proxy.

The purpose of this example is to show how an AI agent can access selected Google Workspace services through the proxy, instead of calling Google APIs directly.

Why use the proxy
-----------------
The proxy holds the Google Workspace credentials server-side and enforces a strict allowlist of permitted HTTP operations through `policy.json`.

This means the AI agent does not receive direct Google OAuth credentials and cannot perform operations outside the subset of actions that the proxy explicitly allows.

Typical capabilities
--------------------
Depending on the current proxy policy and the connected Google Workspace account, an OpenClaw skill can use the proxy for operations such as:
- reading Gmail messages and threads
- creating and updating Gmail drafts
- marking emails as read or archiving them when allowed by policy
- reading Google Calendar events
- searching and reading Google Drive content
- creating or updating Google Docs, Sheets, or Slides in approved folders when allowed by policy

Step-by-step setup
------------------
These steps assume you are starting from this repository and have not yet installed the skill in OpenClaw.

1. Copy the skill folder into the OpenClaw workspace skills directory.

   From the root of this repository, run:

   mkdir -p ~/.openclaw/workspace/skills
   cp -R ./openclaw-skill-example/ai-workspace-proxy ~/.openclaw/workspace/skills/

2. Verify that OpenClaw can see the skill.

Run:

openclaw skills list
openclaw skills info ai-workspace-proxy

3. Create the local config directory used by the skill.

mkdir -p ~/.openclaw/configs
chmod 700 ~/.openclaw/configs

4. Create the local config file used by the skill.

cat > ~/.openclaw/configs/ai-workspace-proxy.json <<'EOF'
{
  "proxy_url": "https://workspace-proxy.company.com",
  "proxy_token": "PASTE_YOUR_PROXY_TOKEN_HERE"
}
EOF

Rewrite the JSON values in the config file, to match your current setup.
The `proxy_url` must match the real externally reachable URL of your proxy, the 'proxy_token' is to be replaced with the one given to you in the proxy's Dashboard.
For `proxy_url` beware of the "http" or "https" URL format, adapt according to your current proxy setup.

Then:

chmod 600 ~/.openclaw/configs/ai-workspace-proxy.json

5. Reload the skills in OpenClaw.

There is no dedicated `openclaw refresh skills` CLI command. The usual way is to open the dashboard and ask the agent to refresh skills, or restart OpenClaw.

Start the dashboard:

openclaw dashboard

Then in the chat, type:

refresh skills

6. Run a smoke test.

In the OpenClaw chat, try:

Use skill ai-workspace-proxy and get my latest unread email.

Or from the CLI, try:

openclaw agent --agent main --session-id workspace-proxy-test --message "Refresh skills then use skill ai-workspace-proxy and get my latest unread email."

Expected usage model
--------------------
- the user signs in to the proxy with Google
- the user connects their Google Workspace account to the proxy
- the user retrieves a Proxy API token from the dashboard
- the OpenClaw skill uses that token to call the proxy, reading it from `~/.openclaw/configs/ai-workspace-proxy.json`
- the proxy validates each request before relaying it to Google

Troubleshooting
---------------
If the skill does not appear or is not used:
- run `openclaw skills list`
- run `openclaw skills info ai-workspace-proxy`
- confirm the folder exists at `~/.openclaw/workspace/skills/ai-workspace-proxy`
- confirm the config file exists at `~/.openclaw/configs/ai-workspace-proxy.json`
- reload skills by asking the agent to `refresh skills` in chat, or restart OpenClaw

Notes
-----
- This folder is only an example integration for OpenClaw.
- The exact capabilities available to the skill depend on the current `policy.json` used by the proxy.
- If the proxy's `policy.json` changes significantly, you may want to update the skill instructions in `SKILL.md` so they stay aligned.

Purpose of this example
-----------------------
This example is intended to help teams understand how to connect OpenClaw to the AI Workspace Proxy and how to let AI agents use Google Workspace services in a more controlled and auditable way.
