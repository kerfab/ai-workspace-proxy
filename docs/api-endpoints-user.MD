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

These endpoints are for user-owned backend tools and tests. Standard agent Workspace API keys are rejected.

The `system` policy is immutable. It cannot be created, updated, or deleted. It can be applied as the user's default policy or to a connected Workspace.

### List Policies

```http
GET /api/user/policies
```

Authentication:

```http
Authorization: Bearer <user_backend_api_token>
```

Response:

```json
{
  "policies": [
    {
      "id": "system",
      "name": "Default system policy",
      "capabilities": ["calendar_read"],
      "system": true,
      "immutable": true,
      "is_default": true
    }
  ]
}
```

### Create Custom Policy

```http
POST /api/user/policies
```

Authentication:

```http
Authorization: Bearer <user_backend_api_token>
Content-Type: application/json
```

Request:

```json
{
  "name": "Policy name",
  "capabilities": ["gmail_messages_read", "calendar_read"]
}
```

The server generates the policy ID. Supplying an `id` is rejected. Supplying `id: "system"` is rejected because the system policy is immutable.

Response:

```json
{
  "policy": {
    "id": "generated-policy-id",
    "name": "Policy name",
    "capabilities": ["calendar_read", "gmail_messages_read"],
    "system": false,
    "immutable": false,
    "is_default": false,
    "created_at": "2026-06-13T10:00:00Z",
    "updated_at": "2026-06-13T10:00:00Z"
  }
}
```

### Get Policy

```http
GET /api/user/policies/{policy_id}
```

Authentication:

```http
Authorization: Bearer <user_backend_api_token>
```

Use `system` as `{policy_id}` to retrieve the immutable system policy.

### Update Custom Policy

```http
PUT /api/user/policies/{policy_id}
PATCH /api/user/policies/{policy_id}
```

Authentication:

```http
Authorization: Bearer <user_backend_api_token>
Content-Type: application/json
```

Request:

```json
{
  "name": "Updated policy name",
  "capabilities": ["gmail_messages_read", "calendar_read"]
}
```

Rules:

- `{policy_id}` must be a custom policy owned by the authenticated user.
- `system` cannot be updated.
- If the body contains `id`, it must match `{policy_id}`.
- Unknown capability keys are rejected.

### Delete Custom Policy

```http
DELETE /api/user/policies/{policy_id}
```

Authentication:

```http
Authorization: Bearer <user_backend_api_token>
```

Rules:

- `{policy_id}` must be a custom policy owned by the authenticated user.
- `system` cannot be deleted.
- If the deleted policy was applied as the default policy or to a Workspace, those assignments are reverted to `system`.

Response:

```json
{
  "deleted_policy_id": "deleted-policy-id",
  "reverted_to": "system",
  "reverted_applied": true
}
```

### Apply Policy

```http
POST /api/user/policies/{policy_id}/apply
```

Authentication:

```http
Authorization: Bearer <user_backend_api_token>
Content-Type: application/json
```

Apply a policy as the user default policy with an empty body or `{}`:

```json
{}
```

Apply a policy to one connected Workspace account by friendly name or email:

```json
{
  "workspace": "work"
}
```

The endpoint accepts `system` as `{policy_id}`.

Response when applied as default:

```json
{
  "applied_as": "default",
  "policy": {
    "id": "system",
    "name": "Default system policy"
  }
}
```

Response when applied to a Workspace:

```json
{
  "applied_as": "workspace",
  "workspace": "person@example.com",
  "policy": {
    "id": "system",
    "name": "Default system policy"
  }
}
```

## Privileged Workspace API

These endpoints are for user-owned backend tools and tests. Standard agent Workspace API keys are rejected.

### List Connected Workspaces

```http
GET /api/user/workspaces
```

Authentication:

```http
Authorization: Bearer <user_backend_api_token>
```

Response:

```json
{
  "workspaces": [
    {
      "email": "person@example.com",
      "name": "work",
      "policy_id": "system",
      "policy_name": "Default system policy"
    }
  ]
}
```

### Get One Connected Workspace

```http
GET /api/user/workspaces?workspace=<Workspace friendly name or Workspace email>
```

Authentication:

```http
Authorization: Bearer <user_backend_api_token>
```

Response:

```json
{
  "workspace": {
    "email": "person@example.com",
    "name": "work",
    "policy_id": "system",
    "policy_name": "Default system policy"
  }
}
```

## Privileged Drive Folder API

These endpoints are for user-owned backend tools and tests. Standard agent Workspace API keys are rejected.

### List Allowed Drive Folder References

```http
GET /api/user/drive-folders?workspace=<Workspace friendly name or Workspace email>
```

Authentication:

```http
Authorization: Bearer <user_backend_api_token>
```

Response:

```json
{
  "workspace": {
    "email": "person@example.com",
    "name": "work",
    "policy_id": "system",
    "policy_name": "Default system policy"
  },
  "drive_folders": [
    {
      "id": "dfr_...",
      "workspace_email": "person@example.com",
      "reference_name": "Test Drive",
      "reference_key": "test drive",
      "folder_id": "folder-id",
      "folder_name": "Live Test Folder",
      "allow_docs": true,
      "allow_sheets": false,
      "allow_slides": false,
      "allow_drive_files": true,
      "allowed_types": ["docs", "drive_files"],
      "created_at": "2026-06-13T10:00:00Z",
      "updated_at": "2026-06-13T10:00:00Z"
    }
  ]
}
```

This endpoint lists the configured root folder references only. Use the agent Drive Folder Tree API when an AI agent needs the cached subfolder tree.

## Dashboard Helper API

These endpoints are used by the web dashboard. They require a valid browser session and CSRF token where applicable.

They are documented here so testers understand the full user-level API surface, but backend test clients should prefer the privileged backend API key where such an endpoint exists.

### Create Agent Skill Install Token

```http
POST /api/agent-skill/install-token
```

Authentication:

- browser session cookie
- CSRF token in form field `csrf_token`

Form fields:

- `platform`: `generic` or `openclaw`

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

### Download Agent Workspace API Config

```http
GET /api/config/agents-workspace-api-access/download
```

Authentication:

- browser session cookie

Downloads:

```text
agents-workspace-api-access.config.json
```

This file contains the standard agent Workspace API key and connected Workspace list.

### Rotate Agent Workspace API Key

```http
POST /api/config/agents-workspace-api-access/rotate
```

Authentication:

- browser session cookie
- CSRF token in form field `csrf_token`

Rotation immediately invalidates the previous standard agent Workspace API key.

### Download User Backend API Config

```http
GET /api/config/user-backend-api-access/download
```

Authentication:

- browser session cookie

Downloads:

```text
user-backend-api-access.config.json
```

This file contains the end-user backend API key.

### Rotate User Backend API Key

```http
POST /api/config/user-backend-api-access/rotate
```

Authentication:

- browser session cookie
- CSRF token in form field `csrf_token`

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

- `400`: invalid JSON, invalid policy, unknown capability, or attempt to mutate `system`
- `401`: missing or invalid end-user backend API key
- `404`: policy or Workspace not found
- `405`: unsupported HTTP method
