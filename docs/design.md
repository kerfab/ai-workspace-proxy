# AI Workspace Proxy Design

This document summarizes the current software design so a coding agent can continue work without relying on chat history.

## Runtime Shape

AI Workspace Proxy is a single Go HTTP service backed by SQLite.

Core runtime components:

- `main.go`: validates configuration, validates the policy catalog, creates runtime directories, opens SQLite, and starts the HTTP server.
- `config.go`: reads environment configuration, validates allowed email domains/admins, and validates the proxy encryption key.
- `store.go`: owns SQLite schema and persistence operations.
- `app.go`: owns the `App` struct, route registration, and static asset serving.
- `templates.go`: loads embedded HTML templates and exposes the template objects used by handlers.
- `templates/`: contains standalone embedded HTML templates for self-contained pages such as login, OAuth errors, and two-factor challenge screens; reusable named shell fragments such as the top app bar, sidebar, main page header, and global overlays; and authenticated page-body templates such as dashboard, Workspace, agents, permissions, logs, settings, and admin `page-*.html` files.
- `static/app.css`: contains the shared browser UI visual system for the authenticated application shell.
- `static/app.js`: contains authenticated-app browser behavior, including progressive navigation, overlays, AJAX forms, log browser behavior, sortable/resizable admin tables, and clocks.
- `static/auth.css`: contains the shared visual system for standalone authentication and sign-in recovery pages.

The browser UI has two distinct navigation shells:

- Standard user mode for day-to-day Workspace, agent, permissions, logs, and settings management.
- Admin mode for organization-administration pages under `/admin/`.

Both shells use a shared app frame with a sticky top app bar, a left navigation rail, and a main content area. The top app bar is split into a left brand cell whose width matches the sidebar and a right context/action area. The brand cell uses the static Sentry Proxy logo asset, constrained by CSS to fit the existing top-bar height without distorting the image. Breadcrumbs and mode labels live in the right app-bar area so the logo column aligns exactly with the sidebar column below it.

Admin mode uses its own left navigation instead of reusing the standard user sidebar, includes a prominent `Return to user mode` action, and applies a distinct admin navigation treatment so it is visually clear that the user is operating inside the admin panel.

Internal GET navigation is progressively enhanced. Normal routes still render complete HTML documents, but same-origin sidebar/top-bar navigation can send `X-AIWP-Partial-Navigation: 1`. The server then renders the same dashboard template and returns JSON fragments for the breadcrumb, sidebar, page header, and main page content. The browser swaps only those fragments, preserving the top app bar logo and avoiding full document reload flicker. Sidebar DOM is preserved when the navigation structure is unchanged; if the server-rendered sidebar structure differs because a Workspace, agent, or mode changed, the sidebar is replaced from the same reusable fragment.

The partial-navigation fragments are cut from the same rendered template output through inert fragment markers. This keeps the full-page and partial-page flows on one source of truth. The top bar, sidebar, main page header, and global overlays are reusable named templates under `src/templates/`, so full-page rendering and partial navigation share the same shell markup. Authenticated route bodies live as `page-*.html` named templates, which lets the main dashboard template behave as a route switch rather than duplicated page markup. New reusable UI regions should be added as named template/fragments or shared partials rather than copy/pasted route-specific HTML. Standalone page templates should live as embedded files under `src/templates/`; large inline template strings are legacy structure to reduce over time.

Static browser assets are served under `/static/`. The handler sets MIME types for CSS/JavaScript and sends `Cache-Control` headers so browsers can reuse assets across page transitions. CSS and JavaScript use short cache lifetimes during development, while images such as the logo use a longer cache lifetime.

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

Drive file permissions are intentionally narrower than Google Drive's generic `File` resource. `Drive files` capabilities apply to Other file types only. Google Docs, Sheets, and Slides are also Drive `File` resources at the Google API layer, but the proxy routes their content export and deletion through the corresponding Docs, Sheets, and Slides policy capabilities. Drive metadata search remains a metadata capability so agents can discover file IDs without granting file-content access.

## Authentication And Identity

Proxy login and Workspace OAuth are separate flows.

- Users sign in to the proxy with Google OpenID Connect.
- Login is allowed only when the verified Google email domain is present in `ALLOWED_EMAIL_DOMAINS`.
- Admin access is controlled only by `ADMIN_EMAILS`.
- Organization-admin access is separate from global SaaS admin access. It is available only to organization-domain users who are currently authorized as admins for their domain, and all organization-admin pages live under `/admin/`.
- Each proxy user has one end-user backend API key and any number of agent identities.
- Each agent identity has its own API key, friendly name, default location, enable/suspend state, optional firewall, and one or more Workspace grants.
- Each proxy user may connect multiple Workspace accounts.
- API requests must include a `workspace` selector matching either the connected Workspace email or its friendly name.
- Browser sessions use server-side session records with configurable per-user session duration and optional TOTP-based second-factor verification.

Workspace account tokens are encrypted at rest with `PROXY_ENCRYPTION_KEY`. Access tokens are refreshed with the stored refresh token when close to expiry.

## Policy Model

The proxy uses broad Google OAuth scopes but exposes only policy-approved operations.

- `policy.go` defines friendly user-facing capabilities.
- Each capability maps to one or more concrete Google API method/path allow rules.
- Some rules also have body policies for deeper request inspection.
- `ValidatePolicyCatalog()` must pass at startup; missing risk scores or risk descriptions prevent the server from starting.
- The system default policy allows all Low-risk capabilities.
- Users may create custom policies and set a default policy.
- Each connected Workspace account may still have a user default policy, but AI-agent enforcement is decided per agent Workspace grant.
- Each agent Workspace grant chooses the policy that applies for that specific agent on that specific Workspace. When no custom policy is selected, the system policy is used.
- Custom policy permissions may be marked as requiring human review and approval. When a matched allowed capability has that flag, the proxy requires `X-AIWP-Human-Approval` with a non-empty explanation before forwarding the request. Generated skill Markdown adds an IMPORTANT notice to the affected operation sections and instructs agents to pass `--human-approval`.

Calendar event write permissions use body inspection and, for update/delete operations, Calendar event inspection to separate myself-only events from events involving third parties.

Contact lookup uses the Google People API search endpoints only. The user-facing `Read contacts` permission is Low risk because it is read-only, and the proxy limits it to search with constrained field masks and page sizes.

## Drive Folder Enforcement

Users register allowed Drive folders per connected Workspace account.

- Each folder has a user-facing Reference Name.
- The proxy extracts and stores the real Drive folder ID.
- The proxy recursively caches subfolders in SQLite.
- Registering a folder does not automatically grant it to every agent.
- Each agent Workspace grant has its own allowed Drive folder grant list for that Workspace.
- Agents use `driveRef` for the registered root and may use `drivePath` or `driveFolderId` for subfolders.
- Drive, Docs, Sheets, and Slides operations are rewritten or denied so they stay inside registered folder trees.
- Drive file create/read/update/delete policies apply to Other file types only. Google Docs, Sheets, and Slides require their own capabilities even when the forwarded Google endpoint is technically a Drive API endpoint such as `files.export` or `files.delete`.

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

The dashboard creates a single-use download token that expires after 10 minutes.
The agent config file contains only:

- `proxy_url`
- `agent_api_token`
- `workspaces`

The installed skill can later update itself through the proxy using that same agent's API key.

Agent records also track whether their installed skill may be stale. Changes that affect generated skill content, such as agent profile updates, API key rotation, Workspace grants, policy content used by the agent, Workspace friendly names, or granted Drive folder access, mark the affected agent with a persisted update warning. The UI shows a red badge in the left agent navigation and an alert on the agent page until the user dismisses it.

Generated Markdown uses `{baseDir}/` for local package paths. `bootstrap_skill.sh` rewrites inline code paths to the absolute installed skill directory.

## Persistence Notes

SQLite stores users, sessions, OAuth states, workspace reauth tokens, agent identities and API keys, agent Workspace grants, per-grant Drive folder grants, per-agent firewall rules, end-user backend API keys, Workspace connections, Workspace order, user settings, two-factor settings, per-user request logs, per-user log view settings, user audit logs, policy audit logs, Workspace audit logs, Drive folder audit logs, policies, Drive folder references, Drive tree cache, daily stats, and agent-skill download/install tokens.

## Logging Model

Proxy relay requests are logged per user in SQLite. Each request log records a request ID, timestamp, proxy user, agent ID, Workspace email, method, service, path, sanitized query, target object type/ID, user agent, remote address, `X-Forwarded-For`, `X-Real-IP`, `Forwarded`, `CF-Connecting-IP`, agent name/location/motive, human approval explanation when required, outcome, HTTP/upstream status, matched policy capability and rule for allowed requests, and error text for failures. Request and response bodies are not logged.

Agent name and location come from the configured agent identity. Motive enforcement is configured per agent Workspace grant. When `RequireAgentMotive` is enabled on a grant, a missing, blank, or oversized `X-AIWP-Agent-Motive` denies the proxy request before Workspace access. Length limits are 256 characters for name, 512 for location, and 4096 for motive.

Logs are always enabled for standard users and are retained for 7 days. Cleanup runs once at startup and then every 30 minutes across request logs and audit log tables. Request-log indexes include user/timestamp, user/outcome/timestamp, user/Workspace/timestamp, user/agent/timestamp, and request ID.

The activity-log console uses locally vendored Tabulator assets and server-side APIs for paging, retention-bounded time ranges, log-type filtering, regex filters, saved visible columns/page size, row details overlays, and CSV/JSON exports. Each grid column maps to one log field. The console consolidates request logs and audit logs into one view.

Policy, Workspace, Drive folder, and user configuration events use separate audit-log tables so each domain can evolve without one overly sparse log schema.

This project intentionally does not carry backward-compatibility migration code unless explicitly requested. When schema changes are introduced during development, wipe/recreate the SQLite database before testing the new release.

## Security Boundaries

- The agent API key authenticates agent calls to the proxy.
- Agent Workspace grants decide which Workspace accounts an agent may access and which policy applies for each Workspace.
- Agent firewall rules can additionally restrict which direct client IP addresses may use an agent key. Firewall checks use the effective remote address and ignore forwarded headers.
- The end-user backend API key is reserved for user-owned configuration clients and must not bypass agent grant policy enforcement.
- Google OAuth tokens stay server-side and encrypted.
- The proxy enforces Workspace ownership before resolving a Workspace selector.
- The proxy enforces policy before forwarding requests to Google.
- The proxy enforces Drive folder restrictions before forwarding Drive/Docs/Sheets/Slides requests.
- Per-user request logs do not store request or response bodies. Denied requests are recorded in the SQL logging model rather than duplicated into text log files.

Generated skill packages include the selected agent's API key by design. Users and agents must treat `config/agents-workspace-api-access.config.json` as secret material.
