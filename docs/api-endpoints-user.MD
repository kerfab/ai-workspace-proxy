# End-User Backend API Endpoints

This file documents API endpoints intended for the end user, testing tools, and user-owned backend clients that control proxy settings at the user level.

Do not give these endpoints or the end-user backend API key to AI agents.

## Authentication Models

There are two user-facing API categories.

Privileged backend APIs use the end-user backend API key from `user-backend-api-access.config.json`:

```http
Authorization: Bearer <user_backend_api_token>
```

Dashboard helper APIs use the browser session cookie plus CSRF token. They are intended for the web dashboard, not backend automation.

## Privileged Policy API

These endpoints are for user-owned backend tools and tests. Agent API keys are rejected.

```http
GET    /api/user/policies
POST   /api/user/policies
GET    /api/user/policies/{policy_id}
PUT    /api/user/policies/{policy_id}
PATCH  /api/user/policies/{policy_id}
DELETE /api/user/policies/{policy_id}
POST   /api/user/policies/{policy_id}/apply
```

The `system` policy is immutable. It cannot be created, updated, or deleted. It can be applied as the user's default policy or to a connected Workspace.

Create and update requests accept:

```json
{
  "name": "Policy name",
  "capabilities": ["gmail_messages_read", "calendar_read", "gmail_send"],
  "review_required_capabilities": ["gmail_send"]
}
```

`review_required_capabilities` is optional. Every listed capability must also appear in `capabilities`.

## Privileged Workspace API

These endpoints are for user-owned backend tools and tests. Agent API keys are rejected.

### List Connected Workspaces

```http
GET /api/user/workspaces
```

Response:

```json
{
  "workspaces": [
    {
      "email": "person@example.com",
      "name": "Work",
      "auth_connected": true,
      "connection_status": "Active"
    }
  ]
}
```

### Get One Connected Workspace

```http
GET /api/user/workspaces?workspace=<Workspace friendly name or Workspace email>
```

Response:

```json
{
  "workspace": {
    "email": "person@example.com",
    "name": "Work",
    "auth_connected": true,
    "connection_status": "Active"
  }
}
```

## Privileged Drive Folder API

These endpoints are for user-owned backend tools and tests. Agent API keys are rejected.

### List Registered Drive Folder References

```http
GET /api/user/drive-folders?workspace=<Workspace friendly name or Workspace email>
```

Response:

```json
{
  "workspace": {
    "email": "person@example.com",
    "name": "Work",
    "auth_connected": true,
    "connection_status": "Active"
  },
  "drive_folders": [
    {
      "id": "dfr_...",
      "workspace_email": "person@example.com",
      "reference_name": "Test Folder",
      "reference_key": "test folder",
      "folder_id": "folder-id",
      "folder_name": "Live Test Folder",
      "created_at": "2026-06-13T10:00:00Z",
      "updated_at": "2026-06-13T10:00:00Z"
    }
  ]
}
```

This endpoint lists registered root references only. Use the agent Drive Folder Tree API when an AI agent needs the cached subfolder tree.

## Privileged Agent Grant API

These endpoints are for user-owned backend tools and tests. Agent API keys are rejected.

```http
GET /api/user/agents
GET /api/user/agents/{agent_id}/grants
GET /api/user/agents/{agent_id}/grants?workspace=<Workspace>
PUT /api/user/agents/{agent_id}/grants
PATCH /api/user/agents/{agent_id}/grants
GET /api/user/agents/{agent_id}/grants/drive-folders?workspace=<Workspace>
PUT /api/user/agents/{agent_id}/grants/drive-folders
PATCH /api/user/agents/{agent_id}/grants/drive-folders
```

Grant update body:

```json
{
  "workspace": "Work",
  "policy_id": "system",
  "enabled": true,
  "require_agent_motive": false
}
```

Rules:

- omit `enabled` or set it to `true` to create or update one grant
- set `enabled` to `false` to remove the grant for that Workspace
- `require_agent_motive` controls agent accountability for that one agent/Workspace grant

Example grant response:

```json
{
  "grant": {
    "agent_id": "agt_...",
    "workspace_email": "person@example.com",
    "policy_id": "system",
    "policy_name": "Default system policy",
    "require_agent_motive": false,
    "created_at": "2026-06-13T10:00:00Z",
    "updated_at": "2026-06-13T10:00:00Z"
  }
}
```

Drive folder grant update body:

```json
{
  "workspace": "Work",
  "folder_ref_ids": ["dfr_..."]
}
```

The Drive folder grant endpoint returns all registered Drive folders for that Workspace with an `allowed` boolean, plus the current `allowed_drive_folders` list.

## Dashboard Helper API

These endpoints are used by the web dashboard. They require a valid browser session and, where noted, a CSRF token.

### Create Agent Skill Install Token

```http
POST /api/agent-skill/install-token
```

Form fields:

- `platform`: `generic` or `openclaw`
- `agent_id`: target agent

Response:

```json
{
  "expires_at": "2026-06-13T10:10:00Z",
  "expires_at_display": "2026-06-13 12:10:00 CEST",
  "download_url": "http://localhost:8080/api/agent-skill/download?token=...",
  "platform": "openclaw",
  "platform_label": "OpenClaw",
  "install_note": "text",
  "post_install_note": "text",
  "install_command": "shell command bundle"
}
```

### Download Agent Config

```http
GET /api/config/agents-workspace-api-access/download?agent=<agent_id>
```

Downloads:

```text
agents-workspace-api-access.config.json
```

This file contains the selected agent's API key plus the Workspace list/scoping needed by the generated skill.

Example:

```json
{
  "proxy_url": "http://localhost:8080",
  "agent_api_token": "atk_...",
  "workspaces": [
    {
      "email": "person@example.com",
      "name": "Work",
      "policy_id": "system",
      "policy_name": "Default system policy"
    }
  ]
}
```

### Download User Backend Config

```http
GET /api/config/user-backend-api-access/download
```

Downloads:

```text
user-backend-api-access.config.json
```

Example:

```json
{
  "config_type": "user_backend_api_access",
  "proxy_url": "http://localhost:8080",
  "user_backend_api_token": "ubk_..."
}
```

### Rotate User Backend API Key

```http
POST /api/config/user-backend-api-access/rotate
```

Rotation immediately invalidates the previous end-user backend API key.

## Common Error Shape

Error responses are JSON:

```json
{
  "error": "invalid_policy",
  "message": "explanation"
}
```

Common statuses:

- `400`: invalid JSON, invalid policy, or invalid request body
- `401`: missing or invalid end-user backend API key
- `404`: policy, Workspace, or agent not found
- `405`: unsupported HTTP method
