# Agent And Skill API Endpoints

This file documents endpoints intended for AI agents and generated skill packages.

Agents must use the standard agent Workspace API key from `agents-workspace-api-access.config.json`:

```http
Authorization: Bearer <proxy_token>
```

Do not use the end-user backend API key from `user-backend-api-access.config.json` for agent operations.

## Workspace Selection

Most agent endpoints require a connected Workspace selector:

```text
workspace=<Workspace friendly name or Workspace email>
```

The selector is checked against Workspaces connected by the same proxy user. Supplying another user's Workspace email cannot grant access.

## Google API Proxy Relay

These endpoints forward requests to Google only after the proxy authenticates the agent, resolves the Workspace, rewrites Drive-related requests when needed, and enforces the Workspace policy.

All relay requests require:

- `Authorization: Bearer <proxy_token>`
- `workspace=<Workspace friendly name or email>`
- the original Google API method, path, query, and JSON/body payload expected by Google

The proxy removes `workspace` before forwarding the request to Google.

### Gmail

Gmail API paths call email resources `messages`; this is the Google API term for Gmail emails.

```http
<METHOD> /gmail.googleapis.com/<gmail-api-path>?workspace=<Workspace>
```

Example:

```http
GET /gmail.googleapis.com/gmail/v1/users/me/messages?workspace=work&q=is%3Aunread
```

### Contacts

People API contact lookup paths search contacts and directory entries through the single `contacts_read` policy permission. The proxy restricts these routes to read-only search endpoints and clamps returned fields.

```http
<METHOD> /people.googleapis.com/<people-api-path>?workspace=<Workspace>
```

Example:

```http
GET /people.googleapis.com/v1/people:searchContacts?workspace=work&query=Frederic
```

### Calendar

```http
<METHOD> /calendar.googleapis.com/<calendar-api-path>?workspace=<Workspace>
```

Example:

```http
GET /calendar.googleapis.com/calendar/v3/users/me/calendarList?workspace=work
```

### Drive

```http
<METHOD> /drive.googleapis.com/<drive-api-path>?workspace=<Workspace>
```

Example:

```http
GET /drive.googleapis.com/drive/v3/files?workspace=work&q=name%20contains%20%27Report%27
```

### Docs

```http
<METHOD> /docs.googleapis.com/<docs-api-path>?workspace=<Workspace>
```

Example:

```http
GET /docs.googleapis.com/v1/documents/<documentId>?workspace=work
```

### Sheets

```http
<METHOD> /sheets.googleapis.com/<sheets-api-path>?workspace=<Workspace>
```

Example:

```http
GET /sheets.googleapis.com/v4/spreadsheets/<spreadsheetId>?workspace=work
```

### Slides

```http
<METHOD> /slides.googleapis.com/<slides-api-path>?workspace=<Workspace>
```

Example:

```http
GET /slides.googleapis.com/v1/presentations/<presentationId>?workspace=work
```

## Drive Folder Tree API

These endpoints expose the proxy's cached Drive folder tree for registered Drive folder references. They are used by agents to discover allowed subfolders and refresh the cache when needed.

### List Cached Drive Folder Trees

```http
GET /api/drive-folders/tree?workspace=<Workspace>
GET /api/drive-folders/tree?workspace=<Workspace>&driveRef=<Reference Name>
```

Authentication:

```http
Authorization: Bearer <proxy_token>
```

Response:

```json
{
  "driveFolders": [
    {
      "referenceName": "Accounting",
      "referenceKey": "accounting",
      "rootFolderId": "folder-id",
      "rootName": "Accounting",
      "refreshedAt": "2026-06-13T10:00:00Z",
      "folders": [
        {
          "folderId": "folder-id",
          "parentFolderId": "",
          "name": "Accounting",
          "path": "",
          "depth": 0,
          "resourceKey": ""
        }
      ]
    }
  ]
}
```

If `driveRef` is omitted, the endpoint returns all registered Drive folder references for the Workspace.

### Refresh Drive Folder Trees

```http
POST /api/drive-folders/tree/refresh?workspace=<Workspace>
POST /api/drive-folders/tree/refresh?workspace=<Workspace>&driveRef=<Reference Name>
```

Authentication:

```http
Authorization: Bearer <proxy_token>
```

Response:

```json
{
  "status": "ok",
  "driveFolders": [
    {
      "referenceName": "Accounting",
      "referenceKey": "accounting",
      "rootFolderId": "folder-id",
      "rootName": "Accounting",
      "folderCount": 12,
      "refreshedAt": "2026-06-13T10:00:00Z"
    }
  ]
}
```

## Agent Skill Package Download

The generated skill uses this endpoint to update itself.

```http
GET /api/agent-skill/download?platform=<generic|openclaw>
```

Authentication:

```http
Authorization: Bearer <proxy_token>
```

Response:

- `Content-Type: application/zip`
- `Content-Disposition: attachment; filename="ai-workspace-proxy-<platform>-skill.zip"`

The dashboard install overlay also creates single-use download URLs in this form:

```http
GET /api/agent-skill/download?token=<single-use-install-token>
```

That token mode is for local installation commands generated by the dashboard. It does not require an `Authorization` header, expires after 10 minutes, and is consumed after one successful download.

## Common Error Shape

Error responses are JSON:

```json
{
  "error": "request_denied",
  "message": "explanation"
}
```

Common statuses:

- `401`: missing, invalid, or suspended agent key
- `403`: Workspace not found, Drive folder denied, or policy denied
- `413`: request body exceeds `MAX_REQUEST_BODY_BYTES`
- `502`: Google token refresh, upstream Google, or Drive tree discovery error
