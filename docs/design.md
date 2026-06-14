# AI Workspace Proxy Design

This document summarizes the current software design so a coding agent can continue work without relying on chat history.

## Runtime Shape

AI Workspace Proxy is a single Go HTTP service backed by SQLite and local log files.

Core runtime components:

- `main.go`: validates configuration, validates the policy catalog, creates runtime directories, opens SQLite, and starts the HTTP server.
- `config.go`: reads environment configuration, validates allowed email domains/admins, and validates the proxy encryption key.
- `store.go`: owns SQLite schema and persistence operations.
- `app.go`: owns the `App` struct and route registration.
- `templates.go`: contains the dashboard/admin HTML, CSS, and browser JavaScript templates.

## HTTP Handler Layout

The HTTP application is split by domain:

- `app_auth.go`: proxy login, logout, and Google Workspace OAuth connection.
- `app_ui.go`: dashboard/settings page rendering.
- `app_policy_handlers.go`: policy editor CRUD and default-policy selection.
- `app_policy_api_handlers.go`: privileged policy CRUD and apply JSON API.
- `app_workspace_handlers.go`: connected Workspace account management.
- `app_drive_folder_handlers.go`: Drive folder registration and Drive tree API handlers.
- `app_token_handlers.go`: agent Workspace API config and end-user backend API config download/rotation.
- `app_admin_handlers.go`: admin user views and actions.
- `app_proxy_relay.go`: Google API proxy relay, policy enforcement, request logging, and upstream forwarding.
- `app_session.go`: session lookup, CSRF validation, admin checks, and Workspace selector resolution.
- `app_google_client.go`: Google OAuth token, user profile, Gmail profile, and Calendar inspection helpers.

Drive-specific helper code is further split:

- `app_drive_refs.go`: Drive link parsing, file-type helpers, and Drive reference utilities.
- `app_drive_google.go`: Google Drive metadata/listing calls.
- `app_drive_tree.go`: cached Drive folder tree discovery and subfolder resolution.
- `app_drive_rewrite.go`: Drive/Docs/Sheets/Slides request rewriting and folder enforcement.

Drive file permissions are intentionally narrower than Google Drive's generic `File` resource. `Drive files` capabilities apply to non-Google-native files only. Google Docs, Sheets, and Slides are also Drive `File` resources at the Google API layer, but the proxy routes their content export and deletion through the corresponding Docs, Sheets, and Slides policy capabilities. Drive metadata search remains a metadata capability so agents can discover file IDs without granting file-content access.

## Authentication And Identity

Proxy login and Workspace OAuth are separate flows.

- Users sign in to the proxy with Google OpenID Connect.
- Login is allowed only when the verified Google email domain is present in `ALLOWED_EMAIL_DOMAINS`.
- Admin access is controlled only by `ADMIN_EMAILS`.
- Each proxy user has one standard agent Workspace API key and one end-user backend API key.
- Each proxy user may connect multiple Workspace accounts.
- API requests must include a `workspace` selector matching either the connected Workspace email or its friendly name.

Workspace account tokens are encrypted at rest with `PROXY_ENCRYPTION_KEY`. Access tokens are refreshed with the stored refresh token when close to expiry.

## Policy Model

The proxy uses broad Google OAuth scopes but exposes only policy-approved operations.

- `policy.go` defines friendly user-facing capabilities.
- Each capability maps to one or more concrete Google API method/path allow rules.
- Some rules also have body policies for deeper request inspection.
- `ValidatePolicyCatalog()` must pass at startup; missing risk scores or risk descriptions prevent the server from starting.
- The system default policy allows all Low-risk capabilities.
- Users may create custom policies and set a default policy.
- Each connected Workspace account has one applied policy. When no custom policy is applied, the system policy is used.

Calendar event write permissions use body inspection and, for update/delete operations, Calendar event inspection to separate myself-only events from events involving third parties.

Contact lookup uses the Google People API search endpoints only. The user-facing `Read contacts` permission is Low risk because it is read-only, and the proxy limits it to search with constrained field masks and page sizes.

## Drive Folder Enforcement

Users register allowed Drive folders per connected Workspace account.

- Each folder has a user-facing Reference Name.
- The proxy extracts and stores the real Drive folder ID.
- The proxy recursively caches subfolders in SQLite.
- Agents use `driveRef` for the registered root and may use `drivePath` or `driveFolderId` for subfolders.
- Drive, Docs, Sheets, and Slides operations are rewritten or denied so they stay inside registered folder trees.
- Drive file create/read/update/delete policies apply to non-Google-native files only. Google Docs, Sheets, and Slides require their own capabilities even when the forwarded Google endpoint is technically a Drive API endpoint such as `files.export` or `files.delete`.

The cached folder tree can be refreshed from the dashboard or through the proxy API.

## Agent Skill Generation

`agent_skill.go` dynamically builds a zip package for AI agents.

The generated package includes:

- `SKILL.md`
- `config/agents-workspace-api-access.config.json`
- `scripts/workspace_proxy_tool.py`
- `scripts/install_skill.sh`
- `scripts/bootstrap_skill.sh`
- `scripts/update_skill.sh`
- policy-filtered per-Workspace Markdown files

Supported package targets:

- `generic`
- `openclaw`

The dashboard creates a single-use download token that expires after 10 minutes. The installed skill can later update itself through the proxy using the user's standard agent Workspace API key.

Generated Markdown uses `{baseDir}/` for local package paths. `bootstrap_skill.sh` rewrites inline code paths to the absolute installed skill directory.

## Persistence Notes

SQLite stores users, sessions, OAuth states, agent Workspace API keys, end-user backend API keys, Workspace connections, Workspace order, user settings, policies, Drive folder references, Drive tree cache, stats, and agent-skill download tokens.

This project intentionally does not carry backward-compatibility migration code unless explicitly requested. When schema changes are introduced during development, wipe/recreate the SQLite database before testing the new release.

## Security Boundaries

- The standard agent Workspace API key authenticates agent calls to the proxy.
- The end-user backend API key is reserved for user-owned configuration clients and must not bypass Workspace policy enforcement.
- Google OAuth tokens stay server-side and encrypted.
- The proxy enforces Workspace ownership before resolving a Workspace selector.
- The proxy enforces policy before forwarding requests to Google.
- The proxy enforces Drive folder restrictions before forwarding Drive/Docs/Sheets/Slides requests.
- Denied requests are logged with authorization and cookie headers removed, and Gmail raw MIME content redacted.

Generated skill packages include the user's standard agent Workspace API key by design. Users and agents must treat `config/agents-workspace-api-access.config.json` as secret material.
