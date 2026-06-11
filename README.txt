AI Workspace Proxy
===================

Developed by Fabien Kerbouci for The Sandbox / Animoca Brands.
A combination of ChatGPT and human manpower was used for development.

Proof-of-concept notice
-----------------------
This project is provided as tested proof-of-concept software. Its purpose is to demonstrate how a middleware proxy can help enforce stronger security controls when AI agents access Google Workspace services.

In particular, it shows how a proxy can reduce operational risk even when Google OAuth scopes are broader than the exact set of actions that should be allowed to an AI agent. It also hides the permissive Google OAuth credentials to AI agents, by delivering a middleware API key instead.

Limitations & Security Recommendations
--------------------------------------
The middleware proxy has been tested for deployment for corporate use. However, take note of the following limitations:
- No HTTPS support by default, it is recommend to deploy the proxy behind an HTTPS proxy (e.g. Cloudflare).
- The code of the proxy has been reviewed, and it embeds security features, such as proper user isolation & encryption at rest. However, it was not formally pentested.
- Consider firewalling the workspace proxy, with an authentication layer (e.g.: Cloudflare Access, IP whitelisting, etc.) so that it is not accessible from the public Internet. This is to mitigate risks of malicious exploitation for cases your Workspace Proxy API key leaks.

Overview
--------
AI Workspace Proxy is a single-binary Go HTTP service that:
- lets users sign in to the proxy with Google
- only admits users whose Google email belongs to one of the configured allowed domains
- lets each user connect one or more Google Workspace accounts via OAuth
- stores Google Workspace tokens server-side only
- issues a separate proxy API token to each user
- exposes relay endpoints for selected Google Workspace APIs
- enforces a server-side whitelist before forwarding requests to Google APIs
- stores operational state in SQLite and files only

The proxy may be granted advanced access to Google Workspace services, but it does not expose that full power directly to AI agents. Instead, it strictly inspects incoming HTTP requests and only relays operations that are explicitly permitted by the user's selected proxy policy. Policies are managed in the dashboard Policy Editor as friendly capabilities, and the proxy translates those capabilities into concrete Google API allow rules.

Supported Google Workspace services/actions supported in this build
-------------------------------------------------------------------
The built-in system default policy is read-only. Users can create custom policies in the Policy Editor to enable additional capabilities.

Gmail
- list messages
- read a message
- read a message attachment
- list threads
- read a thread
- list drafts
- read a draft
- list labels
- read a label
- custom policies may additionally allow draft writing, sending, label management, safe label changes, deletion/trash, imports, and notification watches

Google Calendar
- list the user calendar list
- read a calendar-list entry
- list events from accessible calendars
- read an individual event
- custom policies may additionally allow event writing, calendar management, calendar sharing management, and settings reads

The proxy requests full Google Calendar OAuth access, while the selected proxy policy controls which Calendar operations are actually exposed to agents.

Google Drive
- search/list files
- read file metadata
- export compatible Google Workspace files through Drive export
- custom policies may additionally allow file creation, metadata/content updates, deletion, permission management, comments, replies, and Drive metadata management

Google Docs
- read a document
- custom policies may additionally allow document creation and editing through `documents.batchUpdate`

Google Sheets
- read a spreadsheet
- read spreadsheet values
- custom policies may additionally allow spreadsheet creation and editing values, metadata, sheets, and structure

Google Slides
- read a presentation
- read an individual presentation page
- custom policies may additionally allow presentation creation and editing through `presentations.batchUpdate`

The exact allowlist is defined by the selected dashboard policy and the built-in capability catalog.

Important login and admin note
------------------------------
- Proxy login with Google is separate from the later Google Workspace OAuth step.
- Users may log in only if their Google email is verified and matches one of `ALLOWED_EMAIL_DOMAINS`.
- Admin access is driven only by `ADMIN_EMAILS`.
- At least one admin email must be configured at startup.
- A proxy user may connect multiple Workspace accounts. Each connected Workspace account is identified by its email address and an editable friendly name.
- The same Workspace mailbox may be connected by different proxy users if each user can legitimately complete Google OAuth for that mailbox.
- Agent/API requests must include a `workspace` selector matching either a connected Workspace friendly name or its full email address.

Google Cloud admin setup
------------------------
1. Create or select a Google Cloud project dedicated to the proxy.
2. Enable the APIs used by this build:
   - Gmail API
   - Google Calendar API
   - Google Drive API
   - Google Docs API
   - Google Sheets API
   - Google Slides API
3. Configure Google Auth Platform / OAuth consent:
   - set Branding information
   - choose the correct Audience (typically Internal for a company-only deployment)
   - declare the scopes required by this proxy
4. Create a Web application OAuth client.
5. Add authorized redirect URIs for the proxy.

Required redirect URIs
----------------------
The proxy uses these callback paths:
- `<APP_BASE_URL>/auth/google/callback`
- `<APP_BASE_URL>/auth/workspace/callback`

Examples:
- `https://workspace-proxy.company.com/auth/google/callback`
- `https://workspace-proxy.company.com/auth/workspace/callback`

Use the hostname allocated by your company for the proxy, and keep `APP_BASE_URL` synchronized with that externally visible hostname. If you expose the proxy on a non-default port, the port must be part of both `APP_BASE_URL` and the authorized redirect URIs. Example:
- `https://workspace-proxy.company.com:8443/auth/google/callback`
- `https://workspace-proxy.company.com:8443/auth/workspace/callback`

Google Cloud admin checklist
----------------------------
- Enable the six Google Workspace APIs listed above.
- Create a Web application OAuth client.
- Add the exact redirect URIs used by the proxy.
- Copy the client ID and client secret into the proxy environment:
  - `GOOGLE_WORKSPACE_CLIENT_ID`
  - `GOOGLE_WORKSPACE_CLIENT_SECRET`
- Keep `APP_BASE_URL` synchronized with the real URL users browse to.
- If users reconnect after a scope expansion, ensure the OAuth client and consent screen have already been updated to include the new scopes.

Environment variables
---------------------
Required:
- `APP_BASE_URL`                Example: `https://workspace-proxy.company.com`
- `GOOGLE_WORKSPACE_CLIENT_ID`
- `GOOGLE_WORKSPACE_CLIENT_SECRET`
- `PROXY_ENCRYPTION_KEY`        32 raw bytes, or 64 hex chars, or base64 for 32 bytes
- `ALLOWED_EMAIL_DOMAINS`       Comma-separated domains. Example: `company.com,gmail.com`
- `ADMIN_EMAILS`                comma-separated admin emails; at least one required

Other optional:
- `APP_BIND_ADDR`               default `:8080`
- `APP_NAME`                    default `AI Workspace Proxy`
- `DB_PATH`                     default `./db/ai_workspace_proxy.sqlite3`
- `DENIED_LOG_PATH`             default `./logs/denied.log`
- `SESSION_COOKIE_NAME`         default `ai_workspace_proxy_session`
- `COOKIE_SECURE`               default `false`
- `HTTP_CLIENT_TIMEOUT_SEC`     default `30`
- `SESSION_TTL_HOURS`           default `168`
- `MAX_REQUEST_BODY_BYTES`      default `10485760`

Allowed AI Drive folders
------------------------
Users can configure allowed Google Drive folders from the dashboard under each connected Workspace account. Each folder has:
- a unique Reference Name
- an extracted internal folder ID
- allowed file types (Docs, Sheets, Slides, generic Drive files)

The proxy resolves that Reference Name inside the selected Workspace account and enforces folder-level restrictions server-side. Registered folders include their cached subfolders.

When a folder is added or updated, the proxy recursively caches the subfolder tree in SQLite. Users can refresh that cached tree from the dashboard if folders are added, moved, or renamed in Google Drive.

Agents must include `workspace=<friendly_name_or_email>` for Workspace operations. They keep using `driveRef` for the registered root folder. For subfolders, agents may add:
- `drivePath=Reports/2026` for a relative subfolder path under the selected `driveRef`
- `driveFolderId=<folder_id>` when a path is ambiguous

Agents can inspect the cached folder tree through:
- `GET /api/drive-folders/tree?workspace=<Workspace>`
- `GET /api/drive-folders/tree?workspace=<Workspace>&driveRef=<Reference Name>`

Agents can refresh cached folder trees on behalf of the user through:
- `POST /api/drive-folders/tree/refresh?workspace=<Workspace>`
- `POST /api/drive-folders/tree/refresh?workspace=<Workspace>&driveRef=<Reference Name>`

Downloaded agent config
-----------------------
Users can download `ai-workspace-proxy.json` from the dashboard. It contains the global proxy token plus connected Workspaces sorted alphabetically by email:

```json
{
  "proxy_url": "http://localhost:8080",
  "proxy_token": "ptk_...",
  "workspaces": [
    {
      "email": "person@example.com",
      "name": "work"
    }
  ]
}
```

If the JSON contains exactly one Workspace, an agent may auto-use that Workspace `name`. If it contains multiple Workspaces, the agent should ask the user which Workspace friendly name or email to use.

Downloaded agent skill
----------------------
Users can download `ai-workspace-proxy-skill.zip` from the dashboard with the "Download agent skill" button, or from:

- `GET /api/agent-skill/download`

The endpoint accepts either a logged-in dashboard session or the user's Proxy API token as `Authorization: Bearer <token>`.

The zip contains:
- `SKILL.md`, a compact entrypoint explaining which service file the agent should read
- `scripts/workspace_proxy_tool.py`, the Python helper used for all proxy calls
- `scripts/update_skill.sh`, the shell updater agents can run with `sh ./scripts/update_skill.sh`
- `config/config.json`, filled with the proxy URL, proxy token, and connected Workspaces
- `skills/<workspace-email>/GMAIL.md`, an index pointing to Gmail read/write operation files
- `skills/<workspace-email>/gmail/READ-OPS.md`
- `skills/<workspace-email>/gmail/WRITE-OPS.md`
- `skills/<workspace-email>/DRIVE.md`, an index pointing to Drive product-specific operation files
- `skills/<workspace-email>/drive/FILES.md`
- `skills/<workspace-email>/drive/DOCS.md`
- `skills/<workspace-email>/drive/SHEETS.md`
- `skills/<workspace-email>/drive/SLIDES.md`
- `skills/<workspace-email>/CALENDAR.md`

The per-workspace service files are generated from the Workspace account's applied policy, so they only document operations that policy currently allows. The Python helper remains policy-agnostic and also includes `proxy request` for low-level access to any exposed proxy endpoint when a generated instruction needs it.

Known limitations and deployment recommendations
------------------------------------------------
- The proxy does not currently terminate TLS itself. If users require HTTPS/TLS encryption, place it behind an HTTPS-capable reverse proxy or access layer. A common pattern is to front it with Cloudflare Tunnel and/or Cloudflare Access, or with an internal reverse proxy/load balancer that handles TLS.
- The proxy should not be exposed directly to the public Internet as-is. It is intended for internal use. Prefer to publish it only behind your company's network controls such as a VPN, Zero Trust access gateway, or an access-control layer such as Cloudflare Access.
- Cloudflare Tunnel is a strong option when you want to avoid opening inbound ports or exposing a public IP address for the origin.
- The proxy itself supports Google Sign-In for application login. It does not natively authenticate users directly against third-party identity providers such as Okta.
- If your company uses another IdP such as Okta, you can still place the proxy behind an external access-control layer (for example Cloudflare Access) that authenticates users with that IdP before they ever reach the proxy.
- The dashboard Policy Editor is the current source of user policy configuration. The built-in system default policy remains available and cannot be deleted.
- This is an internal proxy and still deserves a security review before production use.

Operational notes
-----------------
- SQLite runs in WAL mode.
- Login OAuth state is stored safely even before a user record exists.
- Denied requests are logged to a rotating file set capped at 5 files total.
- Gmail relay requests require `workspace=<friendly_name_or_email>`, only accept `/users/me/...` or `/users/<selected_workspace_email>/...`, and normalize outbound Gmail requests to `/users/me/`.
- Calendar, Drive, Docs, Sheets, and Slides relays are all limited by the selected proxy policy.
- Drive folder access is enforced against registered folder trees cached in SQLite for the selected Workspace account.

Build and run
-------------
1. `./setup.sh`
2. Copy `config.env.example` to your own env file and fill the variables
3. `source` that env file
4. `make build`
5. `make run`

Docker build
------------
Build the Docker image from the project root:

`make docker`

This builds the image as `ai-workspace-proxy`.

Example values
--------------
- `ALLOWED_EMAIL_DOMAINS=company.com,gmail.com`
- `ADMIN_EMAILS=admin@company.com`
